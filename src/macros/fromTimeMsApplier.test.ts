import { emptyContext } from "./macrosService";
import { dateTime, TimeRange } from "@grafana/data";
import { FromTimeMsApplier } from "./fromTimeMsApplier";

describe("macros fromTimeMsFilter", () => {
  let fromTimeMsApplier: FromTimeMsApplier;
  beforeEach(() => {
    fromTimeMsApplier = new FromTimeMsApplier();
  });

  it("should apply macros", async () => {
    let result = await fromTimeMsApplier.applyMacros(
      "SELECT $__fromTime_ms()",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21T19:23:44"),
          to: dateTime("2022-10-25T03:17:44"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe("SELECT fromUnixTimestamp64Milli(1666380224000)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await fromTimeMsApplier.applyMacros("SELECT $__fromTime_ms()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macros without time range");
  });
});
