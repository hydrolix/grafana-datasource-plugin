import { emptyContext } from "./macrosService";
import { IntervalSApplier } from "./intervalApplier";

describe("macros interval", () => {
  let intervalSApplier: IntervalSApplier;
  beforeEach(() => {
    intervalSApplier = new IntervalSApplier();
  });

  it("should apply macros", async () => {
    let result = await intervalSApplier.applyMacros("SELECT $__interval_s()", {
      ...emptyContext,
      intervalMs: 30000,
    });
    expect(result).toBe("SELECT 30");
  });

  it("should apply with 0", async () => {
    let result = await intervalSApplier.applyMacros("SELECT $__interval_s()", {
      ...emptyContext,
      intervalMs: 0,
    });
    expect(result).toBe("SELECT 1");
  });

  it("should apply with null", async () => {
    let result = await intervalSApplier.applyMacros("SELECT $__interval_s()", {
      ...emptyContext,
    });
    expect(result).toBe("SELECT 1");
  });
});
