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

    expect(params).toStrictEqual(["1", " 'word'", " func()", " $__mac()"]);
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
