import { getColumnValuesStatement, getTable } from "./ast";
const SQL_WITH_VARIABLES =
  "SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(${timefilter}) as time, ${count} as http\n" +
  "FROM ${table}\n" +
  "WHERE $__timeFilter(${timefilter})\n" +
  "AND $__conditionalAll( statusCode ${AND_statusCode} (${statusCode:sqlstring}), $statusCode)\n" +
  "AND $__conditionalAll( reqHost ${AND_reqHost} (${reqHost:sqlstring}), $reqHost)\n" +
  "AND $__conditionalAll( cacheStatus ${AND_cacheStatus} (${cacheStatus:sqlstring}), $cacheStatus)\n" +
  "AND $__conditionalAll( reqMethod ${AND_reqMethod} (${reqMethod:sqlstring}), $reqMethod)\n" +
  "AND $__conditionalAll( rspContentType ${AND_rspContentType} (${rspContentType:sqlstring}), $rspContentType)\n" +
  "AND $__conditionalAll( errorCode ${AND_errorCode} (${errorCode:sqlstring}), $errorCode)\n" +
  "AND $__conditionalAll( country ${AND_country} (${country:sqlstring}), $country)\n" +
  "AND $__conditionalAll( cp ${AND_cp} (${cp:sqlstring}), $cp)\n" +
  "AND ${filters}\n" +
  "AND $__adHocFilter() \n" +
  "GROUP BY HTTP_Status_Code, time ORDER BY time\n" +
  "SETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment='db=${__dashboard}; dp=HTTP Status Code; du=${__user.login}'";

const SQL_WITHOUT_VARIABLES =
  "SELECT statusCode::String, * EXCEPT (unknown)\n" +
  "FROM summary.logs\n" +
  "WHERE $__timeFilter(reqTimeSec) AND $__adHocFilter()\n" +
  "ORDER BY reqTimeSec DESC\n" +
  "LIMIT 1000";

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

describe("ast getTable", () => {
  test("return table name for statement with variables", () => {
    let result = getTable(SQL_WITH_VARIABLES);
    expect(result).toBe("${table}");
  });

  test("return table name for statement without variables", () => {
    let result = getTable(SQL_WITHOUT_VARIABLES);
    expect(result).toBe("summary.logs");
  });
});
