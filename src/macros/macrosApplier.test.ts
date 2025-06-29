import {
  emptyContext,
  parseMacroArgs,
  applyBaseMacros,
  applyAdHocMacro,
} from "./macrosApplier";
import { dateTime, TimeRange, TypedVariableModel } from "@grafana/data";
import { getFilterExpression } from "./macroFunctions";
import { adHocQueryAST } from "../__mocks__/ast";
import { SYNTHETIC_EMPTY, SYNTHETIC_NULL } from "../constants";

describe("macros base applier", () => {
  it("apply with one macro", async () => {
    let result = await applyBaseMacros("query with $__interval_s()", {
      ...emptyContext,
    });
    expect(result).toBe("query with 1");
  });
  it("apply with multiple macro", async () => {
    let result = await applyBaseMacros(
      "query with $__interval_s() $__interval_s() $__interval_s()",
      {
        ...emptyContext,
      }
    );
    expect(result).toBe("query with 1 1 1");
  });
  it("apply with no macro", async () => {
    let result = await applyBaseMacros("query with", {
      ...emptyContext,
    });
    expect(result).toBe("query with");
  });
  it("apply with no existing macro", async () => {
    let result = await applyBaseMacros("query with $__test", {
      ...emptyContext,
    });
    expect(result).toBe("query with $__test");
  });

  it("apply without query", async () => {
    let result = await applyBaseMacros("", {
      ...emptyContext,
    });
    expect(result).toBe("");
  });
});
describe("macros parse params", () => {
  it("parse multiple params", () => {
    let rawQuery =
      "select 1 from table where $__test(1, 'word', func(), $__mac())";

    const params = parseMacroArgs(rawQuery, rawQuery.indexOf("("));

    expect(params).toStrictEqual(["1", " 'word'", " func()", " $__mac()"]);
  });
});

describe("macros interpolation", () => {
  const cases = [
    {
      origin: "SELECT * FROM foo WHERE $__timeFilter(cast(col as timestamp))",
      interpolated:
        "SELECT * FROM foo WHERE cast(col as timestamp) >= toDateTime(1739360726.123) AND cast(col as timestamp) <= toDateTime(1739447126.456)",
      name: "timeFilter",
    },
    {
      origin: "SELECT * FROM foo WHERE $__timeFilter( cast(col as timestamp) )",
      interpolated:
        "SELECT * FROM foo WHERE  cast(col as timestamp)  >= toDateTime(1739360726.123) AND  cast(col as timestamp)  <= toDateTime(1739447126.456)",
      name: "timeFilter with whitespaces",
    },
    {
      origin:
        "SELECT * FROM foo WHERE $__timeFilter_ms(cast(col as timestamp))",
      interpolated:
        "SELECT * FROM foo WHERE cast(col as timestamp) >= fromUnixTimestamp64Milli(1739360726123) AND cast(col as timestamp) <= fromUnixTimestamp64Milli(1739447126456)",
      name: "timeFilter_ms",
    },
    {
      origin:
        "SELECT * FROM foo WHERE $__timeFilter_ms( cast(col as timestamp) )",
      interpolated:
        "SELECT * FROM foo WHERE  cast(col as timestamp)  >= fromUnixTimestamp64Milli(1739360726123) AND  cast(col as timestamp)  <= fromUnixTimestamp64Milli(1739447126456)",
      name: "timeFilter_ms with whitespaces",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime and col <= $__toTime ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= toDateTime(1739360726.123) and col <= toDateTime(1739447126.456) ) limit 100",
      name: "fromTime and toTime",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime ) and ( col <= $__toTime ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= toDateTime(1739360726.123) ) and ( col <= toDateTime(1739447126.456) ) limit 100",
      name: "fromTime and toTime condition #2",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime_ms and col <= $__toTime_ms ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= fromUnixTimestamp64Milli(1739360726123) and col <= fromUnixTimestamp64Milli(1739447126456) ) limit 100",
      name: "fromTime_ms and toTime_ms",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime_ms ) and ( col <= $__toTime_ms ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= fromUnixTimestamp64Milli(1739360726123) ) and ( col <= fromUnixTimestamp64Milli(1739447126456) ) limit 100",
      name: "fromTime_ms and toTime_ms condition #2",
    },
  ];

  it.each(cases)("$name", async ({ origin, interpolated }) => {
    const actual = await applyBaseMacros(origin, {
      ...emptyContext,
      timeRange: {
        from: dateTime("2025-02-12T11:45:26.123Z"),
        to: dateTime("2025-02-13T11:45:26.456Z"),
        raw: {
          from: "",
          to: "",
        },
      },
    });
    expect(actual).toEqual(interpolated);
  });
});
const SQL_WITH_FILTER =
  "SELECT column1, columnt2 FROM table WHERE $__adHocFilter()";
const SQL_WITHOUT_FILTER = "SELECT column1, columnt2 FROM table";

const context = {
  ...emptyContext,
  query: SQL_WITH_FILTER,
  adHocFilter: {
    filters: [],
    ast: adHocQueryAST,
    keys: () =>
      Promise.resolve([
        {
          text: "column1",
          type: "Nullable(String)",
        },
        {
          text: "column2",
          type: "Nullable(UInt64)",
        },
      ]),
  },
};
describe("$__adHocFilter", () => {
  test("apply to query without macros", async () => {
    const actual = await applyAdHocMacro(SQL_WITHOUT_FILTER, context);
    expect(actual).toEqual(SQL_WITHOUT_FILTER);
  });
  test("apply without filters", async () => {
    const actual = await applyAdHocMacro(SQL_WITH_FILTER, context);
    expect(actual).toEqual("SELECT column1, columnt2 FROM table WHERE 1=1");
  });
  test("apply with filter", async () => {
    const actual = await applyAdHocMacro(SQL_WITH_FILTER, {
      ...context,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [{ key: "column1", operator: "=", value: "value" }],
        ast: adHocQueryAST,
      },
      query: SQL_WITH_FILTER,
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = 'value'"
    );
  });
  test("apply with filters", async () => {
    const actual = await applyAdHocMacro(SQL_WITH_FILTER, {
      ...context,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [
          { key: "column1", operator: "=", value: "value" },
          { key: "column2", operator: "<", value: "value2" },
        ],
        ast: adHocQueryAST,
      },
      query: SQL_WITH_FILTER,
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = 'value' AND column2 < 'value2'"
    );
  });
  test("apply and skip invalid column", async () => {
    const actual = await applyAdHocMacro(SQL_WITH_FILTER, {
      ...context,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [
          { key: "column1", operator: "=", value: "value" },
          { key: "column2", operator: "<", value: "value2" },
          { key: "column3", operator: "=", value: "value3" },
        ],
        ast: adHocQueryAST,
      },
      query: SQL_WITH_FILTER,
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = 'value' AND column2 < 'value2'"
    );
  });
  test("apply with all invalid columns", async () => {
    const actual = await applyAdHocMacro(SQL_WITH_FILTER, {
      ...context,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [
          { key: "column4", operator: "=", value: "value" },
          { key: "column5", operator: "<", value: "value2" },
          { key: "column3", operator: "=", value: "value3" },
        ],
      },
    });
    expect(actual).toEqual("SELECT column1, columnt2 FROM table WHERE 1=1");
  });
});

describe("$__adHocFilter getFilterExpression", () => {
  test("eq expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=",
        value: "value",
      },
      false
    );
    expect(actual).toEqual("column = 'value'");
  });

  test("eq empty sting expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=",
        value: SYNTHETIC_EMPTY,
      },
      false
    );
    expect(actual).toEqual("(column = '' OR column = '__empty__')");
  });
  test("lg expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "<",
        value: "value",
      },
      false
    );
    expect(actual).toEqual("column < 'value'");
  });

  test("eq null expression string ", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=",
        value: "null",
      },
      true
    );
    expect(actual).toEqual("(column IS NULL OR column = '__null__')");
  });
  test("eq null expression non string ", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=",
        value: "null",
      },
      false
    );
    expect(actual).toEqual("column IS NULL");
  });

  test("neq null expression string", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=",
        value: "null",
      },
      true
    );
    expect(actual).toEqual("column IS NOT NULL AND column != '__null__'");
  });

  test("neq null expression not string", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=",
        value: "null",
      },
      false
    );
    expect(actual).toEqual("column IS NOT NULL");
  });

  test("lg null expression string", async () => {
    const actual = () =>
      getFilterExpression(
        {
          key: "column",
          operator: "<",
          value: "null",
        },
        true
      );
    expect(actual).toThrow(
      "column: operator '<' can not be applied to NULL value"
    );
  });
  test("lg null expression non string", async () => {
    const actual = () =>
      getFilterExpression(
        {
          key: "column",
          operator: "<",
          value: "null",
        },
        false
      );
    expect(actual).toThrow(
      "column: operator '<' can not be applied to NULL value"
    );
  });

  test("eq synthetic null expression string", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=",
        value: SYNTHETIC_NULL,
      },
      true
    );
    expect(actual).toEqual("(column IS NULL OR column = '__null__')");
  });
  test("eq synthetic null expression non string", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=",
        value: SYNTHETIC_NULL,
      },
      false
    );
    expect(actual).toEqual("column IS NULL");
  });

  test("neq synthetic null expression string", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=",
        value: SYNTHETIC_NULL,
      },
      true
    );
    expect(actual).toEqual("column IS NOT NULL AND column != '__null__'");
  });

  test("neq synthetic null expression non string", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=",
        value: SYNTHETIC_NULL,
      },
      false
    );
    expect(actual).toEqual("column IS NOT NULL");
  });

  test("lg synthetic null expression string", async () => {
    const actual = () =>
      getFilterExpression(
        {
          key: "column",
          operator: "<",
          value: SYNTHETIC_NULL,
        },
        true
      );
    expect(actual).toThrow(
      "column: operator '<' can not be applied to NULL value"
    );
  });
  test("lg synthetic null expression non string", async () => {
    const actual = () =>
      getFilterExpression(
        {
          key: "column",
          operator: "<",
          value: SYNTHETIC_NULL,
        },
        false
      );
    expect(actual).toThrow(
      "column: operator '<' can not be applied to NULL value"
    );
  });

  test("eq regex expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=~",
        value: "REGEX",
      },
      true
    );
    expect(actual).toEqual("toString(column) LIKE 'REGEX'");
  });

  test("eq regex wildcard expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=~",
        value: "*REGEX*",
      },
      true
    );
    expect(actual).toEqual("toString(column) LIKE '%REGEX%'");
  });

  test("eq regex escaped wildcard expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=~",
        value: "*\\*RE\\*GEX*",
      },
      true
    );
    expect(actual).toEqual("toString(column) LIKE '%*RE*GEX%'");
  });

  test("neq regex expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!~",
        value: "REGEX",
      },
      true
    );
    expect(actual).toEqual("toString(column) NOT LIKE 'REGEX'");
  });

  test("one of expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=|",
        // @ts-ignore
        values: ["one", "two", "three"],
      },
      true
    );
    expect(actual).toEqual("column IN ('one', 'two', 'three')");
  });

  test("not one of expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=|",
        // @ts-ignore
        values: ["one", "two", "three"],
      },
      true
    );
    expect(actual).toEqual("column NOT IN ('one', 'two', 'three')");
  });

  test("one of with string null expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=|",
        // @ts-ignore
        values: ["one", SYNTHETIC_NULL, "two", "three"],
      },
      true
    );
    expect(actual).toEqual(
      "(column IN ('one', '__null__', 'two', 'three') OR column IS NULL)"
    );
  });
  test("one of with non string null expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=|",
        // @ts-ignore
        values: ["one", SYNTHETIC_NULL, "two", "three"],
      },
      false
    );
    expect(actual).toEqual(
      "(column IN ('one', 'two', 'three') OR column IS NULL)"
    );
  });

  test("not one of with string null expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=|",
        // @ts-ignore
        values: ["one", "two", "three", SYNTHETIC_NULL],
      },
      true
    );
    expect(actual).toEqual(
      "column NOT IN ('one', 'two', 'three', '__null__') AND column IS NOT NULL"
    );
  });
  test("not one of with non string null expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=|",
        // @ts-ignore
        values: ["one", "two", "three", SYNTHETIC_NULL],
      },
      false
    );
    expect(actual).toEqual(
      "column NOT IN ('one', 'two', 'three') AND column IS NOT NULL"
    );
  });

  test("one of with empty expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=|",
        // @ts-ignore
        values: ["one", SYNTHETIC_EMPTY, "two", "three"],
      },
      true
    );
    expect(actual).toEqual(
      "column IN ('one', '__empty__', 'two', 'three', '')"
    );
  });

  test("not one of with empty expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=|",
        // @ts-ignore
        values: ["one", "two", "three", SYNTHETIC_EMPTY],
      },
      true
    );
    expect(actual).toEqual(
      "column NOT IN ('one', 'two', 'three', '__empty__', '')"
    );
  });

  test("one of with null and empty expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "=|",
        // @ts-ignore
        values: ["one", SYNTHETIC_EMPTY, "two", "three", SYNTHETIC_NULL],
      },
      true
    );
    expect(actual).toEqual(
      "(column IN ('one', '__empty__', 'two', 'three', '__null__', '') OR column IS NULL)"
    );
  });

  test("not one of with null and empty expression", async () => {
    const actual = getFilterExpression(
      {
        key: "column",
        operator: "!=|",
        // @ts-ignore
        values: ["one", SYNTHETIC_NULL, "two", "three", SYNTHETIC_EMPTY],
      },
      true
    );
    expect(actual).toEqual(
      "column NOT IN ('one', '__null__', 'two', 'three', '__empty__', '') AND column IS NOT NULL"
    );
  });
});

describe("$__conditionalAll", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  const cases: Array<{
    name: string;
    query: string;
    variables: TypedVariableModel[];
    expected: string;
  }> = [
    {
      name: "should replace $__conditionalAll with 1=1 when all is selected",
      query:
        "select foo from table where $__conditionalAll(bar in ($bar), $bar);",
      variables: [{ name: "bar", current: { value: "$__all" } } as any],
      expected: "select foo from table where 1=1;",
    },
    {
      name: "should replace $__conditionalAll with arg when anything else is selected",
      query:
        "select foo from table where $__conditionalAll(bar in ($bar), $bar);",
      variables: [{ name: "bar", current: { value: `'val1', 'val2'` } } as any],
      expected: "select foo from table where bar in ($bar);",
    },
    {
      name: "should replace all $__conditionalAll",
      query:
        "select foo from table where $__conditionalAll(bar in ($bar), $bar) and $__conditionalAll(bar in ($bar2), $bar2);",
      variables: [
        { name: "bar", current: { value: `'val1', 'val2'` } } as any,
        { name: "bar2", current: { value: "$__all" } } as any,
      ],
      expected: "select foo from table where bar in ($bar) and 1=1;",
    },
  ];

  test.each(cases)("$name", async ({ query, variables, expected }) => {
    const actual = await applyBaseMacros(query, {
      ...emptyContext,
      templateVars: variables,
    });
    expect(actual).toEqual(expected);
  });

  test("should fail with 1 param", async () => {
    const t = () =>
      applyBaseMacros(
        "select foo from table where $__conditionalAll(bar in ($bar));",
        {
          ...emptyContext,
        }
      );
    await expect(t()).rejects.toThrow(
      "Macro $__conditionalAll should contain 2 parameters"
    );
  });
});

describe("$__dateFilter", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__dateFilter(date)",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21"),
          to: dateTime("2022-10-25"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= toDate('2022-10-21') AND date <= toDate('2022-10-25')"
    );
  });

  it("should apply without timerange", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__dateFilter(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT * FROM table WHERE $__dateFilter()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__dateFilter should contain 1 parameter"
    );
  });
});

describe("$__dateTimeFilter", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__dateTimeFilter(date, time)",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21T19:23:44"),
          to: dateTime("2022-10-25T03:17:44"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= toDate('2022-10-21') AND date <= toDate('2022-10-25') AND  time >= toDateTime(1666380224) AND  time <= toDateTime(1666667864)"
    );
  });

  it("should apply without timerange", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__dateTimeFilter(date, time)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await applyBaseMacros(
        "SELECT * FROM table WHERE $__dateTimeFilter(date)",
        {
          ...emptyContext,
        }
      );
    await expect(t()).rejects.toThrow(
      "Macro $__dateTimeFilter should contain 2 parameters"
    );
  });

  it("should apply short macros", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__dt(date, time)",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21T19:23:44"),
          to: dateTime("2022-10-25T03:17:44"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= toDate('2022-10-21') AND date <= toDate('2022-10-25') AND  time >= toDateTime(1666380224) AND  time <= toDateTime(1666667864)"
    );
  });
});

describe("$__fromTime", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros("SELECT $__fromTime()", {
      ...emptyContext,
      timeRange: {
        from: dateTime("2022-10-21T19:23:44"),
        to: dateTime("2022-10-25T03:17:44"),
        raw: {
          from: "",
          to: "",
        },
      },
    });
    expect(result).toBe("SELECT toDateTime(1666380224)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT $__fromTime()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macro without time range");
  });
});

describe("$__fromTime_ms", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros("SELECT $__fromTime_ms()", {
      ...emptyContext,
      timeRange: {
        from: dateTime("2022-10-21T19:23:44"),
        to: dateTime("2022-10-25T03:17:44"),
        raw: {
          from: "",
          to: "",
        },
      },
    });
    expect(result).toBe("SELECT fromUnixTimestamp64Milli(1666380224000)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT $__fromTime_ms()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macro without time range");
  });
});

describe("$__interval_s", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros("SELECT $__interval_s()", {
      ...emptyContext,
      intervalMs: 30000,
    });
    expect(result).toBe("SELECT 30");
  });

  it("should apply with 0", async () => {
    let result = await applyBaseMacros("SELECT $__interval_s()", {
      ...emptyContext,
      intervalMs: 0,
    });
    expect(result).toBe("SELECT 1");
  });

  it("should apply with null", async () => {
    let result = await applyBaseMacros("SELECT $__interval_s()", {
      ...emptyContext,
    });
    expect(result).toBe("SELECT 1");
  });
});

describe("$__timeInterval", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros("SELECT $__timeInterval(column)", {
      ...emptyContext,
      intervalMs: 30000,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime(column), INTERVAL 30 second)"
    );
  });

  it("should apply macros with no interval", async () => {
    let result = await applyBaseMacros("SELECT $__timeInterval(column)", {
      ...emptyContext,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime(column), INTERVAL 1 second)"
    );
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT $__timeInterval()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeInterval should contain 1 parameter"
    );
  });
});

describe("$__timeFilter", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__timeFilter(date)",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21T16:25:33"),
          to: dateTime("2022-10-25T16:25:33"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= toDateTime(1666369533) AND date <= toDateTime(1666715133)"
    );
  });

  it("should apply without timerange", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__timeFilter(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT * FROM table WHERE $__timeFilter()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeFilter should contain 1 parameter"
    );
  });
});

describe("$__timeFilter_ms", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__timeFilter_ms(date)",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21T16:25:33"),
          to: dateTime("2022-10-25T16:25:33"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= fromUnixTimestamp64Milli(1666369533000) AND date <= fromUnixTimestamp64Milli(1666715133000)"
    );
  });

  it("$__timeFilter_ms", async () => {
    let result = await applyBaseMacros(
      "SELECT * FROM table WHERE $__timeFilter_ms(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT * FROM table WHERE $__timeFilter_ms()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeFilter_ms should contain 1 parameter"
    );
  });
});

describe("$__timeInterval_ms", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros("SELECT $__timeInterval_ms(column)", {
      ...emptyContext,
      intervalMs: 30000,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime64(column, 3), INTERVAL 30000 millisecond)"
    );
  });
  it("should apply macros with no interval", async () => {
    let result = await applyBaseMacros("SELECT $__timeInterval_ms(column)", {
      ...emptyContext,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime64(column, 3), INTERVAL 1 millisecond)"
    );
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT $__timeInterval_ms()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeInterval_ms should contain 1 parameter"
    );
  });
});

describe("$__toTime", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros("SELECT $__toTime()", {
      ...emptyContext,
      timeRange: {
        from: dateTime("2022-10-21T19:23:44"),
        to: dateTime("2022-10-25T03:17:44"),
        raw: {
          from: "",
          to: "",
        },
      },
    });
    expect(result).toBe("SELECT toDateTime(1666667864)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT $__toTime()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macro without time range");
  });
});

describe("$__toTime_ms", () => {
  it("should apply macros", async () => {
    let result = await applyBaseMacros("SELECT $__toTime_ms()", {
      ...emptyContext,
      timeRange: {
        from: dateTime("2022-10-21T19:23:44"),
        to: dateTime("2022-10-25T03:17:44"),
        raw: {
          from: "",
          to: "",
        },
      },
    });
    expect(result).toBe("SELECT fromUnixTimestamp64Milli(1666667864000)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await applyBaseMacros("SELECT $__toTime_ms()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macro without time range");
  });
});
