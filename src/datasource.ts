import {
  AdHocVariableFilter,
  ConstantVariableModel,
  CoreApp,
  DataFrame,
  dateTime,
  DataQueryError,
  DataQueryRequest,
  DataQueryResponse,
  DataSourceGetTagKeysOptions,
  DataSourceGetTagValuesOptions,
  DataSourceInstanceSettings,
  Field,
  getTimeZone,
  getTimeZoneInfo,
  MetricFindValue,
  ScopedVars,
  TestDataSourceResponse,
  TimeRange,
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
  InterpolationContext,
  InterpolationResult,
  TableIdentifier,
  InterpolationResponse,
  QuerySetting,
} from "./types";
import { from, Observable, switchMap } from "rxjs";
import { map } from "rxjs/operators";
import { ErrorMessageBeautifier } from "./errors/errorBeautifier";
import { getMetadataProvider } from "./editor/metadataProvider";
import { getColumnKeysForMapStatement, getColumnValuesStatement } from "./ast";
import {
  AD_HOC_PRELOAD_LOOKBACK_SECONDS,
  MAP_KEY_REGEX,
  NULLABLE_MAP_TYPES,
  NULLABLE_TYPES,
  SYNTHETIC_EMPTY,
  SYNTHETIC_NULL,
} from "./constants";
import { replace } from "./syntheticVariables";
import { applyConditionalAll } from "./macros/macrosApplier";
import { ErrorExposer } from "./errors/errorExposer";
import defaultConfigs from "./defaultConfigs";
import {
  getDefaultQuery,
  isAnnotationRequest,
  prepareQuery,
} from "./annotations";

export class DataSource extends DataSourceWithBackend<
  HdxQuery,
  HdxDataSourceOptions
> {
  public readonly metadataProvider = getMetadataProvider(this);
  private readonly beautifier = new ErrorMessageBeautifier();
  private errorExposer!: ErrorExposer;

  constructor(
    public instanceSettings: DataSourceInstanceSettings<HdxDataSourceOptions>,
    readonly templateSrv: TemplateSrv = getTemplateSrv()
  ) {
    super(instanceSettings);
    this.errorExposer = new ErrorExposer(
      this.beautifier,
      this.templateSrv,
      this.instanceSettings.jsonData?.exposeErrors ||
        defaultConfigs.exposeErrors
    );
    this.annotations = { prepareQuery, getDefaultQuery };
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
    if (isAnnotationRequest(request)) {
      request = { ...request, app: "annotation" };
    }
    let targets$ = from(
      Promise.all(
        request.targets.map(async (t) => await this.prepareTarget(t, request))
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
                  this.errorExposer.addErrorToVariable(error.message);

                  let message = this.beautifier.beautify(error.message);
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
    const builder = this.querySettingsBuilder({
      raw_query: () => t.rawSql,
      query_source: () => request.app,
      "panel.id": () =>
        request.panelId !== undefined ? String(request.panelId) : "",
      "panel.name": () => request.panelName ?? "",
      app: () => request.app,
      ref_id: () => t.refId,
    });
    builder.addSettings(this.instanceSettings.jsonData.querySettings ?? []);
    builder.addSettings(t.querySettings ?? []);

    return {
      ...t,
      querySettings: builder.build(),
      meta: {
        timezone: this.resolveTimezone(request),
      },
    };
  }

  private querySettingsBuilder(vars: { [v: string]: () => string }) {
    const accumulator: { [v: string]: string } = {};
    return {
      addSettings: (querySettings: QuerySetting[]) =>
        querySettings &&
        querySettings
          .filter((s) => s.setting)
          .reduce((acc: { [key: string]: any }, s) => {
            acc[s.setting] = replace(
              this.templateSrv.replace(`${s.value}`),
              vars
            );
            return acc;
          }, accumulator),
      build: (): QuerySetting[] =>
        Object.keys(accumulator).map((s) => ({
          setting: s,
          value: accumulator[s],
        })),
    };
  }

  public async interpolateQuery(
    query: HdxQuery,
    interpolationId: string,
    context: InterpolationContext
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
      let interpolationResponse = await this.getInterpolatedQuery(
        {
          ...query,
          rawSql: sql,
        },
        context
      );
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

  async getTagKeys(
    options?: DataSourceGetTagKeysOptions<HdxQuery>
  ): Promise<MetricFindValue[]> {
    let table = this.adHocFilterTableName();

    if (table) {
      const keys = await this.metadataProvider.tableKeys(table);
      const maps = await Promise.all(
        keys
          .filter((key) => key.type.includes("Map"))
          .map((key) => key.value?.toString())
          .filter((key) => !!key)
          .map((column) =>
            this.getTagKeysForMap(
              column!,
              table,
              options?.timeRange,
              options?.filters
            )
          )
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
    table: string,
    timeRange?: TimeRange,
    filters?: AdHocVariableFilter[]
  ): Promise<{ key: string; val: string[] }> {
    const response = await this.metadataProvider.executeQuery(
      getColumnKeysForMapStatement(column, table),
      this.adHocPreloadRange(timeRange),
      filters
    );
    let values: string[] = this.getValuesFromResponse(response);
    return { key: column, val: values.map((v) => `${column}['${v}']`) };
  }

  async getInterpolatedQuery(
    query: HdxQuery,
    context: InterpolationContext
  ): Promise<InterpolationResponse> {
    return this.postResource("interpolate", {
      data: {
        rawSql: query.rawSql,
        range: context.range,
        interval: context.interval,
        filters: context.filters,
        round: query.round,
      },
    }).then((a: any) => ({
      error: a.error,
      errorMessage: a.errorMessage,
      data: a.data as string,
      originalSql: query.rawSql,
    }));
  }

  // Read by the Assistant's QueryWithAssistantButton to phrase its prompt
  // around the current query instead of asking for a brand-new one.
  getQueryDisplayText(query: HdxQuery): string {
    return query.rawSql || "";
  }

  // Parses SQL via the backend /ast resource (clickhouse-sql-parser). The
  // parse itself runs no query, but sqlds connects to the cluster when it
  // creates the datasource instance — before serving its first resource call
  // — so an unreachable cluster fails this too. Also rejects on syntax
  // errors, normal for mid-edit SQL; callers treat either as "no structure
  // available".
  async getAst(sql: string): Promise<unknown> {
    const response: { error?: boolean; errorMessage?: string; data?: unknown } =
      await this.postResource("ast", { data: { query: sql } });
    if (response.error) {
      throw new Error(response.errorMessage || "Unknown ast parsing error");
    }
    return response.data;
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

    // For map-access keys (e.g. attrs['env']) the key list only carries the
    // base map column, never the full accessor - resolve the base column's
    // type and gate on the nullable Map(String, Nullable(...)) variant.
    const nullGateType = isMapKey
      ? keys.find((k) => k.value === options.key.split("['")[0])?.type
      : type;
    const nullableTypes = isMapKey ? NULLABLE_MAP_TYPES : NULLABLE_TYPES;
    const isNullable = !!nullGateType && nullableTypes.includes(nullGateType);

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
      this.adHocPreloadRange(options.timeRange),
      options.filters
    );
    let values: string[] = this.getValuesFromResponse(response);
    return [
      ...values
        .filter((v) => v)
        .filter((v) => ![SYNTHETIC_EMPTY, SYNTHETIC_NULL].includes(v)),

      values.filter((v) => v === "").length ? SYNTHETIC_EMPTY : null,
      isNullable ? SYNTHETIC_NULL : null,
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

  /**
   * Time window for the ad-hoc key/value preload queries. Resolution order is
   * tag-keys options -> template service -> trailing lookback.
   *
   * Both preload statements carry `$__timeFilter()`, while `ZERO_TIME_RANGE` —
   * the sentinel `executeQuery` substitutes for a missing range — means "this
   * metadata query has no time macro" and is only correct for the unfiltered
   * `system.*` / `DESCRIBE` lookups. Letting it reach a filtered query
   * resolves to a 1970 window that returns no rows and silently erases the
   * column from the ad-hoc dropdown. This never returns undefined, so the
   * sentinel is unreachable from the preload path.
   */
  private adHocPreloadRange(timeRange?: TimeRange): TimeRange {
    const resolved = timeRange ?? this.templateServiceRange();
    if (resolved) {
      return this.capPreloadTimeRange(resolved);
    }
    // Last resort: the same lookback the cap already enforces for long ranges.
    const to = dateTime();
    const from = dateTime(
      to.valueOf() - AD_HOC_PRELOAD_LOOKBACK_SECONDS * 1000
    );
    return { from, to, raw: { from, to } };
  }

  /**
   * Grafana 10.4 — the floor `plugin.json` declares — does not populate
   * `timeRange` on the tag-keys options, even though the field has existed on
   * `DataSourceFilteringRequestOptions` since 10.3. On that version the
   * dashboard window is reachable only through the template service, so this
   * fallback is load-bearing, not defensive: without it
   * `tests/adHocMapKeys.spec.ts` fails on 10.4.18 (map keys resolve against a
   * now-relative window instead of the dashboard's, find no rows, and the Map
   * column disappears from the dropdown) while passing on 11.6.1 / 12.0.2 /
   * 13.0.1. Re-check against the CI matrix before removing it.
   *
   * `timeRange` is absent from the published `TemplateSrv` type — `runQuery`
   * reads it the same way — so the value carries no type guarantee. Shape-guard
   * it rather than letting a reshaped value throw out of
   * `capPreloadTimeRange` and reject the whole `getTagKeys` promise.
   */
  private templateServiceRange(): TimeRange | undefined {
    const range = (this.templateSrv as any).timeRange;
    return Number.isFinite(range?.from?.valueOf?.()) &&
      Number.isFinite(range?.to?.valueOf?.())
      ? (range as TimeRange)
      : undefined;
  }

  private capPreloadTimeRange(range: TimeRange): TimeRange {
    const lookbackFromMs =
      range.to.valueOf() - AD_HOC_PRELOAD_LOOKBACK_SECONDS * 1000;
    if (range.from.valueOf() >= lookbackFromMs) {
      // Already inside the lookback, so nothing to cap. Return the range
      // untouched rather than rebuilding it: the rewrite below would freeze a
      // relative `raw` (e.g. "now-6h") to an absolute instant for no gain.
      return range;
    }
    // `raw` describes the same window in unresolved form, so it has to move
    // with `from` — a capped window is no longer whatever the picker said
    // (e.g. "now-90d"). `to` is untouched, so `raw.to` carries through.
    return {
      ...range,
      from: dateTime(lookbackFromMs),
      raw: {
        ...range.raw,
        from: dateTime(lookbackFromMs),
      },
    };
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
