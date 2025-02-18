import {DataSourceInstanceSettings, CoreApp, ScopedVars, DataQueryRequest, DataQueryResponse} from '@grafana/data';
import {DataSourceWithBackend, getTemplateSrv} from '@grafana/runtime';

import { HdxQuery, HdxDataSourceOptions, DEFAULT_QUERY } from './types';
import {Observable} from "rxjs";

export class DataSource extends DataSourceWithBackend<HdxQuery, HdxDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<HdxDataSourceOptions>) {
    super(instanceSettings);
  }

  query(request: DataQueryRequest<HdxQuery>): Observable<DataQueryResponse> {
    return super.query(request);
  }

  getDefaultQuery(_: CoreApp): Partial<HdxQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: HdxQuery, scopedVars: ScopedVars) {
    return {
      ...query,
      rawSql: getTemplateSrv().replace(query.rawSql, scopedVars),
    };
  }

  filterQuery(query: HdxQuery): boolean {
    // if no query has been provided, prevent the query from being executed
    return !!query.rawSql;
  }
}
