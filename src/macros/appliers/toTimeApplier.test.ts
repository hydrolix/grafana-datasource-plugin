import { emptyContext } from "../macrosService";
import { dateTime, TimeRange } from "@grafana/data";
import { ToTimeApplier } from "./toTimeApplier";

describe("macros toTimeFilter", () => {
  let toTimeApplier: ToTimeApplier;
  beforeEach(() => {
    toTimeApplier = new ToTimeApplier();
  });

  it("should apply macros", async () => {
    let result = await toTimeApplier.applyMacros("SELECT $__toTime()", {
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
    expect(result).toBe("SELECT toDateTime(1666667864)");
  });

  it("should apply without timerange", async () => {
    let t = async () =>
      await toTimeApplier.applyMacros("SELECT $__toTime()", {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      });
    await expect(t()).rejects.toThrow("cannot apply macros without time range");
  });
});
