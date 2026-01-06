import {
  applyConditionalAll,
  emptyContext,
  parseMacroArgs,
} from "./macrosApplier";
import { TypedVariableModel } from "@grafana/data";

describe("macros parse params", () => {
  it("parse multiple params", () => {
    let rawQuery =
      "select 1 from table where $__test(1, 'word', func(), $__mac())";

    const params = parseMacroArgs(rawQuery, rawQuery.indexOf("("));

    expect(params).toStrictEqual(["1", "'word'", "func()", "$__mac()"]);
  });

  it("parse params with no arguments", () => {
    let rawQuery = "select 1 from table where $__test()";
    const params = parseMacroArgs(rawQuery, rawQuery.indexOf("("));
    expect(params).toStrictEqual([""]);
  });

  it("parse params with nested parentheses", () => {
    let rawQuery = "select 1 from table where $__test(func(a, b), other)";
    const params = parseMacroArgs(rawQuery, rawQuery.indexOf("("));
    expect(params).toStrictEqual(["func(a, b)", "other"]);
  });

  it("return empty array for unclosed parenthesis", () => {
    let rawQuery = "select 1 from table where $__test(arg1, arg2";
    const params = parseMacroArgs(rawQuery, rawQuery.indexOf("("));
    expect(params).toStrictEqual([]);
  });
});

// Ad-hoc filter tests moved to Go backend tests in pkg/datasource/macros_test.go

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

  test.each(cases)("$name", ({ query, variables, expected }) => {
    const actual = applyConditionalAll(query, {
      ...emptyContext,
      templateVars: variables,
    });
    expect(actual).toEqual(expected);
  });

  test("should fail with 1 param", () => {
    const t = () =>
      applyConditionalAll(
        "select foo from table where $__conditionalAll(bar in ($bar));",
        {
          ...emptyContext,
        }
      );
    try {
      t();
      fail();
    } catch (e) {
      let err = e as Error;
      expect(err.message).toStrictEqual(
        "Macro $__conditionalAll should contain 2 parameters"
      );
    }
  });
});

describe("macro escaping", () => {
  const cases: Array<{
    name: string;
    query: string;
    variables: TypedVariableModel[];
    expected: string;
  }> = [
    {
      name: "should escape macro with double dollar sign",
      query: "SELECT * FROM table WHERE $$__conditionalAll(bar in ($bar), $bar)",
      variables: [{ name: "bar", current: { value: "$__all" } } as any],
      expected: "SELECT * FROM table WHERE $__conditionalAll(bar in ($bar), $bar)",
    },
    {
      name: "should handle mix of escaped and unescaped macros",
      query: "SELECT * FROM table WHERE $__conditionalAll(bar in ($bar), $bar) AND $$__conditionalAll(baz in ($baz), $baz)",
      variables: [
        { name: "bar", current: { value: "$__all" } } as any,
        { name: "baz", current: { value: "'val'" } } as any,
      ],
      expected: "SELECT * FROM table WHERE 1=1 AND $__conditionalAll(baz in ($baz), $baz)",
    },
    {
      name: "should escape multiple macros",
      query: "SELECT $$__conditionalAll(bar in ($bar), $bar) FROM table WHERE $$__conditionalAll(baz in ($baz), $baz)",
      variables: [
        { name: "bar", current: { value: "$__all" } } as any,
        { name: "baz", current: { value: "$__all" } } as any,
      ],
      expected: "SELECT $__conditionalAll(bar in ($bar), $bar) FROM table WHERE $__conditionalAll(baz in ($baz), $baz)",
    },
    {
      name: "should process unescaped macro normally",
      query: "SELECT * FROM table WHERE $__conditionalAll(bar in ($bar), $bar)",
      variables: [{ name: "bar", current: { value: "$__all" } } as any],
      expected: "SELECT * FROM table WHERE 1=1",
    },
    {
      name: "should handle macros escaped multiple times ($$$$)",
      query: "SELECT * FROM table WHERE $$$$__conditionalAll(bar in ($bar), $bar)",
      variables: [{ name: "bar", current: { value: "$__all" } } as any],
      expected: "SELECT * FROM table WHERE $$$__conditionalAll(bar in ($bar), $bar)",
    },
    {
      name: "should handle macros escaped multiple times ($$$)",
      query: "SELECT * FROM table WHERE $$$__conditionalAll(bar in ($bar), $bar)",
      variables: [{ name: "bar", current: { value: "$__all" } } as any],
      expected: "SELECT * FROM table WHERE $$__conditionalAll(bar in ($bar), $bar)",
    },
    {
      name: "should handle escaped macro with no parentheses",
      query: "SELECT * FROM table WHERE $$__conditionalAll",
      variables: [],
      expected: "SELECT * FROM table WHERE $__conditionalAll",
    },
    {
      name: "should handle multiple escaped and unescaped macros in different positions",
      query: "SELECT $$__conditionalAll(a in ($a), $a), $__conditionalAll(b in ($b), $b) FROM table WHERE $$__conditionalAll(c in ($c), $c)",
      variables: [
        { name: "a", current: { value: "$__all" } } as any,
        { name: "b", current: { value: "$__all" } } as any,
        { name: "c", current: { value: "$__all" } } as any,
      ],
      expected: "SELECT $__conditionalAll(a in ($a), $a), 1=1 FROM table WHERE $__conditionalAll(c in ($c), $c)",
    },
  ];

  test.each(cases)("$name", ({ query, variables, expected }) => {
    const actual = applyConditionalAll(query, {
      ...emptyContext,
      templateVars: variables,
    });
    expect(actual).toEqual(expected);
  });

  test("should throw error for unclosed macro with escaping", () => {
    expect(() => {
      applyConditionalAll(
        "SELECT * FROM table WHERE $$__conditionalAll(bar in ($bar), $bar",
        {
          ...emptyContext,
          templateVars: [],
        }
      );
    }).toThrow("failed to parse macro arguments (missing close bracket?)");
  });
});
