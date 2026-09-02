import { firstValueFrom, map, Observable, tap } from "rxjs";
import {
  AdHocVariableFilter,
  CoreApp,
  DataQueryResponse,
  dateTime,
  TimeRange,
} from "@grafana/data";
import { v4 } from "uuid";
import {
  ColumnDefinition,
  SchemaDefinition,
  TableDefinition,
  TableIdentifier,
} from "@grafana/plugin-ui";
import { DataSource } from "../datasource";
import { AdHocFilterKeys, QuerySetting } from "../types";
import {
  AD_HOC_KEY_QUERY,
  AD_HOC_PRELOAD_ROUND_INTERVAL,
  COLUMNS_SQL,
  FUNCTIONS_SQL,
  METADATA_QUERY_TIMEOUT_SETTING,
  METADATA_QUERY_TIMEOUT_SETTING_ALIAS,
  METADATA_QUERY_TIMEOUT_VALUE,
  NULLABLE_TYPES,
  SCHEMA_SQL,
  SUPPORTED_TYPES,
  TABLES_SQL,
  PK_SQL,
  ARRAY_TYPES,
  MAP_TYPES,
} from "../constants";

export const ZERO_TIME_RANGE = {
  to: dateTime(0),
  from: dateTime(0),
  raw: {
    to: dateTime(0),
    from: dateTime(0),
  },
};
export const getQueryRunner = (
  ds: DataSource
): ((
  sql: string,
  timeRange?: TimeRange,
  filters?: AdHocVariableFilter[]
) => Observable<DataQueryResponse>) => {
  return (
    sql: string,
    timeRange?: TimeRange,
    filters?: AdHocVariableFilter[]
  ) => {
    const dsLevelSettings = ds.instanceSettings.jsonData.querySettings ?? [];
    // Min-wins: a DS-level value can only tighten the metadata breaker, never
    // loosen it. Missing / non-numeric / <= 0 values (0 means "unlimited" on
    // Hydrolix) are ignored, so the default stands.
    const dsLevelBreakerValues = dsLevelSettings
      .filter(
        (s) =>
          s.setting === METADATA_QUERY_TIMEOUT_SETTING ||
          s.setting === METADATA_QUERY_TIMEOUT_SETTING_ALIAS
      )
      .map((s) => Number(s.value))
      .filter((v) => Number.isFinite(v) && v > 0);
    const breakerValue = Math.min(
      Number(METADATA_QUERY_TIMEOUT_VALUE),
      ...dsLevelBreakerValues
    );
    const querySettings: QuerySetting[] = [
      {
        setting: METADATA_QUERY_TIMEOUT_SETTING,
        value: String(breakerValue),
      },
    ];
    return ds.query({
      requestId: v4(),
      interval: "0",
      intervalMs: 0,
      range: timeRange ? timeRange : ZERO_TIME_RANGE,
      scopedVars: {},
      targets: [
        {
          rawSql: sql,
          refId: "MD",
          round: AD_HOC_PRELOAD_ROUND_INTERVAL,
          querySettings,
          filters,
        },
      ],
      timezone: "UTC",
      app: CoreApp.Unknown,
      startTime: 0,
    });
  };
};

export interface MetadataProvider {
  schemas: () => Promise<SchemaDefinition[]>;
  tables: (t: TableIdentifier) => Promise<TableDefinition[]>;
  columns: (t: TableIdentifier) => Promise<ColumnDefinition[]>;
  primaryKey: (t: TableIdentifier) => Promise<string>;
  functions: () => Promise<
    Array<{ id: string; name: string; description: string }>
  >;
  tableKeys: (table: string) => Promise<AdHocFilterKeys[]>;
  executeQuery: (
    query: string,
    timeRange?: TimeRange,
    filters?: AdHocVariableFilter[]
  ) => Promise<DataQueryResponse>;
}

const transformResponse = (r: DataQueryResponse): string[] => {
  return r.data[0]?.fields?.length ? r.data[0].fields[0].values : [];
};

const transformSchemaResponse = <T>(r: DataQueryResponse): T[] => {
  return transformResponse(r).map((v: string) => ({ name: v } as T));
};

const transformFunctionResponse = (
  r: DataQueryResponse
): Array<{ id: string; name: string; description: string }> => {
  return transformResponse(r).map((v: string) => ({
    id: v,
    name: v,
    description: "",
  }));
};

export const getMetadataProvider = (ds: DataSource): MetadataProvider => {
  const queryRunner = getQueryRunner(ds);
  let schemas: SchemaDefinition[] | undefined;
  let tables: { [table: string]: TableDefinition[] } = {};
  let columns: { [table: string]: ColumnDefinition[] } = {};
  let tableKeys: { [table: string]: AdHocFilterKeys[] } = {};
  let primaryKeys: { [table: string]: string } = {};
  let functions:
    | Array<{ id: string; name: string; description: string }>
    | undefined;
  let tableKeysFn: (table: string) => Promise<AdHocFilterKeys[]>;
  tableKeysFn = (table: string) =>
    !tableKeys[table]
      ? firstValueFrom(
          queryRunner(AD_HOC_KEY_QUERY.replaceAll("${table}", table)).pipe(
            map((r) => {
              try {
                return getKeyMap(r);
              } catch (e: any) {
                throw new Error(
                  `Cannot apply ad hoc filters: unable to resolve filterable columns for "${table}"`
                );
              }
            }),
            tap((k) => (tableKeys[table] = k))
          )
        )
      : Promise.resolve(tableKeys[table]);

  return {
    schemas: () =>
      !schemas
        ? firstValueFrom(
            queryRunner(SCHEMA_SQL).pipe(
              map(transformSchemaResponse<SchemaDefinition>),
              tap((s) => (schemas = s))
            )
          )
        : Promise.resolve(schemas!),
    tables: (t: TableIdentifier) =>
      t && !tables[t?.schema!]
        ? firstValueFrom(
            queryRunner(TABLES_SQL.replace(/\{schema}/, t?.schema!)).pipe(
              map(transformSchemaResponse<TableDefinition>),
              tap((v) => (tables[t.schema!] = v))
            )
          )
        : Promise.resolve(tables[t?.schema!] || []),
    columns: (t: TableIdentifier) =>
      !columns[`${t.schema}.${t.table}`]
        ? firstValueFrom(
            queryRunner(
              COLUMNS_SQL.replace(/\{schema}/, t?.schema!).replace(
                /\{table}/,
                t?.table!
              )
            ).pipe(
              map(transformSchemaResponse<ColumnDefinition>),
              tap((v) => (columns[`${t.schema}.${t.table}`] = v))
            )
          )
        : Promise.resolve(columns[`${t.schema}.${t.table}`]!),
    functions: async () => {
      return !functions
        ? firstValueFrom(
            queryRunner(FUNCTIONS_SQL).pipe(
              map(transformFunctionResponse),
              tap((f) => (functions = f))
            )
          )
        : Promise.resolve(functions!);
    },
    primaryKey: (t: TableIdentifier) => {
      const key = `${t.schema}.${t.table}`;
      // Presence check, not truthiness: a table with no primary key resolves
      // to "" or undefined, and a truthiness test would treat that as "not
      // fetched yet" and re-query the cluster on every call. Assistant
      // publishes context on a 300ms debounce, so that is once per keystroke.
      return !(key in primaryKeys)
        ? firstValueFrom(
            queryRunner(
              PK_SQL.replace(/\{schema}/, t?.schema!).replace(
                /\{table}/,
                t?.table!
              )
            ).pipe(
              map((r) => transformResponse(r)[0]),
              tap((v) => (primaryKeys[key] = v))
            )
          )
        : Promise.resolve(primaryKeys[key]);
    },
    tableKeys: tableKeysFn,
    executeQuery: (
      sql: string,
      timeRange?: TimeRange,
      filters?: AdHocVariableFilter[]
    ) => firstValueFrom(queryRunner(sql, timeRange, filters)),
  };
};

export const getType = (
  definitionByKey: Map<string, KeyDefinition>,
  c: string
) => {
  let definition = definitionByKey.get(c)!;
  if (definition.isAlias) {
    return definitionByKey.get(definition.aliasFor)?.type || "";
  } else {
    return definition.type;
  }
};

export const getKeyMap = (r: DataQueryResponse): AdHocFilterKeys[] => {
  try {
    let fields = r.data[0]?.fields?.length
      ? r.data[0].fields
      : [[], [], [], []];
    let columns: string[] = fields[0]?.values;
    let types: string[] = fields[1]?.values;
    let defaultTypes: string[] = fields[2]?.values;
    let defaultExpression: string[] = fields[3]?.values;
    const aliasRegExp = /`(.*)`/;
    let definitionByKey: Map<string, KeyDefinition> = new Map(
      columns.map((c, i) => [
        c,
        {
          name: c,
          type: types[i],
          isAlias: defaultTypes[i] === "ALIAS",
          aliasFor: aliasRegExp.test(defaultExpression[i])
            ? aliasRegExp.exec(defaultExpression[i])?.[1]!
            : "",
        },
      ])
    );
    return columns
      .filter((c) => {
        let type = getType(definitionByKey, c);
        return (
          !c.includes("(") &&
          (SUPPORTED_TYPES.includes(type) ||
            NULLABLE_TYPES.includes(type) ||
            ARRAY_TYPES.includes(type) ||
            MAP_TYPES.includes(type))
        );
      })
      .map((k) => ({ text: k, value: k, type: getType(definitionByKey, k) }));
  } catch (e: any) {
    throw new Error("can not get columns for ad hoc filter", e.message);
  }
};

interface KeyDefinition {
  name: string;
  type: string;
  isAlias: boolean;
  aliasFor: string;
}
