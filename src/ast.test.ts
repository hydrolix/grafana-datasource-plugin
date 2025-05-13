import { getColumnValuesStatement } from "./ast";

describe("ast getColumnValuesStatement", () => {
  const TEMPLATE =
    "SELECT ${column}, COUNT(${column}) as count  FROM ${table} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter() GROUP BY ${column} ORDER BY count DESC LIMIT 100";

  test("should return statement for statusCode for table with variables", () => {
    let result = getColumnValuesStatement(
      "statusCode",
      "sample.log",
      "ts",
      TEMPLATE
    );
    expect(result).toBe(
      "SELECT statusCode, COUNT(statusCode) as count  FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter() GROUP BY statusCode ORDER BY count DESC LIMIT 100"
    );
  });

  test("should return statement for city for table with variables", () => {
    let result = getColumnValuesStatement("city", "sample.log", "ts", TEMPLATE);
    expect(result).toBe(
      "SELECT city, COUNT(city) as count  FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter() GROUP BY city ORDER BY count DESC LIMIT 100"
    );
  });
});
