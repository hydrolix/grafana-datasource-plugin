import {
    CoreApp,
    DataFrame,
    DataQueryError,
    DataQueryRequest,
    DataQueryResponse,
    DataSourceInstanceSettings,
    ScopedVars,
    TestDataSourceResponse,
    getTimeZone,
    getTimeZoneInfo
} from '@grafana/data';
import {DataSourceWithBackend, getTemplateSrv, TemplateSrv} from '@grafana/runtime';
import {isString} from 'lodash';
import { DEFAULT_QUERY, HdxDataSourceOptions, HdxQuery } from './types';
import {Observable} from "rxjs";
import {map} from 'rxjs/operators'
import {ErrorMessageBeautifier} from "./errorBeautifier";
import {ConditionalAllApplier} from "./conditionalAllApplier";

export class DataSource extends DataSourceWithBackend<HdxQuery, HdxDataSourceOptions> {
    private readonly beautifier = new ErrorMessageBeautifier()
    private readonly conditionalAllApplier = new ConditionalAllApplier()

    constructor(
        instanceSettings: DataSourceInstanceSettings<HdxDataSourceOptions>,
        readonly templateSrv: TemplateSrv = getTemplateSrv()
    ) {
        super(instanceSettings);
    }

    async metricFindQuery(query: HdxQuery | string, options: any) {
        const hdxQuery = isString(query) ? {rawSql: query,} : query;
        if (!hdxQuery.rawSql) {
            return [];
        }

        const frame = await this.runQuery(hdxQuery, options);
        if (frame.fields?.length === 0) {
            return [];
        }
        if (frame?.fields?.length === 1) {
            return frame?.fields[0]?.values.map((text) => ({text, value: text}));
        }
        // convention - assume the first field is an id field
        const ids = frame?.fields[0]?.values;
        return frame?.fields[1]?.values.map((text, i) => ({text, value: ids[i]}));
    }

    query(request: DataQueryRequest<HdxQuery>): Observable<DataQueryResponse> {
        const targets = request.targets
            .map((t) => {
                return {
                    ...t,
                    meta: {
                        timezone: this.resolveTimezone(request),
                    },
                };
            });

        return super.query({
            ...request,
            targets,
        }).pipe(map((response: DataQueryResponse) => {
            const errors = response.errors?.map((error: DataQueryError) => {
                console.error(error)
                if (error.message) {
                    const message = this.beautifier.beautify(error.message)
                    if (message) {
                        return { ...error, message: message }
                    }
                }
                return error
            })

            return {
                ...response,
                errors: errors,
                error: undefined
            };
        }));
    }

    getDefaultQuery(_: CoreApp): Partial<HdxQuery> {
        return DEFAULT_QUERY;
    }

    applyTemplateVariables(query: HdxQuery, scoped: ScopedVars): HdxQuery {
        let rawQuery = query.rawSql || '';
        rawQuery = this.conditionalAllApplier.apply(rawQuery, this.templateSrv.getVariables());
        return {
            ...query,
            rawSql: this.replace(rawQuery, scoped) || '',
        };
    }

    filterQuery(query: HdxQuery): boolean {
        // if no query has been provided, prevent the query from being executed
        return !!query.rawSql;
    }

    private runQuery(request: Partial<HdxQuery>, options?: any): Promise<DataFrame> {
        return new Promise((resolve) => {
            const req = {
                targets: [{...request, refId: String(Math.random())}],
                range: options ? options.range : (this.templateSrv as any).timeRange,
            } as DataQueryRequest<HdxQuery>;
            this.query(req).subscribe((res: DataQueryResponse) => {
                resolve(res.data[0] || {fields: []});
            });
        });
    }

    private replace(value?: string, scopedVars?: ScopedVars) {
        if (value !== undefined) {
            return this.templateSrv.replace(value, scopedVars);
        }
        return value;
    }

    private resolveTimezone(request: DataQueryRequest<HdxQuery>): string | undefined {
        // timezone specified in the time picker
        if (request.timezone && request.timezone !== 'browser') {
            return request.timezone;
        }
        // fall back to the local timezone
        const localTimezoneInfo = getTimeZoneInfo(getTimeZone(), Date.now());
        return localTimezoneInfo?.ianaName;
    }

    testDatasource(): Promise<TestDataSourceResponse> {
        return super.testDatasource().catch(error => {
            console.log('catch', error);
            if (error.message) {
                const message = this.beautifier.beautify(error.message)
                if (message) {
                    return {...error, message: message}
                }
            }
            return error
        });
    }
}
