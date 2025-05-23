import {
  CoreApp,
  DataFrame,
  DataQueryError,
  DataQueryRequest,
  DataQueryResponse,
  DataSourceGetTagValuesOptions,
  DataSourceInstanceSettings,
  Field,
  getTimeZone,
  getTimeZoneInfo,
  MetricFindValue,
  ScopedVars,
  TestDataSourceResponse,
} from "@grafana/data";
import {
  DataSourceWithBackend,
  getTemplateSrv,
  logError,
  logWarning,
  TemplateSrv,
} from "@grafana/runtime";
import {
  AstResponse,
  DEFAULT_QUERY,
  HdxDataSourceOptions,
  HdxQuery,
  InterpolationResult,
  SelectQuery,
} from "./types";
import { from, Observable, switchMap } from "rxjs";
import { map } from "rxjs/operators";
import { ErrorMessageBeautifier } from "./errorBeautifier";
import {
  getMetadataProvider,
  ZERO_TIME_RANGE,
} from "./editor/metadataProvider";
import { getColumnValuesStatement } from "./ast";
import { getFirstValidRound, roundTimeRange } from "./editor/timeRangeUtils";
import { applyMacros } from "./macros/macrosApplier";
import { validateQuery } from "./editor/queryValidation";

export class DataSource extends DataSourceWithBackend<
  HdxQuery,
  HdxDataSourceOptions
> {
  public readonly metadataProvider = getMetadataProvider(this);
  private readonly beautifier = new ErrorMessageBeautifier();
  public options: DataQueryRequest<HdxQuery> | undefined;

  constructor(
    public instanceSettings: DataSourceInstanceSettings<HdxDataSourceOptions>,
    readonly templateSrv: TemplateSrv = getTemplateSrv()
  ) {
    super(instanceSettings);
  }

  async metricFindQuery(query: Partial<HdxQuery> | string, options?: any) {
    const hdxQuery: Partial<HdxQuery> =
      typeof query === "string" ? { rawSql: query } : query;
    if (!hdxQuery.rawSql) {
      return [];
    }
    const frame = await this.runQuery(hdxQuery, options);
    if (frame.fields?.length === 0) {
      return [];
    }
    if (frame?.fields?.length === 1) {
      return frame?.fields[0]?.values.map((text) => ({ text, value: text }));
    }
    // convention - assume the first field is an id field
    const ids = frame?.fields[0]?.values;
    return frame?.fields[1]?.values.map((text, i) => ({ text, value: ids[i] }));
  }

  query(request: DataQueryRequest<HdxQuery>): Observable<DataQueryResponse> {
    if (request.range !== ZERO_TIME_RANGE) {
      this.options = request;
    }
    let targets$ = from(
      Promise.all(
        request.targets
          .filter((t) => !(t.skipNextRun && t.skipNextRun()))
          .map(async (t) => {
            let q: string | undefined;
            try {
              q =
                (
                  await this.interpolateQuery(
                    t.rawSql,
                    request,
                    getFirstValidRound([
                      t.round,
                      this.instanceSettings.jsonData.defaultRound || "",
                    ])
                  )
                ).interpolatedSql || "";
            } catch (e: any) {
              console.error(e);
              throw new Error(`cannot interpolate query, ${e?.message}`);
            }

            return {
              ...t,
              rawSql: q,
              filters: undefined,
              meta: {
                timezone: this.resolveTimezone(request),
              },
            };
          })
      )
    );

    return targets$.pipe(
      switchMap((targets) =>
        super
          .query({
            ...request,
            targets,
          })
          .pipe(
            map((response: DataQueryResponse) => {
              const errors = response.errors?.map((error: DataQueryError) => {
                console.error(error);
                logError(
                  {
                    name: `DataQueryError with status ${error.statusText}`,
                    message: error.message || "",
                  },
                  {
                    data_message: error.data?.message || "",
                    data_error: error.data?.error || "",
                    message: error.message || "",
                    status: error.status?.toString() || "",
                    statusText: error.statusText || "",
                    refId: error.refId || "",
                    traceId: error.traceId || "",
                    type: "" + error.type,
                  }
                );

                if (error.message) {
                  const message = this.beautifier.beautify(error.message);
                  if (message) {
                    return { ...error, message: message };
                  }
                }
                return error;
              });

              return {
                ...response,
                errors: errors,
                error: undefined,
              };
            })
          )
      )
    );
  }

  public async interpolateQuery(
    sql: string,
    request: Partial<DataQueryRequest<HdxQuery>>,
    round?: string
  ): Promise<InterpolationResult> {
    let astResponse;
    try {
      astResponse = await this.getAst(sql);

      let s = await applyMacros(sql, {
        adHocFilter: {
          filters: request.filters,
          ast: astResponse.data,
          keys: (table: string) =>
            this.metadataProvider
              .tableKeys(table)
              .then((arr) => arr.map((k) => k.text)),
        },
        templateVars: this.templateSrv.getVariables(),
        replaceFn: this.templateSrv.replace.bind(this),
        intervalMs: request.intervalMs,
        query: sql,
        timeRange:
          round && request.range
            ? roundTimeRange(request.range, round)
            : request.range,
      });
      let interpolatedSql = this.templateSrv.replace(s);
      if (astResponse.error) {
        return {
          originalSql: sql,
          interpolatedSql: interpolatedSql,
          hasError: true,
          hasWarning: false,
          error: astResponse.error_message || "Unknown Error",
        };
      }
      if (!astResponse.data) {
        return {
          originalSql: sql,
          interpolatedSql: interpolatedSql,
          hasError: false,
          hasWarning: false,
        };
      }
      let validationResult = validateQuery(astResponse.data);
      return {
        originalSql: sql,
        interpolatedSql: interpolatedSql,
        hasError: !!validationResult?.error,
        hasWarning: !!validationResult?.warning,
        error: validationResult?.error,
        warning: validationResult?.warning,
      };
    } catch (e: any) {
      console.error(e);
      return {
        originalSql: sql,
        hasError: true,
        hasWarning: false,
        error: e.message || "Unknown Error",
      };
    }
  }

  getDefaultQuery(_: CoreApp): Partial<HdxQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: HdxQuery, scoped: ScopedVars): HdxQuery {
    let rawQuery = query.rawSql || "";
    return {
      ...query,
      rawSql: this.replace(rawQuery, scoped) || "",
    };
  }

  async getTagKeys(): Promise<MetricFindValue[]> {
    if (!this.instanceSettings.jsonData.adHocKeysQuery) {
      return [];
    }
    let table = this.adHocFilterTableName();

    if (table) {
      return await this.metadataProvider.tableKeys(table);
    } else {
      return [];
    }
  }

  async getAst(query: string): Promise<AstResponse> {
    return new Promise((resolve) =>
      setTimeout(
        () =>
          this.postResource("ast", {
            data: {
              query: query
                ?.replaceAll("{", "(")
                ?.replaceAll("}", ")")
                ?.replaceAll(":", "_"),
            },
          }).then((a: any) => {
            let queryAst: SelectQuery = a.data?.length ? a.data[0] : null;
            return resolve({
              error: a.error,
              error_message: a.error_message,
              data: queryAst,
              originalSql: query,
            });
          }),
        1000
      )
    );
  }

  async getTagValues(
    options: DataSourceGetTagValuesOptions
  ): Promise<MetricFindValue[]> {
    if (!this.instanceSettings.jsonData.adHocValuesQuery) {
      return [];
    }

    let table = this.adHocFilterTableName();
    if (!table) {
      return [];
    }

    let keys = await this.metadataProvider
      .tableKeys(table)
      .then((keys) => keys.map((k) => k.value));
    if (!keys.includes(options.key)) {
      logWarning(
        `ad-hoc filter key ${options.key} is not available for table ${table}`
      );
      return [];
    }

    let timeFilter;
    let timeFilterVariable = this.replace(
      `$\{${this.instanceSettings.jsonData.adHocTimeColumnVariable}}`
    );
    if (timeFilterVariable && !timeFilterVariable?.startsWith("${")) {
      timeFilter = timeFilterVariable;
    }

    let sql;
    if (table && timeFilter) {
      sql = getColumnValuesStatement(
        options.key,
        table,
        timeFilter,
        this.instanceSettings.jsonData.adHocValuesQuery!
      );
    }
    if (!sql) {
      return [];
    }

    let response = await this.metadataProvider.executeQuery(
      (
        await this.interpolateQuery(sql, {
          ...this.options,
          filters: options.filters,
          range:
            options.timeRange ||
            this.instanceSettings.jsonData.adHocDefaultTimeRange,
        })
      ).interpolatedSql || ""
    );
    let fields: Field[] = response.data[0]?.fields?.length
      ? response.data[0].fields
      : [];
    let values: string[] = fields[0]?.values;
    return values
      .filter((n) => n !== "")
      .map((n) => ({
        text: n ? n : "null",
        value: n,
      }));
  }

  private adHocFilterTableName() {
    let table = this.replace(
      `$\{${this.instanceSettings.jsonData.adHocTableVariable}}`
    );

    if (table && !table.startsWith("${")) {
      if (table.includes(".")) {
        return table;
      } else {
        return `${this.instanceSettings.jsonData.defaultDatabase}.${table}`;
      }
    } else {
      return undefined;
    }
  }

  filterQuery(query: HdxQuery): boolean {
    // if no query has been provided, prevent the query from being executed
    return !!query.rawSql;
  }

  private runQuery(
    request: Partial<HdxQuery>,
    options?: any
  ): Promise<DataFrame> {
    return new Promise((resolve) => {
      const req = {
        targets: [{ ...request, refId: String(Math.random()) }],
        range: options ? options.range : (this.templateSrv as any).timeRange,
      } as DataQueryRequest<HdxQuery>;
      this.query(req).subscribe((res: DataQueryResponse) => {
        resolve(res.data[0] || { fields: [] });
      });
    });
  }

  private replace(value?: string, scopedVars?: ScopedVars) {
    if (value !== undefined) {
      return this.templateSrv.replace(value, scopedVars);
    }
    return value;
  }

  private resolveTimezone(
    request: DataQueryRequest<HdxQuery>
  ): string | undefined {
    // timezone specified in the time picker
    if (request.timezone && request.timezone !== "browser") {
      return request.timezone;
    }
    // fall back to the local timezone
    const localTimezoneInfo = getTimeZoneInfo(getTimeZone(), Date.now());
    return localTimezoneInfo?.ianaName;
  }

  testDatasource(): Promise<TestDataSourceResponse> {
    return super.testDatasource().catch((error) => {
      if (error.message) {
        const message = this.beautifier.beautify(error.message);
        if (message) {
          return { ...error, message: message };
        }
      }
      return error;
    });
  }
}
