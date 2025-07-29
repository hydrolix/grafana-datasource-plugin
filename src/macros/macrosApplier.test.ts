import {
  emptyContext,
  parseMacroArgs,
  applyBaseMacros,
  applyAstAwareMacro,
} from "./macrosApplier";
import { dateTime, TimeRange, TypedVariableModel } from "@grafana/data";
import { getFilterExpression, getTableName } from "./macroFunctions";
import { adHocQueryAST } from "../__mocks__/ast";
import { SYNTHETIC_EMPTY, SYNTHETIC_NULL } from "../constants";
import { Context } from "../types";

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
    let context = {
      ...emptyContext,
      timeRange: {
        from: dateTime("2025-02-12T11:45:26.123Z"),
        to: dateTime("2025-02-13T11:45:26.456Z"),
        raw: {
          from: "",
          to: "",
        },
      },
    };
    const actual = await applyAstAwareMacro(
      await applyBaseMacros(origin, context),
      context
    );
    expect(actual).toEqual(interpolated);
  });
});
const SQL_WITH_FILTER =
  "SELECT column1, columnt2 FROM table WHERE $__adHocFilter()";
const SQL_WITHOUT_FILTER = "SELECT column1, columnt2 FROM table";

const context = {
  ...emptyContext,
  query: SQL_WITH_FILTER,
  ast: adHocQueryAST,
  adHocFilter: {
    filters: [],
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
    const actual = await applyAstAwareMacro(SQL_WITHOUT_FILTER, context);
    expect(actual).toEqual(SQL_WITHOUT_FILTER);
  });
  test("apply without filters", async () => {
    const actual = await applyAstAwareMacro(SQL_WITH_FILTER, context);
    expect(actual).toEqual("SELECT column1, columnt2 FROM table WHERE 1=1");
  });
  test("apply with filter", async () => {
    const actual = await applyAstAwareMacro(SQL_WITH_FILTER, {
      ...context,
      ast: adHocQueryAST,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [{ key: "column1", operator: "=", value: "value" }],
      },
      query: SQL_WITH_FILTER,
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = $$value$$"
    );
  });
  test("apply with filters", async () => {
    const actual = await applyAstAwareMacro(SQL_WITH_FILTER, {
      ...context,
      ast: adHocQueryAST,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [
          { key: "column1", operator: "=", value: "value" },
          { key: "column2", operator: "<", value: "value2" },
        ],
      },
      query: SQL_WITH_FILTER,
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = $$value$$ AND column2 < $$value2$$"
    );
  });
  test("apply and skip invalid column", async () => {
    const actual = await applyAstAwareMacro(SQL_WITH_FILTER, {
      ...context,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [
          { key: "column1", operator: "=", value: "value" },
          { key: "column2", operator: "<", value: "value2" },
          { key: "column3", operator: "=", value: "value3" },
        ],
      },
      ast: adHocQueryAST,
      query: SQL_WITH_FILTER,
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = $$value$$ AND column2 < $$value2$$"
    );
  });
  test("apply with all invalid columns", async () => {
    const actual = await applyAstAwareMacro(SQL_WITH_FILTER, {
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
    expect(actual).toEqual("column = $$$$value$$$$");
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
    expect(actual).toEqual("column < $$$$value$$$$");
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
    expect(actual).toEqual("toString(column) LIKE $$$$REGEX$$$$");
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
    expect(actual).toEqual("toString(column) LIKE $$$$%REGEX%$$$$");
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
    expect(actual).toEqual("toString(column) LIKE $$$$%*RE*GEX%$$$$");
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
    expect(actual).toEqual("toString(column) NOT LIKE $$$$REGEX$$$$");
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
    expect(actual).toEqual(
      "column IN ($$$$one$$$$, $$$$two$$$$, $$$$three$$$$)"
    );
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
    expect(actual).toEqual(
      "column NOT IN ($$$$one$$$$, $$$$two$$$$, $$$$three$$$$)"
    );
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
      "(column IN ($$$$one$$$$, $$$$__null__$$$$, $$$$two$$$$, $$$$three$$$$) OR column IS NULL)"
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
      "(column IN ($$$$one$$$$, $$$$two$$$$, $$$$three$$$$) OR column IS NULL)"
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
      "column NOT IN ($$$$one$$$$, $$$$two$$$$, $$$$three$$$$, $$$$__null__$$$$) AND column IS NOT NULL"
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
      "column NOT IN ($$$$one$$$$, $$$$two$$$$, $$$$three$$$$) AND column IS NOT NULL"
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
      "column IN ($$$$one$$$$, $$$$__empty__$$$$, $$$$two$$$$, $$$$three$$$$, $$$$$$$$)"
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
      "column NOT IN ($$$$one$$$$, $$$$two$$$$, $$$$three$$$$, $$$$__empty__$$$$, $$$$$$$$)"
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
      "(column IN ($$$$one$$$$, $$$$__empty__$$$$, $$$$two$$$$, $$$$three$$$$, $$$$__null__$$$$, $$$$$$$$) OR column IS NULL)"
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
      "column NOT IN ($$$$one$$$$, $$$$__null__$$$$, $$$$two$$$$, $$$$three$$$$, $$$$__empty__$$$$, $$$$$$$$) AND column IS NOT NULL"
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
    let result = await applyAstAwareMacro("SELECT $__timeInterval(column)", {
      ...emptyContext,
      intervalMs: 30000,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime(column), INTERVAL 30 second)"
    );
  });

  it("should apply macros with no interval", async () => {
    let result = await applyAstAwareMacro("SELECT $__timeInterval(column)", {
      ...emptyContext,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime(column), INTERVAL 1 second)"
    );
  });

  it("should apply macros with no params", async () => {
    let query = "SELECT $__timeInterval() from table";
    let result = await applyAstAwareMacro(query, {
      ...emptyContext,
      ast: JSON.parse(
        '{"SelectPos":0,"StatementEnd":35,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":{"Name":"$__timeInterval","QuoteType":1,"NamePos":7,"NameEnd":22},"Params":{"LeftParenPos":22,"RightParenPos":23,"Items":{"ListPos":23,"ListEnd":23,"HasDistinct":false,"Items":[]},"ColumnArgList":null}},"Modifiers":[],"Alias":null}],"From":{"FromPos":25,"Expr":{"Table":{"TablePos":30,"TableEnd":35,"Alias":null,"Expr":{"Database":null,"Table":{"Name":"table","QuoteType":1,"NamePos":30,"NameEnd":35}},"HasFinal":false},"StatementEnd":35,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":null,"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
      ),
      pk: () => Promise.resolve("ts"),
      query,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime(ts), INTERVAL 1 second) from table"
    );
  });

  it("should fail on empty ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeInterval()", {
        ...emptyContext,
        ast: {},
      });
    await expect(t()).rejects.toThrow(
      "cannot find table for macro $__timeInterval"
    );
  });

  it("should fail with on ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeInterval()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow("query ast is not provided");
  });

  it("should fail with 2 paramst", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeInterval(param1, param2)", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeInterval should not contain more than 1 parameter"
    );
  });
});

describe("$__timeFilter", () => {
  it("should apply macros", async () => {
    let result = await applyAstAwareMacro(
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
    let result = await applyAstAwareMacro(
      "SELECT * FROM table WHERE $__timeFilter(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should apply macros with no params", async () => {
    let query = "SELECT * FROM table WHERE $__timeFilter()";
    let result = await applyAstAwareMacro(query, {
      ...emptyContext,
      timeRange: {
        from: dateTime("2022-10-21T16:25:33"),
        to: dateTime("2022-10-25T16:25:33"),
        raw: {
          from: "",
          to: "",
        },
      },
      ast: JSON.parse(
        '{"SelectPos":0,"StatementEnd":40,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":"*","QuoteType":0,"NamePos":7,"NameEnd":7},"Modifiers":[],"Alias":null}],"From":{"FromPos":9,"Expr":{"Table":{"TablePos":14,"TableEnd":19,"Alias":null,"Expr":{"Database":null,"Table":{"Name":"table","QuoteType":1,"NamePos":14,"NameEnd":19}},"HasFinal":false},"StatementEnd":19,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":{"WherePos":20,"Expr":{"Name":{"Name":"$__timeFilter","QuoteType":1,"NamePos":26,"NameEnd":39},"Params":{"LeftParenPos":39,"RightParenPos":40,"Items":{"ListPos":40,"ListEnd":40,"HasDistinct":false,"Items":[]},"ColumnArgList":null}}},"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
      ),
      pk: () => Promise.resolve("ts"),
      query,
    });
    expect(result).toBe(
      "SELECT * FROM table WHERE ts >= toDateTime(1666369533) AND ts <= toDateTime(1666715133)"
    );
  });

  it("should fail on empty ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeFilter()", {
        ...emptyContext,
        ast: {},
      });
    await expect(t()).rejects.toThrow(
      "cannot find table for macro $__timeFilter"
    );
  });

  it("should fail with on ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeFilter()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow("query ast is not provided");
  });

  it("should fail with 2 params", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeFilter(param1, param2)", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeFilter should not contain more than 1 parameter"
    );
  });
});

describe("$__timeFilter_ms", () => {
  it("should apply macros", async () => {
    let result = await applyAstAwareMacro(
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
    let result = await applyAstAwareMacro(
      "SELECT * FROM table WHERE $__timeFilter_ms(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should apply macros with no params", async () => {
    let query = "SELECT * FROM table WHERE $__timeFilter_ms()";
    let result = await applyAstAwareMacro(query, {
      ...emptyContext,
      timeRange: {
        from: dateTime("2022-10-21T16:25:33"),
        to: dateTime("2022-10-25T16:25:33"),
        raw: {
          from: "",
          to: "",
        },
      },
      ast: JSON.parse(
        '{"SelectPos":0,"StatementEnd":43,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":"*","QuoteType":0,"NamePos":7,"NameEnd":7},"Modifiers":[],"Alias":null}],"From":{"FromPos":9,"Expr":{"Table":{"TablePos":14,"TableEnd":19,"Alias":null,"Expr":{"Database":null,"Table":{"Name":"table","QuoteType":1,"NamePos":14,"NameEnd":19}},"HasFinal":false},"StatementEnd":19,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":{"WherePos":20,"Expr":{"Name":{"Name":"$__timeFilter_ms","QuoteType":1,"NamePos":26,"NameEnd":42},"Params":{"LeftParenPos":42,"RightParenPos":43,"Items":{"ListPos":43,"ListEnd":43,"HasDistinct":false,"Items":[]},"ColumnArgList":null}}},"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
      ),
      pk: () => Promise.resolve("ts"),
      query,
    });
    expect(result).toBe(
      "SELECT * FROM table WHERE ts >= fromUnixTimestamp64Milli(1666369533000) AND ts <= fromUnixTimestamp64Milli(1666715133000)"
    );
  });

  it("should fail on empty ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeFilter_ms()", {
        ...emptyContext,
        ast: {},
      });
    await expect(t()).rejects.toThrow(
      "cannot find table for macro $__timeFilter_ms"
    );
  });

  it("should fail with on ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeFilter_ms()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow("query ast is not provided");
  });

  it("should fail with 2 params", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeFilter_ms(param1, param2)", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeFilter_ms should not contain more than 1 parameter"
    );
  });
});

describe("$__timeInterval_ms", () => {
  it("should apply macros", async () => {
    let result = await applyAstAwareMacro("SELECT $__timeInterval_ms(column)", {
      ...emptyContext,
      intervalMs: 30000,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime64(column, 3), INTERVAL 30000 millisecond)"
    );
  });
  it("should apply macros with no interval", async () => {
    let result = await applyAstAwareMacro("SELECT $__timeInterval_ms(column)", {
      ...emptyContext,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime64(column, 3), INTERVAL 1 millisecond)"
    );
  });

  it("should apply macros with no params", async () => {
    let query = "SELECT $__timeInterval_ms() from table";
    let result = await applyAstAwareMacro(query, {
      ...emptyContext,
      ast: JSON.parse(
        '{"SelectPos":0,"StatementEnd":38,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":{"Name":"$__timeInterval_ms","QuoteType":1,"NamePos":7,"NameEnd":25},"Params":{"LeftParenPos":25,"RightParenPos":26,"Items":{"ListPos":26,"ListEnd":26,"HasDistinct":false,"Items":[]},"ColumnArgList":null}},"Modifiers":[],"Alias":null}],"From":{"FromPos":28,"Expr":{"Table":{"TablePos":33,"TableEnd":38,"Alias":null,"Expr":{"Database":null,"Table":{"Name":"table","QuoteType":1,"NamePos":33,"NameEnd":38}},"HasFinal":false},"StatementEnd":38,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":null,"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
      ),
      pk: () => Promise.resolve("ts"),
      query,
    });
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime64(ts, 3), INTERVAL 1 millisecond) from table"
    );
  });

  it("should fail on empty ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeInterval_ms()", {
        ...emptyContext,
        ast: {},
      });
    await expect(t()).rejects.toThrow(
      "cannot find table for macro $__timeInterval_ms"
    );
  });

  it("should fail with on ast", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeInterval_ms()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow("query ast is not provided");
  });

  it("should fail with 2 params", async () => {
    let t = async () =>
      await applyAstAwareMacro("SELECT $__timeInterval_ms(param1, param2)", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macro $__timeInterval_ms should not contain more than 1 parameter"
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

describe("complex scenarios", () => {
  it("should apply ast dependent macros in query and subquery with adHocFilter", async () => {
    let query =
      "SELECT\n" +
      "  $__timeInterval(),\n" +
      "  count()\n" +
      "FROM\n" +
      "  table\n" +
      "WHERE\n" +
      "  $__timeFilter(ts)\n" +
      "  AND $__adHocFilter()\n" +
      "  AND statusCode IN (\n" +
      "    SELECT DISTINCT\n" +
      "      statusCode\n" +
      "    FROM\n" +
      "      table\n" +
      "    WHERE\n" +
      "      $__timeFilter(ts)\n" +
      "      AND $__adHocFilter()\n" +
      "  )\n" +
      "GROUP BY 1";
    let result = await applyAstAwareMacro(query, {
      ...emptyContext,
      adHocFilter: {
        ...context.adHocFilter,
        filters: [{ key: "column1", operator: "=", value: "value" }],
      },
      timeRange: {
        from: dateTime("2022-10-21T16:25:33"),
        to: dateTime("2022-10-25T16:25:33"),
        raw: {
          from: "",
          to: "",
        },
      },
      ast: JSON.parse(
        '{"SelectPos":0,"StatementEnd":255,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":{"Name":"$__timeInterval","QuoteType":1,"NamePos":9,"NameEnd":24},"Params":{"LeftParenPos":24,"RightParenPos":25,"Items":{"ListPos":25,"ListEnd":25,"HasDistinct":false,"Items":[]},"ColumnArgList":null}},"Modifiers":[],"Alias":null},{"Expr":{"Name":{"Name":"count","QuoteType":1,"NamePos":30,"NameEnd":35},"Params":{"LeftParenPos":35,"RightParenPos":36,"Items":{"ListPos":36,"ListEnd":36,"HasDistinct":false,"Items":[]},"ColumnArgList":null}},"Modifiers":[],"Alias":null}],"From":{"FromPos":38,"Expr":{"Table":{"TablePos":45,"TableEnd":50,"Alias":null,"Expr":{"Database":null,"Table":{"Name":"table","QuoteType":1,"NamePos":45,"NameEnd":50}},"HasFinal":false},"StatementEnd":50,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":{"WherePos":51,"Expr":{"LeftExpr":{"LeftExpr":{"Name":{"Name":"$__timeFilter","QuoteType":1,"NamePos":59,"NameEnd":72},"Params":{"LeftParenPos":72,"RightParenPos":75,"Items":{"ListPos":73,"ListEnd":75,"HasDistinct":false,"Items":[{"Expr":{"Name":"ts","QuoteType":1,"NamePos":73,"NameEnd":75},"Alias":null}]},"ColumnArgList":null}},"Operation":"AND","RightExpr":{"Name":{"Name":"$__adHocFilter","QuoteType":1,"NamePos":83,"NameEnd":97},"Params":{"LeftParenPos":97,"RightParenPos":98,"Items":{"ListPos":98,"ListEnd":98,"HasDistinct":false,"Items":[]},"ColumnArgList":null}},"HasGlobal":false,"HasNot":false},"Operation":"AND","RightExpr":{"LeftExpr":{"Name":"statusCode","QuoteType":1,"NamePos":106,"NameEnd":116},"Operation":"IN","RightExpr":{"HasParen":true,"Select":{"SelectPos":126,"StatementEnd":245,"With":null,"Top":null,"HasDistinct":true,"SelectItems":[{"Expr":{"Name":"statusCode","QuoteType":1,"NamePos":148,"NameEnd":158},"Modifiers":[],"Alias":null}],"From":{"FromPos":163,"Expr":{"Table":{"TablePos":174,"TableEnd":179,"Alias":null,"Expr":{"Database":null,"Table":{"Name":"table","QuoteType":1,"NamePos":174,"NameEnd":179}},"HasFinal":false},"StatementEnd":179,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":{"WherePos":184,"Expr":{"LeftExpr":{"Name":{"Name":"$__timeFilter","QuoteType":1,"NamePos":196,"NameEnd":209},"Params":{"LeftParenPos":209,"RightParenPos":212,"Items":{"ListPos":210,"ListEnd":212,"HasDistinct":false,"Items":[{"Expr":{"Name":"ts","QuoteType":1,"NamePos":210,"NameEnd":212},"Alias":null}]},"ColumnArgList":null}},"Operation":"AND","RightExpr":{"Name":{"Name":"$__adHocFilter","QuoteType":1,"NamePos":224,"NameEnd":238},"Params":{"LeftParenPos":238,"RightParenPos":239,"Items":{"ListPos":239,"ListEnd":239,"HasDistinct":false,"Items":[]},"ColumnArgList":null}},"HasGlobal":false,"HasNot":false}},"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}},"HasGlobal":false,"HasNot":false},"HasGlobal":false,"HasNot":false}},"GroupBy":{"GroupByPos":245,"GroupByEnd":255,"AggregateType":"","Expr":{"ListPos":254,"ListEnd":255,"HasDistinct":false,"Items":[{"Expr":{"NumPos":254,"NumEnd":255,"Literal":"1","Base":10},"Alias":null}]},"WithCube":false,"WithRollup":false,"WithTotals":false},"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
      ),
      pk: () => Promise.resolve("ts"),
      query,
    });
    expect(result).toBe(
      "SELECT\n" +
        "  toStartOfInterval(toDateTime(ts), INTERVAL 1 second),\n" +
        "  count()\n" +
        "FROM\n" +
        "  table\n" +
        "WHERE\n" +
        "  ts >= toDateTime(1666369533) AND ts <= toDateTime(1666715133)\n" +
        "  AND column1 = $$value$$\n" +
        "  AND statusCode IN (\n" +
        "    SELECT DISTINCT\n" +
        "      statusCode\n" +
        "    FROM\n" +
        "      table\n" +
        "    WHERE\n" +
        "      ts >= toDateTime(1666369533) AND ts <= toDateTime(1666715133)\n" +
        "      AND column1 = $$value$$\n" +
        "  )\n" +
        "GROUP BY 1"
    );
  });
});

describe("get table name", () => {
  it("should return the table", async () => {
    let tableName = getTableName(
      "$__macro",
      {
        ast: JSON.parse(
          '{"SelectPos":0,"StatementEnd":35,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":"*","QuoteType":0,"NamePos":7,"NameEnd":7},"Modifiers":[],"Alias":null}],"From":{"FromPos":9,"Expr":{"Table":{"TablePos":14,"TableEnd":19,"Alias":null,"Expr":{"Database":null,"Table":{"Name":"table","QuoteType":1,"NamePos":14,"NameEnd":19}},"HasFinal":false},"StatementEnd":19,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":{"WherePos":20,"Expr":{"Name":{"Name":"$__macro","QuoteType":1,"NamePos":26,"NameEnd":34},"Params":{"LeftParenPos":34,"RightParenPos":35,"Items":{"ListPos":35,"ListEnd":35,"HasDistinct":false,"Items":[]},"ColumnArgList":null}}},"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
        ),
        query: "SELECT * FROM table WHERE $__macro()",
      } as Context,
      (node) => node.Where,
      26
    );
    expect(tableName).toBe("table");
  });
  it("should return the table with schema", async () => {
    let tableName = getTableName(
      "$__macro",
      {
        ast: JSON.parse(
          '{"SelectPos":0,"StatementEnd":42,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":"*","QuoteType":0,"NamePos":7,"NameEnd":7},"Modifiers":[],"Alias":null}],"From":{"FromPos":9,"Expr":{"Table":{"TablePos":14,"TableEnd":26,"Alias":null,"Expr":{"Database":{"Name":"schema","QuoteType":1,"NamePos":14,"NameEnd":20},"Table":{"Name":"table","QuoteType":1,"NamePos":21,"NameEnd":26}},"HasFinal":false},"StatementEnd":26,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":{"WherePos":27,"Expr":{"Name":{"Name":"$__macro","QuoteType":1,"NamePos":33,"NameEnd":41},"Params":{"LeftParenPos":41,"RightParenPos":42,"Items":{"ListPos":42,"ListEnd":42,"HasDistinct":false,"Items":[]},"ColumnArgList":null}}},"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
        ),
        query: "SELECT * FROM schema.table WHERE $__macro()",
      } as Context,
      (node) => node.Where,
      33
    );
    expect(tableName).toBe("schema.table");
  });

  it("should return the table and schema with alias", async () => {
    let tableName = getTableName(
      "$__macro",
      {
        ast: JSON.parse(
          '{"SelectPos":0,"StatementEnd":48,"With":null,"Top":null,"HasDistinct":false,"SelectItems":[{"Expr":{"Name":"*","QuoteType":0,"NamePos":7,"NameEnd":7},"Modifiers":[],"Alias":null}],"From":{"FromPos":9,"Expr":{"Table":{"TablePos":14,"TableEnd":32,"Alias":null,"Expr":{"Expr":{"Database":{"Name":"schema","QuoteType":1,"NamePos":14,"NameEnd":20},"Table":{"Name":"table","QuoteType":1,"NamePos":21,"NameEnd":26}},"AliasPos":30,"Alias":{"Name":"t1","QuoteType":1,"NamePos":30,"NameEnd":32}},"HasFinal":false},"StatementEnd":32,"SampleRatio":null,"HasFinal":false}},"ArrayJoin":null,"Window":null,"Prewhere":null,"Where":{"WherePos":33,"Expr":{"Name":{"Name":"$__macro","QuoteType":1,"NamePos":39,"NameEnd":47},"Params":{"LeftParenPos":47,"RightParenPos":48,"Items":{"ListPos":48,"ListEnd":48,"HasDistinct":false,"Items":[]},"ColumnArgList":null}}},"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"Format":null,"UnionAll":null,"UnionDistinct":null,"Except":null}'
        ),
        query: "SELECT * FROM schema.table as t1 WHERE $__macro()",
      } as Context,
      (node) => node.Where,
      39
    );
    expect(tableName).toBe("schema.table");
  });
});
