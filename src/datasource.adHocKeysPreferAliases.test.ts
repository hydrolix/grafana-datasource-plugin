import { DataQueryRequest, toDataFrame } from '@grafana/data';
import { of } from 'rxjs';
import { MockDataSourceInstanceSettings, setupDataSourceMock } from './__mocks__/datasource';
import { adHocTableVariable } from './__mocks__/variable';
import { HdxQuery } from './types';

const describeRows = (rows: string[][]) => ({
  data: [
    toDataFrame({
      fields: ['name', 'type', 'default_type', 'default_expression'].map((name, i) => ({
        name,
        values: rows.map((row) => row[i]),
      })),
    }),
  ],
});

const schema = describeRows([
  ['cliIP', 'String', '', ''],
  ['client_ip', 'String', 'ALIAS', 'cliIP'],
  ['customerField', 'String', '', ''],
  ['attrs', 'Map(String, String)', '', ''],
]);

const singleColumn = (values: string[]) => ({ data: [toDataFrame({ fields: [{ name: 'value', values }] })] });

function setup({
  adHocKeysPreferAliases = true,
  tableValue = 'table',
  defaultDatabase = 'sample',
  values = ['env'],
}: { adHocKeysPreferAliases?: boolean; tableValue?: string; defaultDatabase?: string; values?: string[] } = {}) {
  const { datasource, queryMock } = setupDataSourceMock({
    variables: [
      {
        ...adHocTableVariable,
        current: { value: tableValue, text: tableValue, selected: true },
        options: [{ value: tableValue, text: tableValue, selected: true }],
      },
    ],
    customInstanceSettings: {
      ...MockDataSourceInstanceSettings,
      jsonData: { ...MockDataSourceInstanceSettings.jsonData, defaultDatabase, adHocKeysPreferAliases },
    },
  });
  // Every metadata statement (DESCRIBE, Map-key discovery, value preload)
  // reaches the backend through the one mocked query(); answer by statement.
  queryMock.mockImplementation((request: DataQueryRequest<HdxQuery>) =>
    of(request.targets[0].rawSql.startsWith('DESCRIBE') ? schema : singleColumn(values))
  );
  jest.spyOn(datasource.metadataProvider, 'primaryKey').mockResolvedValue('ts');
  const execute = jest.spyOn(datasource.metadataProvider, 'executeQuery');
  return { datasource, queryMock, execute };
}

const sent = (queryMock: jest.Mock): string[] =>
  queryMock.mock.calls.map(([request]: [DataQueryRequest<HdxQuery>]) => request.targets[0].rawSql);

const describes = (queryMock: jest.Mock) => sent(queryMock).filter((sql) => sql.startsWith('DESCRIBE'));

beforeEach(() => jest.clearAllMocks());

describe('ad-hoc keys with adHocKeysPreferAliases', () => {
  it('offers the alias instead of its base and still enumerates Map keys', async () => {
    const { datasource, queryMock } = setup();
    expect(await datasource.getTagKeys()).toEqual([
      { text: 'client_ip', value: 'client_ip', type: 'String', aliasFor: 'cliIP' },
      { text: 'customerField', value: 'customerField', type: 'String' },
      { text: "attrs['env']", value: "attrs['env']", type: 'Map(String, String)' },
    ]);
    expect(describes(queryMock)).toEqual(['DESCRIBE sample.table']);
  });

  it('reads a quoted view name under the default database', async () => {
    const { datasource, queryMock } = setup({ tableValue: 'logs#redacted', defaultDatabase: 'akamai' });
    const keys = await datasource.getTagKeys();
    expect(keys.map((k) => k.value)).toEqual(['client_ip', 'customerField', "attrs['env']"]);
    expect(describes(queryMock)).toEqual(['DESCRIBE akamai.`logs#redacted`']);
  });

  it('issues one DESCRIBE per table across dropdown opens', async () => {
    const { datasource, queryMock } = setup();
    await datasource.getTagKeys();
    await datasource.getTagKeys();
    expect(describes(queryMock)).toEqual(['DESCRIBE sample.table']);
  });

  it('returns the stock shape when the option is off', async () => {
    const { datasource, queryMock } = setup({ adHocKeysPreferAliases: false });
    expect(await datasource.getTagKeys()).toEqual([
      { text: 'cliIP', value: 'cliIP', type: 'String' },
      { text: 'client_ip', value: 'client_ip', type: 'String', aliasFor: 'cliIP' },
      { text: 'customerField', value: 'customerField', type: 'String' },
      { text: "attrs['env']", value: "attrs['env']", type: 'Map(String, String)' },
    ]);
    expect(describes(queryMock)).toEqual(['DESCRIBE sample.table']);
  });

  it.each(['cliIP', 'client_ip'])('still loads values for %s with the standard preload', async (key) => {
    const { datasource, execute } = setup({ values: ['192.0.2.20', '192.0.2.80'] });
    const filters = [{ key, operator: '=', value: '192.0.2.20' }];
    expect(await datasource.getTagValues({ key, filters })).toEqual([
      { text: '192.0.2.20', value: '192.0.2.20' },
      { text: '192.0.2.80', value: '192.0.2.80' },
    ]);
    expect(execute).toHaveBeenCalledTimes(1);
    const [sql, range, passedFilters] = execute.mock.calls[0];
    expect(sql).toContain(`(${key})) AS value`);
    expect(sql).toContain('FROM sample.table');
    expect(sql).toContain('$__timeFilter(ts)');
    expect(sql).toContain('hdx_query_max_timerange_sec');
    expect(range?.from.valueOf()).toBeLessThan(range!.to.valueOf());
    expect(passedFilters).toEqual(filters);
  });

  it.each(['cliIP', 'client_ip'])('leaves SQL naming %s untouched', (column) => {
    const { datasource } = setup();
    const rawSql = `SELECT ${column}, count() FROM sample.table GROUP BY ${column}`;
    expect(datasource.applyTemplateVariables({ refId: 'A', rawSql, round: '', querySettings: [] }, {}).rawSql).toBe(
      rawSql
    );
  });
});

describe('metricFindQuery', () => {
  it('leaves rawSql variables on the query path', async () => {
    const { datasource, queryMock } = setup({ values: ['a', 'b'] });
    const expected = [
      { text: 'a', value: 'a' },
      { text: 'b', value: 'b' },
    ];
    expect(await datasource.metricFindQuery('SELECT 1')).toEqual(expected);
    expect(await datasource.metricFindQuery({ rawSql: 'SELECT 1' })).toEqual(expected);
    expect(sent(queryMock)).toEqual(['SELECT 1', 'SELECT 1']);
  });
});
