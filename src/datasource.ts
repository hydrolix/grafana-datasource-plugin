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
import { DEFAULT_QUERY, HdxDataSourceOptions, HdxQuery } from "./types";
import { from, Observable, switchMap } from "rxjs";
import { map } from "rxjs/operators";
import { ErrorMessageBeautifier } from "./errorBeautifier";
import { getMetadataProvider } from "./editor/metadataProvider";
import { getColumnValuesStatement, getTable as getAstTable } from "./ast";
import { getFirstValidRound } from "./editor/timeRangeUtils";
import { registerMacrosService } from "./macros/registerMacrosService";

export class DataSource extends DataSourceWithBackend<
  HdxQuery,
  HdxDataSourceOptions
> {
  public readonly metadataProvider = getMetadataProvider(this);
  private readonly beautifier = new ErrorMessageBeautifier();

  private readonly macrosService = registerMacrosService(
    this.metadataProvider,
    this.getTable.bind(this)
  );

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
    let targets$ = from(
      Promise.all(
        request.targets.map((t) =>
          this.applyMacros(t.rawSql, request).then((q) => ({
            ...t,
            rawSql: q,
            round: getFirstValidRound([
              t.round,
              this.instanceSettings.jsonData.defaultRound || "",
            ]),
            filters: undefined,
            meta: {
              timezone: this.resolveTimezone(request),
            },
          }))
        )
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

  private applyMacros(
    sql: string,
    request: Partial<DataQueryRequest<HdxQuery>>
  ) {
    return this.macrosService.applyMacros(sql || "", {
      filters: request.filters,
      templateVars: this.templateSrv.getVariables(),
      replaceFn: this.templateSrv.replace.bind(this),
      intervalMs: request.intervalMs,
      timeRange: request.range,
    });
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

  getTable(sql: string): string {
    let astTable = getAstTable(sql);
    const varRegex = /\$\{(.*)}/;
    if (varRegex.test(astTable)) {
      return this.templateSrv.replace(astTable);
    } else {
      return astTable;
    }
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
      await this.applyMacros(sql, { filters: options.filters }),
      options.timeRange || this.instanceSettings.jsonData.adHocDefaultTimeRange
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
