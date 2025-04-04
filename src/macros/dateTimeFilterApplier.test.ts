import { emptyContext } from "./macrosService";
import { dateTime, TimeRange } from "@grafana/data";
import { DateTimeFilterApplier } from "./dateTimeFilterApplier";

describe("macros dateTimeFilter", () => {
  let dateTimeFilterApplier: DateTimeFilterApplier;
  beforeEach(() => {
    dateTimeFilterApplier = new DateTimeFilterApplier();
  });

  it("should apply macros", async () => {
    let result = await dateTimeFilterApplier.applyMacros(
      "SELECT * FROM table WHERE $__dateTimeFilter(date, time)",
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
    expect(result).toBe(
      "SELECT * FROM table WHERE date >= toDate(2022-10-21) AND date <= toDate(2022-10-25) AND  time >= toDateTime(1666380224) AND  time <= toDateTime(1666667864)"
    );
  });

  it("should apply without timerange", async () => {
    let result = await dateTimeFilterApplier.applyMacros(
      "SELECT * FROM table WHERE $__dateTimeFilter(date, time)",
      {
        ...emptyContext,
        timeRange: null as unknown as TimeRange,
      }
    );
    expect(result).toBe("SELECT * FROM table WHERE 1=1");
  });

  it("should fail on macros with no params", async () => {
    let t = async () =>
      await dateTimeFilterApplier.applyMacros(
        "SELECT * FROM table WHERE $__dateTimeFilter(date)",
        {
          ...emptyContext,
        }
      );
    await expect(t()).rejects.toThrow(
      "Macros $__dateTimeFilter should contain 2 parameters"
    );
  });
});
