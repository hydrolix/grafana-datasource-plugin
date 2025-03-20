import {
  CoreApp,
  DataFrame,
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
} from "@grafana/data";
import {
  DataSourceWithBackend,
  getTemplateSrv,
  TemplateSrv,
} from "@grafana/runtime";
import { isString } from "lodash";
import { DEFAULT_QUERY, HdxDataSourceOptions, HdxQuery } from "./types";
import { from, Observable, switchMap } from "rxjs";
import { map } from "rxjs/operators";
import { ErrorMessageBeautifier } from "./errorBeautifier";
import { ConditionalAllApplier } from "./conditionalAllApplier";
import { AdHocFilterApplier, keyToColumnAndTable } from "./adHocFilterApplier";
import { getMetadataProvider } from "./editor/metadataProvider";
import { getColumnValuesStatement, getTable as getAstTable } from "./ast";

export class DataSource extends DataSourceWithBackend<
  HdxQuery,
  HdxDataSourceOptions
> {
  public readonly metadataProvider = getMetadataProvider(this);
  private readonly beautifier = new ErrorMessageBeautifier();
  private readonly conditionalAllApplier = new ConditionalAllApplier();
  private readonly adHocFilterApplier = new AdHocFilterApplier(
    this.metadataProvider,
    this.getTable.bind(this)
  );

  constructor(
    public instanceSettings: DataSourceInstanceSettings<HdxDataSourceOptions>,
    readonly templateSrv: TemplateSrv = getTemplateSrv()
  ) {
    super(instanceSettings);
  }

  async metricFindQuery(query: HdxQuery | string, options?: any) {
    const hdxQuery = isString(query) ? { rawSql: query } : query;
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
          this.adHocFilterApplier
            .apply(t.rawSql || "", request.filters)
            .then((q) => ({
              ...t,
              rawSql: q,
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

  getDefaultQuery(_: CoreApp): Partial<HdxQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: HdxQuery, scoped: ScopedVars): HdxQuery {
    let rawQuery = query.rawSql || "";
    rawQuery = this.conditionalAllApplier.apply(
      rawQuery,
      this.templateSrv.getVariables()
    );
    return {
      ...query,
      rawSql: this.replace(rawQuery, scoped) || "",
    };
  }

  async getTagKeys(
    options: DataSourceGetTagKeysOptions<HdxQuery>
  ): Promise<MetricFindValue[]> {
    if (!this.instanceSettings.jsonData.adHocKeyQuery) {
      return [];
    }
    let table = this.replace(
      `$\{${this.instanceSettings.jsonData.adHocTableVariable}}`
    );

    if (table && !table.startsWith("${")) {
      return await this.metadataProvider.tableKeys(table);
    } else if (
      this.instanceSettings.jsonData.defaultDatabase &&
      this.instanceSettings.jsonData.defaultTable
    ) {
      return await this.metadataProvider.tableKeys(
        `${this.instanceSettings.jsonData.defaultDatabase}.${this.instanceSettings.jsonData.defaultTable}`
      );
    } else {
      let tables = options.queries
        ?.map((q) => this.getTable(q.rawSql))
        .filter((t, i, arr) => arr.indexOf(t) === i);
      if (tables && tables.length) {
        return (
          await Promise.all(
            tables.map((t) => this.metadataProvider.tableKeys(t))
          )
        )
          .flatMap((k) => k)
          .map((k) => ({
            ...k,
            value: `${k.group}.${k.value}`,
          }));
      } else {
        return [];
      }
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
    options: DataSourceGetTagValuesOptions<HdxQuery>
  ): Promise<MetricFindValue[]> {
    if (!this.instanceSettings.jsonData.adHocValuesQuery) {
      return [];
    }
    let [column, table] = keyToColumnAndTable(options.key);
    if (!table) {
      let variable = this.replace(
        `$\{${this.instanceSettings.jsonData.adHocTableVariable}}`
      );
      if (variable && !variable?.startsWith('${')) {
        table = variable
      }
    }
    if (
      !table &&
      this.instanceSettings.jsonData.defaultDatabase &&
      this.instanceSettings.jsonData.defaultTable
    ) {
      table = `${this.instanceSettings.jsonData.defaultDatabase}.${this.instanceSettings.jsonData.defaultTable}`;
    }
    if (!table) {
      return [];
    }
    let sql = options.queries
      ?.map((q): string => {
        if (this.getTable(q.rawSql) === table) {
          return getColumnValuesStatement(
            column,
            q.rawSql,
            this.instanceSettings.jsonData.adHocValuesQuery!
          );
        }
        return "";
      })
      .find((s) => s);
    if (!sql) {
      sql = options.queries
        ?.map((q): string =>
          getColumnValuesStatement(
            column,
            q.rawSql,
            this.instanceSettings.jsonData.adHocValuesQuery!
          )
        )
        .find((s) => s);
    }

    if (!sql) {
      return [];
    }

    let response = await this.metadataProvider.executeQuery(
      await this.adHocFilterApplier.apply(sql, options.filters),
      options.timeRange
    );
    let fields: Field[] = response.data[0]?.fields?.length
      ? response.data[0].fields
      : [];
    let values: string[] = fields[0]?.values;
    let counts: string[] = fields[1]?.values;
    return values
      .filter((n) => n !== "")
      .map((n, i) => ({
        text: n ? `${n} (${counts[i]})` : n,
        value: n,
      }));
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
