import {
    DataSourceInstanceSettings,
    PluginType,
} from '@grafana/data';
import {DataSource} from '../datasource';
import {HdxDataSourceOptions} from "../types";

export const MockHdxSettings: DataSourceInstanceSettings<HdxDataSourceOptions> = {
    jsonData: {},
    id: 0,
    uid: '',
    type: '',
    name: 'Mock Hydrolix Data Source',
    meta: {
        id: '',
        name: '',
        type: PluginType.datasource,
        info: {
            author: {
                name: '',
            },
            description: '',
            links: [],
            logos: {
                large: '',
                small: '',
            },
            screenshots: [],
            updated: '',
            version: '',
        },
        module: '',
        baseUrl: '',
    },
    readOnly: false,
    access: 'direct',
};

export const mockHdxDataSource = (): DataSource => {
    return new DataSource(MockHdxSettings);
};
