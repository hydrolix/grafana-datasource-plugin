import {
  emptyContext,
  parseMacroArgs,
  applyBaseMacros,
  applyAstAwareMacro,
} from "./macrosApplier";
import { TypedVariableModel } from "@grafana/data";
import { getFilterExpression } from "./macroFunctions";
import { adHocQueryAST } from "../__mocks__/ast";
import { SYNTHETIC_EMPTY, SYNTHETIC_NULL } from "../constants";

describe("macros parse params", () => {
  it("parse multiple params", () => {
    let rawQuery =
      "select 1 from table where $__test(1, 'word', func(), $__mac())";

    const params = parseMacroArgs(rawQuery, rawQuery.indexOf("("));

    expect(params).toStrictEqual(["1", " 'word'", " func()", " $__mac()"]);
  });
});

const SQL_WITH_FILTER =
  "SELECT column1, columnt2 FROM table WHERE $__adHocFilter()";
const MACRO_CTE = {
  macro: "$__adHocFilter",
  macroPos: 42,
  cte: "table",
  table: "table",
  database: "",
  pos: 30,
};

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
      macroCTE: [MACRO_CTE],
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
      macroCTE: [MACRO_CTE],
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
      macroCTE: [MACRO_CTE],
      query: SQL_WITH_FILTER,
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = $$value$$ AND column2 < $$value2$$"
    );
  });
  test("apply with all invalid columns", async () => {
    const actual = await applyAstAwareMacro(SQL_WITH_FILTER, {
      ...context,
      macroCTE: [MACRO_CTE],
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
