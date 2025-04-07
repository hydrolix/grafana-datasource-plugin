import { emptyContext } from "../macrosService";
import { TimeIntervalMsApplier } from "./timeIntervalMsApplier";

describe("macros time interval ms", () => {
  let timeIntervalMsApplier: TimeIntervalMsApplier;
  beforeEach(() => {
    timeIntervalMsApplier = new TimeIntervalMsApplier();
  });

  it("should apply macros", async () => {
    let result = await timeIntervalMsApplier.applyMacros(
      "SELECT $__timeInterval_ms(column)",
      {
        ...emptyContext,
        intervalMs: 30000,
      }
    );
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime64(column, 3), INTERVAL 30000 millisecond)"
    );
  });
  it("should apply macros with no interval", async () => {
    let result = await timeIntervalMsApplier.applyMacros(
      "SELECT $__timeInterval_ms(column)",
      {
        ...emptyContext,
      }
    );
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime64(column, 3), INTERVAL 1 millisecond)"
    );
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await timeIntervalMsApplier.applyMacros("SELECT $__timeInterval_ms()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macros $__timeInterval_ms should contain 1 parameter"
    );
  });
});
