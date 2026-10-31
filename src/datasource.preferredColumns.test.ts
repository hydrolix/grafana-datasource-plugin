import { toDataFrame } from '@grafana/data';
import { MockDataSourceInstanceSettings, setupDataSourceMock } from './__mocks__/datasource';
import { adHocTableVariable } from './__mocks__/variable';

const schema = {
  data: [
    toDataFrame({
      fields: [
        { name: 'name', values: ['cliIP', 'client_ip', 'customerField', 'attrs'] },
        { name: 'type', values: ['String', 'String', 'String', 'Map(String, String)'] },
        { name: 'default_type', values: ['', 'ALIAS', '', ''] },
        { name: 'default_expression', values: ['', 'cliIP', '', ''] },
      ],
    }),
  ],
};

function setup(preferredFieldNames = true) {
  const { datasource } = setupDataSourceMock({
    variables: [adHocTableVariable],
    customInstanceSettings: {
      ...MockDataSourceInstanceSettings,
      jsonData: { ...MockDataSourceInstanceSettings.jsonData, preferredFieldNames },
    },
  });
  const execute = jest.spyOn(datasource.metadataProvider, 'executeQuery').mockResolvedValue(schema);
  jest.spyOn(datasource.metadataProvider, 'primaryKey').mockResolvedValue('ts');
  return { datasource, execute };
}

beforeEach(() => jest.clearAllMocks());

describe('opt-in preferred columns', () => {
  it('offers alias keys and unrelated columns without enumerating map keys', async () => {
    const { datasource, execute } = setup();
    expect(await datasource.getTagKeys()).toEqual([
      { text: 'client_ip', value: 'client_ip' },
      { text: 'customerField', value: 'customerField' },
    ]);
    expect(execute.mock.calls).toEqual([['DESCRIBE sample.table']]);
  });

  it('keeps the default key-discovery path', async () => {
    const { datasource, execute } = setup(false);
    const keys = [{ text: 'cliIP', value: 'cliIP', type: 'String' }];
    jest.spyOn(datasource.metadataProvider, 'tableKeys').mockResolvedValue(keys);
    expect(await datasource.getTagKeys()).toEqual(keys);
    expect(execute).not.toHaveBeenCalled();
  });

  it('opts a field variable in independently and resolves the table variable', async () => {
    const { datasource, execute } = setup(false);
    expect(await datasource.metricFindQuery({ preferredColumns: 'sample.$table' })).toEqual([
      { text: 'client_ip', value: 'client_ip' },
      { text: 'customerField', value: 'customerField' },
      { text: 'attrs', value: 'attrs' },
    ]);
    expect(execute.mock.calls).toEqual([['DESCRIBE sample.table']]);
  });

  it('rejects SQL in the metadata table option before making a request', async () => {
    const { datasource, execute } = setup();
    await expect(datasource.metricFindQuery({ preferredColumns: 'sample.table; SELECT 1' })).rejects.toThrow(
      'qualified database.table'
    );
    expect(execute).not.toHaveBeenCalled();
  });

  it.each(['cliIP', 'client_ip'])('retains values and the requested SQL binding for %s', async (key) => {
    const { datasource, execute } = setup();
    execute.mockResolvedValueOnce(schema).mockResolvedValueOnce({
      data: [toDataFrame({ fields: [{ name: key, values: ['192.0.2.20', '192.0.2.80'] }] })],
    });
    const filters = [{ key, operator: '=', value: '192.0.2.20' }];
    expect(await datasource.getTagValues({ key, filters })).toEqual([
      { text: '192.0.2.20', value: '192.0.2.20' },
      { text: '192.0.2.80', value: '192.0.2.80' },
    ]);
    const [sql, range, passedFilters] = execute.mock.calls[1];
    expect(sql).toContain(`(${key})) AS value`);
    expect(sql).toContain('FROM sample.table');
    expect(sql).toContain('$__timeFilter(ts)');
    expect(range?.from.valueOf()).toBeLessThan(range!.to.valueOf());
    expect(passedFilters).toEqual(filters);
    const rawSql = `SELECT ${key}, count() FROM sample.table GROUP BY ${key}`;
    expect(datasource.applyTemplateVariables({ refId: 'A', rawSql, round: '', querySettings: [] }, {}).rawSql).toBe(rawSql);
  });
});
