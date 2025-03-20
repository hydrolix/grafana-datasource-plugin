import { DataSourceJsonData } from "@grafana/data";
import { DataQuery } from "@grafana/schema";

export interface HdxQuery extends DataQuery {
  rawSql: string;
  round: string;
  format?: number;
}

/**
 * QueryType determines the display/query format.
 */
export enum QueryType {
  Table = 1,
  TimeSeries = 0,
  Logs = 2,
  Traces = 3,
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
  defaultTable?: string;
  adHocTableVariable?: string;
  adHocKeyQuery?: string;
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
