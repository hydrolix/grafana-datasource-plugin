import { getColumnValuesStatement } from "./ast";

describe("ast getColumnValuesStatement", () => {
  test("should return topK statement for a plain column", () => {
    let result = getColumnValuesStatement("statusCode", "sample.log", "ts", "");
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(statusCode)) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should return topK statement for city for table with variables", () => {
    let result = getColumnValuesStatement("city", "sample.log", "ts", "");
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(city)) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should return topK statement for statusCode with condition", () => {
    let result = getColumnValuesStatement(
      "statusCode",
      "sample.log",
      "ts",
      "toString(statusCode) like '2%'"
    );
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(statusCode)) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter() AND toString(statusCode) like '2%' SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should use arrayJoin expansion for Array-typed columns", () => {
    let result = getColumnValuesStatement(
      "arrayJoin(tags)",
      "sample.log",
      "ts",
      ""
    );
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(arrayJoin(tags))) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should support map-access column expressions", () => {
    let result = getColumnValuesStatement(
      "attributes['env']",
      "sample.log",
      "ts",
      ""
    );
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(attributes['env'])) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should not contain GROUP BY or ORDER BY count", () => {
    let result = getColumnValuesStatement("clientIP", "sample.log", "ts", "");
    expect(result).not.toContain("GROUP BY");
    expect(result).not.toContain("ORDER BY count");
  });

  test("should carry both SETTINGS entries", () => {
    let result = getColumnValuesStatement("clientIP", "sample.log", "ts", "");
    expect(result).toContain("timeout_overflow_mode = 'break'");
    expect(result).toContain("hdx_query_max_timerange_sec = 87000");
  });
});
