import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export interface HdxQuery extends DataQuery {
  rawSql: string;
  round: string;
}

/**
 * QueryType determines the display/query format.
 */
export enum QueryType {
  Graph = 'graph',
  Table = 'table',
  Logs = 'logs',
  TimeSeries = 'timeseries',
  Traces = 'traces',
}

export const DEFAULT_QUERY: Partial<HdxQuery> = {
};

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
  dialTimeout?: string
  queryTimeout?: string
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface HdxSecureJsonData {
  password?: string;
}

export enum Protocol {
  Native = 'native',
  Http = 'http',
}
