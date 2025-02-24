import {
  DataSourceInstanceSettings,
  CoreApp,
  ScopedVars,
  DataQueryRequest,
  DataQueryResponse,
  DataQueryError
} from '@grafana/data';
import {DataSourceWithBackend, getTemplateSrv} from '@grafana/runtime';

import { HdxQuery, HdxDataSourceOptions, DEFAULT_QUERY } from './types';
import { Observable } from "rxjs";
import { map } from 'rxjs/operators'
import { ErrorMessageBeautifier } from "./errorBeautifier";

export class DataSource extends DataSourceWithBackend<HdxQuery, HdxDataSourceOptions> {
  private readonly beautifier = new ErrorMessageBeautifier()

  constructor(instanceSettings: DataSourceInstanceSettings<HdxDataSourceOptions>) {
    super(instanceSettings);
  }

  query(request: DataQueryRequest<HdxQuery>): Observable<DataQueryResponse> {
    return super.query(request).pipe(map((response: DataQueryResponse) => {
      const errors = response.errors?.map((error: DataQueryError) => {
        if (error.message) {
          const message = this.beautifier.beautify(error.message)
          if (message) {
            return {...error, message: message}
          }
        }
        return error
      })
      return {...response, errors: errors};
    }));
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
