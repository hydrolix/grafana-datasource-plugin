import { TypedVariableModel } from "@grafana/data";
import { ConditionalAllApplier } from "./conditionalAllApplier";
import { emptyContext } from "../macrosService";

describe("ConditionalAllApplier", () => {
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
    const applier = new ConditionalAllApplier();
    const actual = await applier.applyMacros(query, {
      ...emptyContext,
      templateVars: variables,
    });
    expect(actual).toEqual(expected);
  });
});
