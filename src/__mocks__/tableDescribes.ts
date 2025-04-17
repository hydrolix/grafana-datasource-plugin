export const DESCRIBE1 = [
  {
    name: "toStartOfHour(datetime)",
    type: "DateTime",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "if(less(modulo(cityHash64(request_path, client_ip, datetime, request_query_params), 100), 1), request_path, '~~~SAMPLED_OUT~~~')",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "count()",
    type: "AggregateFunction(count)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "countIf(and(greater(status_code, 199), less(status_code, 300)))",
    type: "AggregateFunction(countIf, Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "countIf(and(greater(status_code, 399), less(status_code, 500)))",
    type: "AggregateFunction(countIf, Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "countIf(and(greater(status_code, 499), less(status_code, 600)))",
    type: "AggregateFunction(countIf, Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "countIf(isNotNull(status_code))",
    type: "AggregateFunction(countIf, UInt8)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "avg(response_body_bytes)",
    type: "AggregateFunction(avg, Nullable(UInt64))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "quantiles(0.75, 0.99)(response_time_to_first_byte)",
    type: "AggregateFunction(quantiles(0.75, 0.99), Nullable(Float64))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "quantiles(0.75, 0.99)(response_time_to_last_byte)",
    type: "AggregateFunction(quantiles(0.75, 0.99), Nullable(Float64))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sum(response_total_bytes)",
    type: "AggregateFunction(sum, Nullable(UInt64))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "hour_ts",
    type: "DateTime",
    default_type: "ALIAS",
    default_expression: "`toStartOfHour(datetime)`",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sampled_request_path",
    type: "Nullable(String)",
    default_type: "ALIAS",
    default_expression:
      "`if(less(modulo(cityHash64(request_path, client_ip, datetime, request_query_params), 100), 1), request_path, '~~~SAMPLED_OUT~~~')`",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_all",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "countMerge(`count()`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_2xx_status_code",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression:
      "countIfMerge(`countIf(and(greater(status_code, 199), less(status_code, 300)))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_4xx_status_code",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression:
      "countIfMerge(`countIf(and(greater(status_code, 399), less(status_code, 500)))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_5xx_status_code",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression:
      "countIfMerge(`countIf(and(greater(status_code, 499), less(status_code, 600)))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_status_code",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "countIfMerge(`countIf(isNotNull(status_code))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "avg_body_bytes",
    type: "Nullable(Float64)",
    default_type: "ALIAS",
    default_expression: "avgMerge(`avg(response_body_bytes)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "ttfb_p75_p99",
    type: "Array(Float64)",
    default_type: "ALIAS",
    default_expression:
      "quantilesMerge(0.75, 0.99)(`quantiles(0.75, 0.99)(response_time_to_first_byte)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "ttlb_p75_p99",
    type: "Array(Float64)",
    default_type: "ALIAS",
    default_expression:
      "quantilesMerge(0.75, 0.99)(`quantiles(0.75, 0.99)(response_time_to_last_byte)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sum_total_bytes",
    type: "Nullable(UInt64)",
    default_type: "ALIAS",
    default_expression: "sumMerge(`sum(response_total_bytes)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
];

export const DESCRIBE2 = [
  {
    name: "toStartOfMinute(reqTimeSec)",
    type: "DateTime",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "statusCode",
    type: "Nullable(UInt32)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "reqHost",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "city",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "state",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "country",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cacheable",
    type: "Nullable(UInt8)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "errorCode",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "reqMethod",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "rspContentType",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "proto",
    type: "Nullable(String)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cacheStatus",
    type: "Nullable(UInt8)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cp",
    type: "Nullable(UInt32)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "count()",
    type: "AggregateFunction(count)",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "countIf(equals(cacheStatus, 0))",
    type: "AggregateFunction(countIf, Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "countIf(equals(cacheStatus, 1))",
    type: "AggregateFunction(countIf, Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sum(totalBytes)",
    type: "AggregateFunction(sum, Nullable(UInt64))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sumIf(totalBytes, equals(cacheStatus, 0))",
    type: "AggregateFunction(sumIf, Nullable(UInt64), Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sumIf(totalBytes, equals(cacheStatus, 1))",
    type: "AggregateFunction(sumIf, Nullable(UInt64), Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "countIf(greater(statusCode, 400))",
    type: "AggregateFunction(countIf, Nullable(UInt8))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "uniq(cliIP)",
    type: "AggregateFunction(uniq, Nullable(String))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "uniq(cliIP, UA)",
    type: "AggregateFunction(uniq, Nullable(String), Nullable(String))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "quantiles(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(turnAroundTimeMSec)",
    type: "AggregateFunction(quantiles(0.25, 0.5, 0.75, 0.9, 0.95, 0.99), Nullable(UInt32))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "quantiles(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(transferTimeMSec)",
    type: "AggregateFunction(quantiles(0.25, 0.5, 0.75, 0.9, 0.95, 0.99), Nullable(UInt32))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "topK(50)(cliIP)",
    type: "AggregateFunction(topK(50), Nullable(String))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "topK(50)(UA)",
    type: "AggregateFunction(topK(50), Nullable(String))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "topK(50)(reqPath)",
    type: "AggregateFunction(topK(50), Nullable(String))",
    default_type: "",
    default_expression: "",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "timestamp_min",
    type: "DateTime",
    default_type: "ALIAS",
    default_expression: "`toStartOfMinute(reqTimeSec)`",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_all",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "countMerge(`count()`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_originHits",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "countIfMerge(`countIf(equals(cacheStatus, 0))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_edgeHits",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "countIfMerge(`countIf(equals(cacheStatus, 1))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "pct_offLoadHit",
    type: "Float64",
    default_type: "ALIAS",
    default_expression:
      "(countIfMerge(`countIf(equals(cacheStatus, 1))`) / countMerge(`count()`)) * 100",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sum_totalBytes",
    type: "Nullable(UInt64)",
    default_type: "ALIAS",
    default_expression: "sumMerge(`sum(totalBytes)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sum_BytesOrigin",
    type: "Nullable(UInt64)",
    default_type: "ALIAS",
    default_expression:
      "sumIfMerge(`sumIf(totalBytes, equals(cacheStatus, 0))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "sum_BytesEdge",
    type: "Nullable(UInt64)",
    default_type: "ALIAS",
    default_expression:
      "sumIfMerge(`sumIf(totalBytes, equals(cacheStatus, 1))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "pct_offLoadBytes",
    type: "Nullable(Float64)",
    default_type: "ALIAS",
    default_expression:
      "(sumIfMerge(`sumIf(totalBytes, equals(cacheStatus, 1))`) / sumMerge(`sum(totalBytes)`)) * 100",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "cnt_errorHits",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "countIfMerge(`countIf(greater(statusCode, 400))`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "pct_errorRate",
    type: "Float64",
    default_type: "ALIAS",
    default_expression:
      "(countIfMerge(`countIf(greater(statusCode, 400))`) / countMerge(`count()`)) * 100",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "uniq_cliIP",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "uniqMerge(`uniq(cliIP)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "uniq_session",
    type: "UInt64",
    default_type: "ALIAS",
    default_expression: "uniqMerge(`uniq(cliIP, UA)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "quantiles_turnAroundTimeMSec",
    type: "Array(Float64)",
    default_type: "ALIAS",
    default_expression:
      "quantilesMerge(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(`quantiles(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(turnAroundTimeMSec)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "quantiles_transferTimeMSec",
    type: "Array(Float64)",
    default_type: "ALIAS",
    default_expression:
      "quantilesMerge(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(`quantiles(0.25, 0.5, 0.75, 0.9, 0.95, 0.99)(transferTimeMSec)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "top_50_cliIP",
    type: "Array(String)",
    default_type: "ALIAS",
    default_expression: "topKMerge(50)(`topK(50)(cliIP)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "top_50_UA",
    type: "Array(String)",
    default_type: "ALIAS",
    default_expression: "topKMerge(50)(`topK(50)(UA)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
  {
    name: "top_50_reqPath",
    type: "Array(String)",
    default_type: "ALIAS",
    default_expression: "topKMerge(50)(`topK(50)(reqPath)`)",
    comment: "",
    codec_expression: "",
    ttl_expression: "",
  },
];
