import { DataQueryResponse, toDataFrame } from '@grafana/data';
import { preferredColumnsFromResponse } from './preferredColumns';

const response = (rows: string[][]): DataQueryResponse => ({
  data: [
    toDataFrame({
      fields: ['name', 'type', 'default_type', 'default_expression'].map((name, i) => ({
        name,
        values: rows.map((row) => row[i]),
      })),
    }),
  ],
});

describe('experimental preferred field names', () => {
  it.each(['cliIP', '`cliIP`'])('prefers the exact alias for expression %s, keeping new fields', (expression) => {
    const result = preferredColumnsFromResponse(
      response([
        ['cliIP', 'String', '', ''],
        ['unrelatedPhysical', 'String', '', ''],
        ['client_ip', 'String', 'ALIAS', expression],
        ['customerField', 'String', '', ''],
      ])
    );
    expect(result.map((c) => c.name)).toEqual(['unrelatedPhysical', 'client_ip', 'customerField']);
  });
  it('preserves computed aliases, type changes, unresolved targets and cycles', () => {
    const rows = [
      ['cliIP', 'String', '', ''],
      ['computed', 'String', 'ALIAS', 'lower(`cliIP`)'],
      ['cast', 'IPv4', 'ALIAS', 'cliIP'],
      ['missing', 'String', 'ALIAS', 'unknown'],
      ['a', 'String', 'ALIAS', 'b'],
      ['b', 'String', 'ALIAS', 'a'],
    ];
    expect(preferredColumnsFromResponse(response(rows)).map((c) => c.name)).toEqual(rows.map((r) => r[0]));
  });
});
