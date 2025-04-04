import { emptyContext } from "./macrosService";
import { dateTime, TimeRange } from "@grafana/data";
import { ToTimeMsApplier } from "./toTimeMsApplier";

describe("macros toTimeMsFilter", () => {
  let toTimeMsApplier: ToTimeMsApplier;
  beforeEach(() => {
    toTimeMsApplier = new ToTimeMsApplier();
  });

  it("should apply macros", async () => {
    let result = await toTimeMsApplier.applyMacros("SELECT $__toTime_ms()", {
      ...emptyContext,
      timeRange: {
        from: dateTime("2022-10-21T19:23:44"),
        to: dateTime("2022-10-25T03:17:44"),
        raw: {
          from: "",
          to: "",
        },
      },
    });
    expect(result).toBe("SELECT fromUnixTimestamp64Milli(1666667864000)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await toTimeMsApplier.applyMacros("SELECT $__toTime_ms()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macros without time range");
  });
});
