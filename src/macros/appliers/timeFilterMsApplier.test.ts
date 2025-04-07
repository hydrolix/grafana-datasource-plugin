import { emptyContext } from "../macrosService";
import { dateTime, TimeRange } from "@grafana/data";
import { TimeFilterMsApplier } from "./timeFilterMsApplier";

describe("macros timeFilter_ms", () => {
  let timeFilterMsApplier: TimeFilterMsApplier;
  beforeEach(() => {
    timeFilterMsApplier = new TimeFilterMsApplier();
  });

  it("should apply macros", async () => {
    let result = await timeFilterMsApplier.applyMacros(
      "SELECT * FROM table WHERE $__timeFilter_ms(date)",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21T16:25:33"),
          to: dateTime("2022-10-25T16:25:33"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= fromUnixTimestamp64Milli(1666369533000) AND date <= fromUnixTimestamp64Milli(1666715133000)"
    );
  });

  it("should apply without timerange", async () => {
    let result = await timeFilterMsApplier.applyMacros(
      "SELECT * FROM table WHERE $__timeFilter_ms(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await timeFilterMsApplier.applyMacros(
        "SELECT * FROM table WHERE $__timeFilter_ms()",
        {
          ...emptyContext,
        }
      );
    await expect(t()).rejects.toThrow(
      "Macros $__timeFilter_ms should contain 1 parameter"
    );
  });
});
