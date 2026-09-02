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
export const PK_SQL =
  "SELECT primary_key FROM system.tables WHERE database='{schema}' AND table ='{table}'";
export const FUNCTIONS_SQL = "SELECT name FROM  system.functions";

export const AD_HOC_VALUE_TOP_K = 100;

export const AD_HOC_PRELOAD_LOOKBACK_SECONDS = 86400;
export const AD_HOC_PRELOAD_ROUND_INTERVAL = "5m";
export const AD_HOC_PRELOAD_ROUND_INTERVAL_SECONDS = 300;
export const AD_HOC_PRELOAD_MAX_TIMERANGE_SECONDS =
  AD_HOC_PRELOAD_LOOKBACK_SECONDS + 2 * AD_HOC_PRELOAD_ROUND_INTERVAL_SECONDS;

// Stand-in for the panel's maxDataPoints when the editor has no panel request
// to read it from. The interpolation preview only needs a representative bucket
// width, not the exact one the panel will run with.
export const DEFAULT_INTERPOLATION_RESOLUTION = 1000;

export const METADATA_QUERY_TIMEOUT_SETTING = "hdx_query_max_execution_time";
export const METADATA_QUERY_TIMEOUT_SETTING_ALIAS = "max_execution_time";
export const METADATA_QUERY_TIMEOUT_VALUE = "10";

const AD_HOC_QUERY_GUARDRAIL_SETTINGS = `SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = ${AD_HOC_PRELOAD_MAX_TIMERANGE_SECONDS}`;

export const AD_HOC_KEY_QUERY = "DESCRIBE ${table}";
export const AD_HOC_MAP_KEY_QUERY = `SELECT distinct(arrayJoin(mapKeys(\${column}))) FROM \${table} WHERE $__timeFilter() AND $__adHocFilter() ${AD_HOC_QUERY_GUARDRAIL_SETTINGS}`;
export const AD_HOC_VALUE_QUERY = `SELECT arrayJoin(topK(${AD_HOC_VALUE_TOP_K})(\${column})) AS value FROM \${table} WHERE $__timeFilter(\${timeColumn}) AND $__adHocFilter() \${condition} ${AD_HOC_QUERY_GUARDRAIL_SETTINGS}`;

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
  "UUID",
  "IPv4",
  "IPv6",
];

export const NULLABLE_TYPES = SUPPORTED_TYPES.map((t) => `Nullable(${t})`);
export const ARRAY_TYPES = [...SUPPORTED_TYPES, ...NULLABLE_TYPES].map(
  (t) => `Array(${t})`
);
export const MAP_TYPES = [...SUPPORTED_TYPES, ...NULLABLE_TYPES].map(
  (t) => `Map(String, ${t})`
);
export const NULLABLE_MAP_TYPES = NULLABLE_TYPES.map(
  (t) => `Map(String, ${t})`
);

export const VARIABLE_REGEX = /(?<=\$\{)\w+(?=})|(?<=\$)\w+/;

export const MAP_KEY_REGEX = /^.*\['.*']$/;

export const DATE_FORMAT = "YYYY-MM-DD";
