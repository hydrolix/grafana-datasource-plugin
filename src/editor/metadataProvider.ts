import { firstValueFrom, map, Observable, tap } from "rxjs";
import {
  CoreApp,
  DataQueryResponse,
  dateTime,
  MetricFindValue,
  TimeRange,
} from "@grafana/data";
import { v4 } from "uuid";
import {
  ColumnDefinition,
  SchemaDefinition,
  TableIdentifier,
} from "@grafana/plugin-ui";
import { TableDefinition } from "@grafana/plugin-ui/dist/src/components/SQLEditor/types";
import { DataSource } from "../datasource";

const SCHEMA_SQL =
  "SELECT DISTINCT database as project FROM system.tables WHERE engine = 'TurbineStorage' AND (project != 'sample_project' AND project != 'hdx' AND total_rows > 0)";
const TABLES_SQL =
  "SELECT name FROM system.tables WHERE engine = 'TurbineStorage' AND database = '{schema}' AND total_rows > 0";
const COLUMNS_SQL =
  "SELECT name FROM system.columns WHERE database='{schema}' AND table ='{table}'";
const FUNCTIONS_SQL = "SELECT name FROM  system.functions";

const SUPPORTED_TYPES =[
    'DateTime', 'DateTime64',
    'String',
    'Int8', 'Int16', 'Int32', 'Int64', 'Int128', 'Int256',
    'UInt8', 'UInt16', 'UInt32', 'UInt64', 'UInt128', 'UInt256',
    'Float32', 'Float64',
    'Decimal32', 'Decimal64', 'Decimal128', 'Decimal256',
]

const NULLABLE_TYPES = SUPPORTED_TYPES.map(t => `Nullable(${t})`)

export const getQueryRunner = (
  ds: DataSource
): ((sql: string, timeRange?: TimeRange) => Observable<DataQueryResponse>) => {
  return (sql: string, timeRange?: TimeRange) =>
    ds.query({
      requestId: v4(),
      interval: "0",
      intervalMs: 0,
      range: timeRange
        ? timeRange
        : {
            to: dateTime(0),
            from: dateTime(0),
            raw: {
              to: dateTime(0),
              from: dateTime(0),
            },
          },
      scopedVars: {},
      targets: [
        {
          rawSql: sql,
          refId: "MD",
          round: "",
        },
      ],
      timezone: "UTC",
      app: CoreApp.Unknown,
      startTime: 0,
    });
};

export interface MetadataProvider {
  schemas: () => Promise<SchemaDefinition[]>;
  tables: (t: TableIdentifier) => Promise<TableDefinition[]>;
  columns: (t: TableIdentifier) => Promise<ColumnDefinition[]>;
  functions: () => Promise<
    Array<{ id: string; name: string; description: string }>
  >;
  tableKeys: (table: string) => Promise<MetricFindValue[]>;
  executeQuery: (
    query: string,
    timeRange?: TimeRange
  ) => Promise<DataQueryResponse>;
}
const transformResponse = (r: DataQueryResponse): string[] => {
  return r.data[0]?.fields?.length ? r.data[0].fields[0].values : [];
};

const transformSchemaResponse = <T,>(r: DataQueryResponse): T[] => {
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
  let tableKeys: { [table: string]: MetricFindValue[] } = {};
  let functions:
    | Array<{ id: string; name: string; description: string }>
    | undefined;
  let tableKeysFn: (table: string) => Promise<MetricFindValue[]>;
  if (ds.instanceSettings.jsonData.adHocKeyQuery) {
    tableKeysFn = (table: string) =>
      !tableKeys[table]
        ? firstValueFrom(
            queryRunner(
              ds.instanceSettings.jsonData.adHocKeyQuery!.replaceAll(
                "${table}",
                table
              )
            ).pipe(
              map((r) => {
                let fields = r.data[0]?.fields?.length
                  ? r.data[0].fields
                  : [[], []];
                let columns: string[] = fields[0]?.values;
                let types: string[] = fields[1]?.values;
                let defaultTypes: string[] = fields[2]?.values;
                return columns
                  .filter(
                    (c, i) =>
                        !c.includes("(") &&
                        (SUPPORTED_TYPES.includes(types[i]) || NULLABLE_TYPES.includes(types[i])) &&
                        defaultTypes[i] !== 'ALIAS'
                  )
                  .map((k) => ({ value: k, group: table } as MetricFindValue));
              }),
              tap((k) => (tableKeys[table] = k))
            )
          )
        : Promise.resolve(tableKeys[table]);
  } else {
    tableKeysFn = (_) => Promise.resolve([]);
  }
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
    tableKeys: tableKeysFn,
    executeQuery: (sql: string, timeRange?: TimeRange) =>
      firstValueFrom(queryRunner(sql, timeRange)),
  };
};
