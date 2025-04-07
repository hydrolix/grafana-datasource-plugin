import { emptyContext } from "../macrosService";
import { dateTime } from "@grafana/data";
import { DTApplier } from "./dtApplier";

describe("macros dateTimeFilter", () => {
  let dateTimeFilterApplier: DTApplier;
  beforeEach(() => {
    dateTimeFilterApplier = new DTApplier();
  });

  it("should apply macros", async () => {
    let result = await dateTimeFilterApplier.applyMacros(
      "SELECT * FROM table WHERE $__dt(date, time)",
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
});
