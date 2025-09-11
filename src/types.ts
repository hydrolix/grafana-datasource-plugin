import {
  AdHocVariableFilter,
  DataSourceJsonData,
  TimeRange,
  TypedVariableModel,
} from "@grafana/data";
import { DataQuery } from "@grafana/schema";

export interface HdxQuery extends DataQuery {
  rawSql: string;
  round: string;
  queryFormat?: string;
  filters?: AdHocVariableFilter[];
  format?: number;
  skipNextRun?: () => boolean;
  querySettings: { [setting: string]: string };
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
  credentialsType?: CredentialsType;
  username?: string;
  protocol?: Protocol;
  token?: string;
  secure?: boolean;
  path?: string;
  skipTlsVerify?: boolean;
  defaultDatabase?: string;
  defaultRound?: string;
  adHocDefaultTimeRange?: TimeRange;
  adHocTableVariable?: string;
  adHocConditionVariable?: string;
  dialTimeout?: string;
  queryTimeout?: string;
  querySettings?: QuerySetting[];
}

export interface QuerySetting {
  setting: string;
  value: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface HdxSecureJsonData {
  password?: string;
  token?: string;
}

export enum Protocol {
  Native = "native",
  Http = "http",
}

export enum CredentialsType {
  UserAccount = "userAccount",
  ServiceAccount = "serviceAccount",
}

export interface AdHocFilterKeys {
  text: string;
  value?: string | number;
  group?: string;
  type: string;
}

export interface MacroCTEResponse extends ResourceResponse<MacroCTE[]> {}
export interface InterpolationResponse extends ResourceResponse<string> {}

export interface ResourceResponse<T> {
  originalSql: string;
  error: boolean;
  errorMessage: string;
  data: T;
}

export interface InterpolationResult {
  originalSql: string;
  interpolationId: string;
  interpolatedSql?: string;
  finalSql?: string;
  hasError: boolean;
  hasWarning: boolean;
  error?: string;
  warning?: string;
}

export interface ValidationResult {
  error?: string;
  warning?: string;
}

export interface SelectQuery {
  SelectItems: SelectItem[];
  From: Expr;
  Where: Expr;
  GroupBy: GroupBy;
  OrderBy: OrderBy;
  Settings: Settings;
}

export interface Settings {
  Items: Expr[];
}
export interface SelectItem {
  Expr: Expr;
  Modifiers: [];
  Alias: Alias;
}
interface OrderBy {
  Items: Expr[];
}
interface GroupBy {
  AggregateType: string;
  Expr: Expr;
}
export interface Alias {
  Name: string;
}
export interface ColumnExprList {
  HasDistinct: boolean;
  Items: Expr[];
}
export interface Params {
  Items: ColumnExprList;
}
export interface TableIdentifier {}

export interface Expr {
  Expr: Expr;
  Name: string;
  Literal: string;
  Alias: Alias;
  Direction: string;
  Params: Params;
  LeftExpr: Expr;
  RightExpr: Expr;
  Table: Expr;
  Database: Expr;
  Operation: string;
}

export interface MacroFunctionMap {
  [macro: string]: (
    params: string[],
    context: Context,
    index: number
  ) => string;
}

export interface Context {
  templateVars: TypedVariableModel[];
  query: string;
}

export interface MacroCTE {
  macro: string;
  macroPos: number;
  cte: string;
}
