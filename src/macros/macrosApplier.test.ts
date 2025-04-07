import { MacrosApplier } from "./macrosApplier";
import { Context, emptyContext } from "./macrosService";

class TestApplier extends MacrosApplier {
  async applyMacro(sql: string, context: Context): Promise<string> {
    return sql.replace(`$__test()`, "");
  }
  macroName(): string {
    return "$__test";
  }
}
describe("macros applier", () => {
  let testApplier = new TestApplier();
  it("apply with one macro", async () => {
    let result = await testApplier.applyMacros("query with $__test()", {
      ...emptyContext,
    });
    expect(result).toBe("query with ");
  });
  it("apply with multiple macro", async () => {
    let result = await testApplier.applyMacros(
      "query with $__test() $__test() $__test()",
      {
        ...emptyContext,
      }
    );
    expect(result).toBe("query with   ");
  });
  it("apply with no macro", async () => {
    let result = await testApplier.applyMacros("query with", {
      ...emptyContext,
    });
    expect(result).toBe("query with");
  });

  it("apply without query", async () => {
    let result = await testApplier.applyMacros("", {
      ...emptyContext,
    });
    expect(result).toBe("");
  });
});
describe("macros parse params", () => {
  it("parse multiple params", () => {
    let testApplier = new TestApplier();
    let rawQuery =
      "select 1 from table where $__test(1, 'word', func(), $__mac())";

    const params = testApplier.parseMacroArgs(rawQuery);

    expect(params).toStrictEqual(["1", " 'word'", " func()", " $__mac()"]);
  });
});
