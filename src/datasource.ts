import {
  AdHocVariableFilter,
  ConstantVariableModel,
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
  MacroCTEResponse,
  Context,
  DEFAULT_QUERY,
  HdxDataSourceOptions,
  HdxQuery,
  InterpolationResult,
  TableIdentifier,
  InterpolationResponse,
} from "./types";
import { from, Observable, switchMap } from "rxjs";
import { map } from "rxjs/operators";
import { ErrorMessageBeautifier } from "./errorBeautifier";
import {
  getMetadataProvider,
  ZERO_TIME_RANGE,
} from "./editor/metadataProvider";
import { getColumnKeysForMapStatement, getColumnValuesStatement } from "./ast";
import { MAP_KEY_REGEX, SYNTHETIC_EMPTY, SYNTHETIC_NULL } from "./constants";
import { replace } from "./syntheticVariables";
import { applyConditionalAll } from "./macros/macrosApplier";

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
          .map(async (t) => await this.prepareTarget(t, request))
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

  private async prepareTarget(
    t: HdxQuery,
    request: DataQueryRequest<HdxQuery>
  ) {
    const querySettings = (
      this.instanceSettings.jsonData.querySettings ?? []
    ).reduce((acc: { [key: string]: any }, s) => {
      acc[s.setting] = replace(this.templateSrv.replace(`${s.value}`), {
        raw_query: () => t.rawSql,
        query_source: () => request.app,
      });
      return acc;
    }, {});

    return {
      ...t,
      querySettings,
      meta: {
        timezone: this.resolveTimezone(request),
      },
    };
  }

  public async interpolateQuery(
    query: HdxQuery,
    interpolationId: string
  ): Promise<InterpolationResult> {
    let macroContext: Context = {
      templateVars: this.templateSrv.getVariables(),
      query: query.rawSql,
    };
    let sql = applyConditionalAll(query.rawSql, macroContext);

    sql = this.templateSrv.replace(sql);

    let result: InterpolationResult = {
      originalSql: query.rawSql,
      interpolationId,
      interpolatedSql: sql,
      finalSql: sql,
      hasError: false,
      hasWarning: false,
    };
    try {
      let interpolationResponse = await this.getInterpolatedQuery({
        ...query,
        rawSql: sql,
      });
      if (interpolationResponse.error) {
        result = {
          ...result,
          hasError: true,
          error: interpolationResponse.errorMessage,
        };
      } else {
        result = {
          ...result,
          interpolatedSql: interpolationResponse.data,
        };
      }
    } catch (e: any) {
      console.error(e);
      result = {
        ...result,
        hasError: true,
        error: "Unknown ast parsing error",
      };
    }
    return result;
  }

  wrapSyntaxError(errorMessage: string, query: string) {
    if (!errorMessage || errorMessage === "Unknown Error") {
      return `Cannot apply ad hoc filter: unknown error occurred while parsing query '${query}'`;
    }
    const fullMessage = errorMessage;
    const errorRegExp = /^line\s(\d*):(\d*) (.*)$/;

    const [message] = fullMessage.split("\n");
    const match = errorRegExp.exec(message);
    if (match) {
      return `Cannot apply Grafana macros due to a query syntax error at line ${
        +match[1] + 1
      }: ${match[3]}\nPlease correct the query syntax.`;
    } else {
      return fullMessage;
    }
  }

  getDefaultQuery(_: CoreApp): Partial<HdxQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(
    query: HdxQuery,
    scoped: ScopedVars,
    filters: AdHocVariableFilter[] = []
  ): HdxQuery {
    let rawQuery = query.rawSql || "";
    rawQuery = applyConditionalAll(rawQuery, {
      query: rawQuery,
      templateVars: this.templateSrv.getVariables(),
    });
    return {
      ...query,
      filters,
      rawSql: this.replace(rawQuery, scoped) || "",
    };
  }

  async getTagKeys(): Promise<MetricFindValue[]> {
    let table = this.adHocFilterTableName();

    if (table) {
      const keys = await this.metadataProvider.tableKeys(table);
      const maps = await Promise.all(
        keys
          .filter((key) => key.type.includes("Map"))
          .map((key) => key.value?.toString())
          .filter((key) => !!key)
          .map((column) => this.getTagKeysForMap(column!, table))
      ).then((response: Array<{ key: string; val: string[] }>) =>
        response.reduce((map, obj) => {
          map[obj.key] = obj.val;
          return map;
        }, {} as { [key: string]: string[] })
      );

      return keys
        .map((key) => {
          return (key.value || "") in maps
            ? maps[key.value!].map((r: string) => ({
                ...key,
                value: r,
                text: r,
              }))
            : key;
        })
        .flat();
    } else {
      return [];
    }
  }

  async getTagKeysForMap(
    column: string,
    table: string
  ): Promise<{ key: string; val: string[] }> {
    const response = await this.metadataProvider.executeQuery(
      getColumnKeysForMapStatement(column, table),
      this.options?.range,
      this.filters
    );
    let values: string[] = this.getValuesFromResponse(response);
    return { key: column, val: values.map((v) => `${column}['${v}']`) };
  }

  async getInterpolatedQuery(query: HdxQuery): Promise<InterpolationResponse> {
    return this.postResource("interpolate", {
      data: {
        rawSql: query.rawSql,
        range: this.options?.range,
        interval: this.options?.interval,
        filters: this.filters,
        round: query.round,
      },
    }).then((a: any) => ({
      error: a.error,
      errorMessage: a.errorMessage,
      data: a.data as string,
      originalSql: query.rawSql,
    }));
  }

  async getMacroCTE(query: string): Promise<MacroCTEResponse> {
    if (query.toUpperCase().startsWith("DESCRIBE")) {
      return {
        error: false,
        errorMessage: "",
        data: [],
        originalSql: query,
      };
    }
    return this.postResource("macroCTE", {
      data: { query },
    }).then((a: any) => ({
      error: a.error,
      errorMessage: a.errorMessage,
      data: a.data,
      originalSql: query,
    }));
  }

  async getTagValues(
    options: DataSourceGetTagValuesOptions
  ): Promise<MetricFindValue[]> {
    let table = this.adHocFilterTableName();
    if (!table) {
      return [];
    }

    const keys = await this.metadataProvider.tableKeys(table);
    const isMapKey = MAP_KEY_REGEX.test(options.key);

    const keyNames = keys.map((k) => k.value);

    if (
      (!isMapKey && !keyNames.includes(options.key)) ||
      (isMapKey &&
        !keyNames
          .filter((name) => !!name)
          .map((name) => name!.toString())
          .some((name) => options.key.startsWith(name)))
    ) {
      logWarning(
        `ad hoc filter key ${options.key} is not available for table ${table}`
      );
      return [];
    }
    const type = keys.find((k) => k.value === options.key)?.type;
    let column: string;
    if (type?.includes("Array")) {
      column = `arrayJoin(${options.key})`;
    } else {
      column = options.key;
    }

    let timeFilter = await this.metadataProvider.primaryKey(
      this.getTableIdentifier(table)
    );

    let sql;
    if (table && timeFilter) {
      sql = getColumnValuesStatement(
        column,
        table,
        timeFilter,
        this.getAdHocFilterValueCondition()
      );
    }
    if (!sql) {
      return [];
    }
    let response = await this.metadataProvider.executeQuery(
      sql,
      options.timeRange,
      options.filters
    );
    let values: string[] = this.getValuesFromResponse(response);
    return [
      ...values
        .filter((v) => v)
        .filter((v) => ![SYNTHETIC_EMPTY, SYNTHETIC_NULL].includes(v)),

      values.filter((v) => v === "").length ? SYNTHETIC_EMPTY : null,
      values.filter((v) => v === null || v === undefined).length
        ? SYNTHETIC_NULL
        : null,
    ]
      .filter((v) => v !== null)
      .map((n: string) => ({
        text: n,
        value: n,
      }));
  }
  private getValuesFromResponse(response: DataQueryResponse): string[] {
    let fields: Field[] = response.data[0]?.fields?.length
      ? response.data[0].fields
      : [];
    return fields[0]?.values || [];
  }

  private adHocFilterTableName() {
    let table = this.replace(
      `$\{${this.instanceSettings.jsonData.adHocTableVariable}}`
    )?.trim();

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

  private getAdHocFilterValueCondition(): string {
    const varName = this.instanceSettings.jsonData.adHocConditionVariable;
    if (!varName) {
      return "";
    }
    const variable = this.templateSrv
      .getVariables()
      .find((v) => v.name === varName);
    if (!variable) {
      return "";
    }
    return (variable as ConstantVariableModel).query;
  }

  private getTableIdentifier(s: string): TableIdentifier {
    if (s.includes(".")) {
      let arr = s.split(".");
      return {
        schema: arr[0],
        table: arr[1],
      };
    } else {
      return {
        schema: this.instanceSettings.jsonData.defaultDatabase,
        table: s,
      };
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
