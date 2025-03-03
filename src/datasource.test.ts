import { of } from 'rxjs';
import {TypedVariableModel, toDataFrame,} from '@grafana/data';
import { mockHdxDataSource } from '__mocks__/datasource';
import { HdxQuery } from "./types";

interface InstanceConfig {
    queryResponse: {} | [];
}

const templateSrvMock = { replace: jest.fn(), getVariables: jest.fn(), getAdhocFilters: jest.fn() };

// noinspection JSUnusedGlobalSymbols
jest.mock('@grafana/runtime', () => ({
    ...(jest.requireActual('@grafana/runtime') as unknown as object),
    getTemplateSrv: () => templateSrvMock,
}));

const createInstance = ({ queryResponse }: Partial<InstanceConfig> = {}) => {
    const instance = mockHdxDataSource();
    jest.spyOn(instance, 'query').mockImplementation((_request) => of({ data: [toDataFrame(queryResponse ?? [])] }));
    return instance;
};

describe('HdxDataSource', () => {
    describe('metricFindQuery', () => {
        it('fetches values', async () => {
            const mockedValues = [1, 100];
            const queryResponse = {
                fields: [{ name: 'field', type: 'number', values: mockedValues }],
            };
            const expectedValues = mockedValues.map((v) => ({ text: v, value: v }));
            const values = await createInstance({ queryResponse }).metricFindQuery('mock', {});
            expect(values).toEqual(expectedValues);
        });

        it('fetches name/value pairs', async () => {
            const mockedIds = [1, 2];
            const mockedValues = [100, 200];
            const queryResponse = {
                fields: [
                    { name: 'id', type: 'number', values: mockedIds },
                    { name: 'values', type: 'number', values: mockedValues },
                ],
            };
            const expectedValues = mockedValues.map((v, i) => ({ text: v, value: mockedIds[i] }));
            const values = await createInstance({ queryResponse }).metricFindQuery('mock', {});
            expect(values).toEqual(expectedValues);
        });
    });

    describe('applyTemplateVariables', () => {
        it('interpolates', async () => {
            const rawSql = 'foo';
            const spyOnReplace = jest.spyOn(templateSrvMock, 'replace').mockImplementation(() => rawSql);
            const query = { rawSql: 'select' } as HdxQuery;
            const val = createInstance({}).applyTemplateVariables(query, {});
            expect(spyOnReplace).toHaveBeenCalled();
            expect(val).toEqual({ rawSql });
        });
        it('should handle $__conditionalAll and not replace', async () => {
            const query = { rawSql: '$__conditionalAll(foo, $fieldVal)' } as HdxQuery;
            const vars = [{ current: { value: `'val1', 'val2'` }, name: 'fieldVal' }] as TypedVariableModel[];
            const spyOnReplace = jest.spyOn(templateSrvMock, 'replace').mockImplementation((x) => x);
            const spyOnGetVars = jest.spyOn(templateSrvMock, 'getVariables').mockImplementation(() => vars);
            const val = createInstance({}).applyTemplateVariables(query, {});
            expect(spyOnReplace).toHaveBeenCalled();
            expect(spyOnGetVars).toHaveBeenCalled();
            expect(val).toEqual({ rawSql: `foo` });
        });
        it('should handle $__conditionalAll and replace', async () => {
            const query = { rawSql: '$__conditionalAll(foo, $fieldVal)' } as HdxQuery;
            const vars = [{ current: { value: '$__all' }, name: 'fieldVal' }] as TypedVariableModel[];
            const spyOnReplace = jest.spyOn(templateSrvMock, 'replace').mockImplementation((x) => x);
            const spyOnGetVars = jest.spyOn(templateSrvMock, 'getVariables').mockImplementation(() => vars);
            const val = createInstance({}).applyTemplateVariables(query, {});
            expect(spyOnReplace).toHaveBeenCalled();
            expect(spyOnGetVars).toHaveBeenCalled();
            expect(val).toEqual({ rawSql: `1=1` });
        });
    });

    describe('Conditional All', () => {
        it('should replace $__conditionalAll with 1=1 when all is selected', async () => {
            const rawSql = 'select stuff from table where $__conditionalAll(fieldVal in ($fieldVal), $fieldVal);';
            const val = createInstance({}).applyConditionalAll(rawSql, [
                { name: 'fieldVal', current: { value: '$__all' } } as any,
            ]);
            expect(val).toEqual('select stuff from table where 1=1;');
        });
        it('should replace $__conditionalAll with arg when anything else is selected', async () => {
            const rawSql = 'select stuff from table where $__conditionalAll(fieldVal in ($fieldVal), $fieldVal);';
            const val = createInstance({}).applyConditionalAll(rawSql, [
                { name: 'fieldVal', current: { value: `'val1', 'val2'` } } as any,
            ]);
            expect(val).toEqual(`select stuff from table where fieldVal in ($fieldVal);`);
        });
        it('should replace all $__conditionalAll', async () => {
            const rawSql =
                'select stuff from table where $__conditionalAll(fieldVal in ($fieldVal), $fieldVal) and $__conditionalAll(fieldVal in ($fieldVal2), $fieldVal2);';
            const val = createInstance({}).applyConditionalAll(rawSql, [
                { name: 'fieldVal', current: { value: `'val1', 'val2'` } } as any,
                { name: 'fieldVal2', current: { value: '$__all' } } as any,
            ]);
            expect(val).toEqual(`select stuff from table where fieldVal in ($fieldVal) and 1=1;`);
        });
    });
});
