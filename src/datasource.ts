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
import { getColumnValuesStatement } from "./ast";
import { getFirstValidRound, roundTimeRange } from "./editor/timeRangeUtils";
import { applyAstAwareMacro, applyBaseMacros } from "./macros/macrosApplier";
import { validateQuery } from "./editor/queryValidation";
import { SYNTHETIC_EMPTY, SYNTHETIC_NULL } from "./constants";
import { replace } from "./syntheticVariables";

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
    let interpolationResult: InterpolationResult;
    try {
      interpolationResult = await this.preInterpolateQuery(
        t.rawSql,
        "",
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
      rawSql: interpolationResult.finalSql ?? "",
      filters: undefined,
      querySettings,
      meta: {
        timezone: this.resolveTimezone(request),
      },
    };
  }

  public async interpolateQuery(
    query: HdxQuery,
    interpolationId: string,
    request: Partial<DataQueryRequest<HdxQuery>>,
    round?: string
  ): Promise<InterpolationResult> {
    let result = await this.preInterpolateQuery(
      query.rawSql,
      interpolationId,
      request,
      round
    );
    if (!result.hasError) {
      try {
        let interpolationResponse = await this.getInterpolatedQuery({
          ...query,
          rawSql: result.interpolatedSql ?? "",
          round: round ?? query.round,
        });
        if (interpolationResponse.error) {
          result = {
            ...result,
            hasError: true,
            error: interpolationResponse.error_message,
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
    }
    return result;
  }

  public async preInterpolateQuery(
    sql: string,
    interpolationId: string,
    request: Partial<DataQueryRequest<HdxQuery>>,
    round?: string
  ): Promise<InterpolationResult> {
    let preInterpolatedSql = sql;
    try {
      let macroContext: Context = {
        templateVars: this.templateSrv.getVariables(),
        intervalMs: request.intervalMs,
        query: sql,
        timeRange:
          round && request.range
            ? roundTimeRange(request.range, round)
            : request.range,
      };
      console.log("start", preInterpolatedSql);
      preInterpolatedSql = await applyBaseMacros(sql, macroContext);
      console.log("applyBaseMacros", preInterpolatedSql);

      preInterpolatedSql = this.templateSrv.replace(preInterpolatedSql);
      console.log("replace", preInterpolatedSql);

      let macroCTEResponse;
      try {
        macroCTEResponse = await this.getMacroCTE(preInterpolatedSql);
      } catch (e: any) {
        console.error(e);
        macroCTEResponse = {
          originalSql: preInterpolatedSql,
          error: true,
          error_message: "Unknown ast parsing error",
        };
      }

      try {
        preInterpolatedSql = await applyAstAwareMacro(preInterpolatedSql, {
          ...macroContext,
          query: preInterpolatedSql,
          macroCTE: macroCTEResponse.data,
          adHocFilter: {
            filters: request.filters,
            keys: (table: string) => this.metadataProvider.tableKeys(table),
          },
        });
        console.log("applyAstAwareMacro", preInterpolatedSql);
      } catch (e: any) {
        return {
          originalSql: sql,
          interpolationId,
          interpolatedSql: preInterpolatedSql,
          hasError: true,
          hasWarning: false,
          error: macroCTEResponse.error
            ? this.wrapSyntaxError(
                macroCTEResponse.error_message,
                preInterpolatedSql
              )
            : e.message,
        };
      }
      if (!macroCTEResponse.data) {
        return {
          originalSql: sql,
          interpolationId,
          interpolatedSql: preInterpolatedSql,
          finalSql: preInterpolatedSql,
          hasError: false,
          hasWarning: false,
        };
      }
      let validationResult = validateQuery(macroCTEResponse.data);
      return {
        originalSql: sql,
        interpolationId,
        interpolatedSql: preInterpolatedSql,
        finalSql: preInterpolatedSql,
        hasError: !!validationResult?.error,
        hasWarning: !!validationResult?.warning,
        error: validationResult?.error,
        warning: validationResult?.warning,
      };
    } catch (e: any) {
      console.error(e);
      return {
        originalSql: sql,
        interpolationId,
        interpolatedSql: preInterpolatedSql,
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

  async getInterpolatedQuery(query: HdxQuery): Promise<InterpolationResponse> {
    return this.postResource("interpolate", {
      data: {
        rawSql: query.rawSql,
        range: this.options?.range,
        interval: this.options?.interval,
        round: query.round,
      },
    }).then((a: any) => ({
      error: a.error,
      error_message: a.error_message,
      data: a.data as string,
      originalSql: query.rawSql,
    }));
  }

  async getMacroCTE(query: string): Promise<MacroCTEResponse> {
    if (query.toUpperCase().startsWith("DESCRIBE")) {
      return {
        error: false,
        error_message: "",
        data: [],
        originalSql: query,
      };
    }
    return this.postResource("macroCTE", {
      data: { query },
    }).then((a: any) => ({
      error: a.error,
      error_message: a.error_message,
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

    let keys = await this.metadataProvider
      .tableKeys(table)
      .then((keys) => keys.map((k) => k.value));
    if (!keys.includes(options.key)) {
      logWarning(
        `ad hoc filter key ${options.key} is not available for table ${table}`
      );
      return [];
    }

    let timeFilter = await this.metadataProvider.primaryKey(
      this.getTableIdentifier(table)
    );

    let sql;
    if (table && timeFilter) {
      sql = getColumnValuesStatement(
        options.key,
        table,
        timeFilter,
        this.getAdHocFilterValueCondition()
      );
    }
    if (!sql) {
      return [];
    }
    let response = await this.metadataProvider.executeQuery(
      (
        await this.preInterpolateQuery(sql, "", {
          ...this.options,
          filters: options.filters,
          range:
            options.timeRange ||
            this.instanceSettings.jsonData.adHocDefaultTimeRange,
        })
      ).interpolatedSql || "",
      options.timeRange
    );
    let fields: Field[] = response.data[0]?.fields?.length
      ? response.data[0].fields
      : [];
    let values: string[] = fields[0]?.values;
    if (!values) {
      return [];
    }

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
