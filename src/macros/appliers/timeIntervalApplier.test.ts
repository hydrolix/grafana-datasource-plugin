import { emptyContext } from "../macrosService";
import { TimeIntervalApplier } from "./timeIntervalApplier";

describe("macros time interval", () => {
  let timeIntervalApplier: TimeIntervalApplier;
  beforeEach(() => {
    timeIntervalApplier = new TimeIntervalApplier();
  });

  it("should apply macros", async () => {
    let result = await timeIntervalApplier.applyMacros(
      "SELECT $__timeInterval(column)",
      {
        ...emptyContext,
        intervalMs: 30000,
      }
    );
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime(column), INTERVAL 30 second)"
    );
  });

  it("should apply macros with no interval", async () => {
    let result = await timeIntervalApplier.applyMacros(
      "SELECT $__timeInterval(column)",
      {
        ...emptyContext,
      }
    );
    expect(result).toBe(
      "SELECT toStartOfInterval(toDateTime(column), INTERVAL 1 second)"
    );
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await timeIntervalApplier.applyMacros("SELECT $__timeInterval()", {
        ...emptyContext,
      });
    await expect(t()).rejects.toThrow(
      "Macros $__timeInterval should contain 1 parameter"
    );
  });
});
