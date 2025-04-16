import { DataSourceJsonData, TimeRange } from "@grafana/data";
import { DataQuery } from "@grafana/schema";

export interface HdxQuery extends DataQuery {
  rawSql: string;
  round: string;
  queryFormat?: string;
  format?: number;
  skipNextRun?: () => boolean;
}

/**
 * QueryType determines the display/query format.
 */
export enum QueryType {
  Table = 1,
  TimeSeries = 0,
  Logs = 2,
}

export const DEFAULT_QUERY: Partial<HdxQuery> = {};

export interface DataPoint {
  Time: number;
  Value: number;
}

export interface DataSourceResponse {
  datapoints: DataPoint[];
}

/**
 * These are options configured for each DataSource instance
 */
export interface HdxDataSourceOptions extends DataSourceJsonData {
  host?: string;
  port?: number;
  useDefaultPort?: boolean;
  username?: string;
  protocol?: Protocol;
  secure?: boolean;
  path?: string;
  skipTlsVerify?: boolean;
  defaultDatabase?: string;
  defaultRound?: string;
  adHocDefaultTimeRange?: TimeRange;
  adHocTableVariable?: string;
  adHocTimeColumnVariable?: string;
  adHocKeysQuery?: string;
  adHocValuesQuery?: string;
  dialTimeout?: string;
  queryTimeout?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface HdxSecureJsonData {
  password?: string;
}

export enum Protocol {
  Native = "native",
  Http = "http",
}

export interface AdHocFilterKeys {
  text: string;
  value?: string | number;
  group: string;
}
