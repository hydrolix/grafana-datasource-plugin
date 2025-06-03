import {
  AdHocVariableFilter,
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
import { applyAdHocMacro, applyBaseMacros } from "./macros/macrosApplier";
import { validateQuery } from "./editor/queryValidation";

export class DataSource extends DataSourceWithBackend<
  HdxQuery,
  HdxDataSourceOptions
> {
  public readonly metadataProvider = getMetadataProvider(this);
  private readonly beautifier = new ErrorMessageBeautifier();
  public options: DataQueryRequest<HdxQuery> | undefined;
  public filters: AdHocVariableFilter[] | undefined;

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
    if (request.app === CoreApp.Dashboard) {
      this.filters = request.filters;
    }
    let targets$ = from(
      Promise.all(
        request.targets
          .filter((t) => !(t.skipNextRun && t.skipNextRun()))
          .map(async (t) => {
            let interpolationResult: InterpolationResult;
            try {
              interpolationResult = await this.interpolateQuery(
                t.rawSql,
                request,
                getFirstValidRound([
                  t.round,
                  this.instanceSettings.jsonData.defaultRound || "",
                ])
              );
            } catch (e: any) {
              console.error(e);
              throw new Error(`cannot interpolate query, ${e?.message}`);
            }
            if (!interpolationResult.finalSql && interpolationResult.hasError) {
              throw new Error(interpolationResult.error);
            }

            return {
              ...t,
              rawSql: interpolationResult.finalSql ?? "",
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
    let interpolatedSql = sql;
    try {
      let macroContext = {
        templateVars: this.templateSrv.getVariables(),
        replaceFn: this.templateSrv.replace.bind(this),
        intervalMs: request.intervalMs,
        query: sql,
        timeRange:
          round && request.range
            ? roundTimeRange(request.range, round)
            : request.range,
      };
      interpolatedSql = await applyBaseMacros(sql, macroContext);
      interpolatedSql = this.templateSrv.replace(interpolatedSql);
      let astResponse;
      try {
        astResponse = await this.getAst(
          // this workaround is needed since ast parser doesn't recognise millisecond as a valid time unit
          // should be removed when PR https://github.com/AfterShip/clickhouse-sql-parser/pull/166 is applied
          interpolatedSql.replaceAll(" millisecond)", " second)")
        );
      } catch (e: any) {
        console.error(e);
        astResponse = {
          originalSql: interpolatedSql,
          error: true,
          error_message: "Unknown ast parsing error",
          data: null,
        };
      }

      try {
        interpolatedSql = await applyAdHocMacro(interpolatedSql, {
          ...macroContext,
          query: interpolatedSql,
          adHocFilter: {
            filters: request.filters,
            ast: astResponse.data,
            keys: (table: string) =>
              this.metadataProvider
                .tableKeys(table)
                .then((arr) => arr.map((k) => k.text)),
          },
        });
      } catch (e: any) {
        return {
          originalSql: sql,
          interpolatedSql: interpolatedSql,
          hasError: true,
          hasWarning: false,
          error: astResponse.error
            ? this.wrapSyntaxError(astResponse.error_message, interpolatedSql)
            : e.message,
        };
      }
      if (!astResponse.data) {
        return {
          originalSql: sql,
          interpolatedSql: interpolatedSql,
          finalSql: interpolatedSql,
          hasError: false,
          hasWarning: false,
        };
      }
      let validationResult = validateQuery(astResponse.data);
      return {
        originalSql: sql,
        interpolatedSql: interpolatedSql,
        finalSql: interpolatedSql,
        hasError: !!validationResult?.error,
        hasWarning: !!validationResult?.warning,
        error: validationResult?.error,
        warning: validationResult?.warning,
      };
    } catch (e: any) {
      console.error(e);
      return {
        originalSql: sql,
        interpolatedSql: interpolatedSql,
        hasError: true,
        hasWarning: false,
        error: e.message || "Unknown Error",
      };
    }
  }

  wrapSyntaxError(error_message: string, query: string) {
    if (!error_message || error_message === "Unknown Error") {
      return `Cannot apply ad hoc filter: unknown error occurred while parsing query '${query}'`;
    }
    const fullMessage = error_message;
    console.log(fullMessage);
    const errorRegExp = /^line\s(\d*):(\d*) (.*)$/;

    const [message] = fullMessage.split("\n");
    const match = errorRegExp.exec(message);
    if (match) {
      return `Cannot apply ad-hoc filter because of syntax error at line ${
        +match[1] + 1
      }: ${match[3]}`;
    } else {
      return fullMessage;
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
    let table = this.adHocFilterTableName();

    if (table) {
      return await this.metadataProvider.tableKeys(table);
    } else {
      return [];
    }
  }

  async getAst(query: string): Promise<AstResponse> {
    if (query.toUpperCase().startsWith("DESCRIBE")) {
      return {
        error: false,
        error_message: "",
        data: { describe: query },
        originalSql: query,
      };
    }
    return this.postResource("ast", {
      data: { query },
    }).then((a: any) => {
      let queryAst: SelectQuery = a.data?.length ? a.data[0] : null;
      return {
        error: a.error,
        error_message: a.error_message,
        data: queryAst,
        originalSql: query,
      };
    });
  }

  async getTagValues(
    options: DataSourceGetTagValuesOptions
  ): Promise<MetricFindValue[]> {
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
      sql = getColumnValuesStatement(options.key, table, timeFilter);
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
