import { emptyContext, MacrosService } from "./macrosService";

describe("macros service", () => {
  let macrosService: MacrosService;
  beforeEach(() => {
    macrosService = new MacrosService();
  });

  it("should apply macros", async () => {
    macrosService.registerMacros({
      applyMacros: (sql, _) => sql + "macros applied",
    });
    let result = await macrosService.applyMacros("sql ", emptyContext);
    expect(result).toBe("sql macros applied");
  });

  it("should apply both macros", async () => {
    macrosService.registerMacros({
      applyMacros: (sql, _) => sql + "macros1 applied ",
    });
    macrosService.registerMacros({
      applyMacros: (sql, _) => sql + "macros2 applied",
    });
    let result = await macrosService.applyMacros("sql ", emptyContext);
    expect(result).toBe("sql macros1 applied macros2 applied");
  });
});
