import { emptyContext } from "../macrosService";
import { dateTime, TimeRange } from "@grafana/data";
import { FromTimeApplier } from "./fromTimeApplier";

describe("macros fromTimeFilter", () => {
  let fromTimeApplier: FromTimeApplier;
  beforeEach(() => {
    fromTimeApplier = new FromTimeApplier();
  });

  it("should apply macros", async () => {
    let result = await fromTimeApplier.applyMacros("SELECT $__fromTime()", {
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
    expect(result).toBe("SELECT toDateTime(1666380224)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await fromTimeApplier.applyMacros("SELECT $__fromTime()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macros without time range");
  });
});
