import { Context, emptyContext, MacrosService } from "./macrosService";
import { MacrosApplier } from "./macrosApplier";

describe("macros service", () => {
  let macrosService: MacrosService;
  beforeEach(() => {
    macrosService = new MacrosService();
  });
  const getMockApplier = (
    apply: (sql: string, context: Context) => Promise<string>
  ) =>
    ({
      applyMacros(sql: string, context: Context): Promise<string> {
        return apply(sql, context);
      },
      macroName: function (): string {
        return "macros";
      },
      applyMacro: function (sql: string, context: Context): Promise<string> {
        throw new Error("Function not implemented.");
      },
      parseMacroArgs: function (query: string, argsIndex: number): string[] {
        throw new Error("Function not implemented.");
      },
    } as MacrosApplier);

  it("should apply macros", async () => {
    macrosService.registerMacros(
      getMockApplier((sql, _) => Promise.resolve(sql + "macros applied"))
    );
    let result = await macrosService.applyMacros("sql ", emptyContext);
    expect(result).toBe("sql macros applied");
  });

  it("should apply both macros", async () => {
    macrosService.registerMacros(
      getMockApplier((sql, _) => Promise.resolve(sql + "macros1 applied "))
    );
    macrosService.registerMacros(
      getMockApplier((sql, _) => Promise.resolve(sql + "macros2 applied"))
    );
    let result = await macrosService.applyMacros("sql ", emptyContext);
    expect(result).toBe("sql macros1 applied macros2 applied");
  });
});
