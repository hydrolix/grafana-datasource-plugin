import { emptyContext } from "./macrosService";
import { DateFilterApplier } from "./dateFilterApplier";
import { dateTime, TimeRange } from "@grafana/data";

describe("macros dateFilter", () => {
  let dateFilterApplier: DateFilterApplier;
  beforeEach(() => {
    dateFilterApplier = new DateFilterApplier();
  });

  it("should apply macros", async () => {
    let result = await dateFilterApplier.applyMacros(
      "SELECT * FROM table WHERE $__dateFilter(date)",
      {
        ...emptyContext,
        timeRange: {
          from: dateTime("2022-10-21"),
          to: dateTime("2022-10-25"),
          raw: {
            from: "",
            to: "",
          },
        },
      }
    );
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= toDate(2022-10-21) AND date <= toDate(2022-10-25)"
    );
  });

  it("should apply without timerange", async () => {
    let result = await dateFilterApplier.applyMacros(
      "SELECT * FROM table WHERE $__dateFilter(date)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await dateFilterApplier.applyMacros(
        "SELECT * FROM table WHERE $__dateFilter()",
        {
          ...emptyContext,
        }
      );
    await expect(t()).rejects.toThrow(
      "Macros $__dateFilter should contain 1 parameter"
    );
  });
});
