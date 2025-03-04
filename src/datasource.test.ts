import { of } from 'rxjs';
import {toDataFrame} from '@grafana/data';
import { setupDataSourceMock } from '__mocks__/datasource';
import { fooVariable } from "./__mocks__/variable";

describe('HdxDataSource', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    describe('When performing metricFindQuery', () => {
        beforeEach(() => {
            jest.clearAllMocks();
        });

        const cases: Array<{
            name: string;
            response: any;
            expected: any;
        }> = [
            {
                name: 'it should return values',
                response: {
                    fields: [{name: 'values', type: 'number', values: [100, 200]}],
                },
                expected: [
                    { text: 100, value: 100 },
                    { text: 200, value: 200 }
                ]
            },
            {
                name: 'it should return identified values',
                response: {
                    fields: [
                        {name: 'ids', type: 'number', values: [1, 2]},
                        {name: 'values', type: 'number', values: [100, 200]}
                    ],
                },
                expected: [
                    { text: 100, value: 1 },
                    { text: 200, value: 2 }
                ]
            }
        ]

        const { datasource, queryMock } = setupDataSourceMock({})

        test.each(cases)('$name', async ({ response, expected }) => {
            queryMock.mockImplementation((_) => of({data: [toDataFrame(response)]}))
            const actual = await datasource.metricFindQuery('mock', {});
            expect(actual).toEqual(expected);
        });
    })

    const filterQueryCases: Array<{ query: string; valid: boolean }> = [
        { query: "", valid: false },
        { query: "select 1;", valid: true }
    ]

    test.each(filterQueryCases)('should filter out invalid query', ({query, valid}) => {
        const { datasource } = setupDataSourceMock({});
        const actual = datasource.filterQuery({
            refId: '',
            rawSql: query,
            round: '',
        });
        expect(actual).toEqual(valid)
    })

    it('should interpolate variables in the query', async () => {
        const { datasource } = setupDataSourceMock({
            variables: [fooVariable],
        });
        const actual = datasource.applyTemplateVariables({
            refId: '',
            rawSql: 'foo $foo',
            round: '',
        }, {});
        expect(actual.rawSql).toEqual('foo templatedFoo');
    });
});
