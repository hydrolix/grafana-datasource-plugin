import { emptyContext } from "../macrosService";
import { dateTime, TimeRange } from "@grafana/data";
import { TimeFilterApplier } from "./timeFilterApplier";

describe("macros timeFilter", () => {
  let timeFilterApplier: TimeFilterApplier;
  beforeEach(() => {
    timeFilterApplier = new TimeFilterApplier();
  });

  it("should apply macros", async () => {
    let result = await timeFilterApplier.applyMacros(
      "SELECT * FROM table WHERE $__timeFilter(date)",
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
      "SELECT * FROM table WHERE date >= toDateTime(1666369533) AND date <= toDateTime(1666715133)"
    );
  });

  it("should apply without timerange", async () => {
    let result = await timeFilterApplier.applyMacros(
      "SELECT * FROM table WHERE $__timeFilter(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await timeFilterApplier.applyMacros(
        "SELECT * FROM table WHERE $__timeFilter()",
        {
          ...emptyContext,
        }
      );
    await expect(t()).rejects.toThrow(
      "Macros $__timeFilter should contain 1 parameter"
    );
  });
});
