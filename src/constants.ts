export const SHOW_VALIDATION_BAR = false;
export const SHOW_INTERPOLATED_QUERY_ERRORS = false;

export const SYNTHETIC_NULL = "__null__";
export const SYNTHETIC_EMPTY = "__empty__";

export const SCHEMA_SQL =
  "SELECT DISTINCT database as project FROM system.tables WHERE engine = 'TurbineStorage' AND (project != 'sample_project' AND project != 'hdx' AND total_rows > 0)";
export const TABLES_SQL =
  "SELECT name FROM system.tables WHERE engine = 'TurbineStorage' AND database = '{schema}' AND total_rows > 0";
export const COLUMNS_SQL =
  "SELECT name FROM system.columns WHERE database='{schema}' AND table ='{table}'";
export const FUNCTIONS_SQL = "SELECT name FROM  system.functions";

export const AD_HOC_KEY_QUERY = "DESCRIBE ${table}";
export const AD_HOC_VALUE_QUERY =
  "SELECT ${column}, COUNT(${column}) as count  FROM ${table} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter() GROUP BY ${column} ORDER BY count DESC LIMIT 100";

export const SUPPORTED_TYPES = [
  "DateTime",
  "DateTime64",
  "String",
  "Int8",
  "Int16",
  "Int32",
  "Int64",
  "Int128",
  "Int256",
  "UInt8",
  "UInt16",
  "UInt32",
  "UInt64",
  "UInt128",
  "UInt256",
  "Float32",
  "Float64",
  "Decimal32",
  "Decimal64",
  "Decimal128",
  "Decimal256",
];

export const NULLABLE_TYPES = SUPPORTED_TYPES.map((t) => `Nullable(${t})`);

export const VARIABLE_REGEX = /(?<=\$\{)\w+(?=})|(?<=\$)\w+/;

export const DATE_FORMAT = "YYYY-MM-DD";
