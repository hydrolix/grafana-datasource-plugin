import {Props as QueryEditorProps} from "../components/QueryEditor";
import {firstValueFrom, map, Observable, tap} from "rxjs";
import {DataQueryResponse} from "@grafana/data";
import {v4} from "uuid";
import {ColumnDefinition, SchemaDefinition, TableIdentifier} from "@grafana/plugin-ui";
import {TableDefinition} from "@grafana/plugin-ui/dist/src/components/SQLEditor/types";


const SCHEMA_SQL = 'SELECT DISTINCT database as project FROM system.tables WHERE engine = \'TurbineStorage\' AND (project != \'sample_project\' AND project != \'hdx\' AND total_rows > 0)';
const TABLES_SQL = 'SELECT name FROM system.tables WHERE engine = \'TurbineStorage\' AND database = \'{schema}\' AND total_rows > 0';
const COLUMNS_SQL = 'SELECT name FROM system.columns WHERE database=\'{schema}\' AND table =\'{table}\'';
const FUNCTIONS_SQL = 'SELECT name FROM  system.functions';

const getQueryRunner = (props: QueryEditorProps): (sql: string) => Observable<DataQueryResponse> => {
    return (sql: string) => props.datasource.query({
        requestId: v4(),
        interval: '0',
        intervalMs: 0,
        range: props.range!,
        scopedVars: {},
        targets: [{
            rawSql: sql,
            refId: props.query.refId,
            round: ''
        }],
        timezone: "UTC",
        app: props.app!,
        startTime: 0
    });
};

export interface MetadataProvider {
    schemas: () => Promise<SchemaDefinition[]>,
    tables: (t: TableIdentifier) => Promise<TableDefinition[]>,
    columns: (t: TableIdentifier) => Promise<ColumnDefinition[]>,
    functions: () => Promise<Array<{ id: string, name: string, description: string }>>,
}
const transformResponse = (r: DataQueryResponse): string[] =>  {
    return r.data[0]?.fields?.length ? r.data[0].fields[0].values : []
}

const transformSchemaResponse = <T,>(r: DataQueryResponse): T[] =>  {
    return transformResponse(r).map((v: string) => ({name: v,} as T))
}

const transformFunctionResponse = (r: DataQueryResponse): Array<{ id: string, name: string, description: string }> =>  {
    return transformResponse(r).map((v: string) => ({id: v, name: v, description: ''}))
}

export const getMetadataProvider = (props: QueryEditorProps): MetadataProvider => {
    const queryRunner = getQueryRunner(props);
    let schemas: SchemaDefinition[] | undefined
    let tables: { [table: string]: TableDefinition[] } = {}
    let columns: { [table: string]: ColumnDefinition[] } = {}
    let functions: Array<{ id: string, name: string, description: string }> | undefined
    return {
        schemas: () => !schemas ? firstValueFrom(queryRunner(SCHEMA_SQL).pipe(
            map(transformSchemaResponse<SchemaDefinition>),
            tap(s => schemas = s))
        ) : Promise.resolve(schemas!),
        tables: (t: TableIdentifier) =>  t && !tables[t?.schema!] ? firstValueFrom(queryRunner(TABLES_SQL.replace(/\{schema}/, t?.schema!)).pipe(
                map(transformSchemaResponse<TableDefinition>),
                tap(v => tables[t.schema!] = v))
            ) : Promise.resolve(tables[t?.schema!] || []),
        columns: (t: TableIdentifier) => !columns[`${t.schema}.${t.table}`] ? firstValueFrom(queryRunner(COLUMNS_SQL.replace(/\{schema}/, t?.schema!).replace(/\{table}/, t?.table!)).pipe(
            map(transformSchemaResponse<ColumnDefinition>),
            tap(v => columns[`${t.schema}.${t.table}`] = v)
        )) : Promise.resolve(columns[`${t.schema}.${t.table}`]!),
        functions: async () => {
            return !functions ? firstValueFrom(queryRunner(FUNCTIONS_SQL).pipe(
                map(transformFunctionResponse),
                tap(f => functions = f))
            ) : Promise.resolve(functions!)
        }
    }
};
