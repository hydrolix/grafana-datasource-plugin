import { registerMacrosService } from "./registerMacrosService";
import { MD_PROVIDER, TABLE_FN } from "./appliers/adHocFilterApplier.test";
import { emptyContext } from "./macrosService";
import { dateTime } from "@grafana/data";

describe("register macros service", () => {
  const macrosService = registerMacrosService(MD_PROVIDER, TABLE_FN);
  const cases = [
    {
      origin: "SELECT * FROM foo WHERE $__timeFilter(cast(col as timestamp))",
      interpolated:
        "SELECT * FROM foo WHERE cast(col as timestamp) >= toDateTime(1739360726.123) AND cast(col as timestamp) <= toDateTime(1739447126.456)",
      name: "timeFilter",
    },
    {
      origin: "SELECT * FROM foo WHERE $__timeFilter( cast(col as timestamp) )",
      interpolated:
        "SELECT * FROM foo WHERE  cast(col as timestamp)  >= toDateTime(1739360726.123) AND  cast(col as timestamp)  <= toDateTime(1739447126.456)",
      name: "timeFilter with whitespaces",
    },
    {
      origin:
        "SELECT * FROM foo WHERE $__timeFilter_ms(cast(col as timestamp))",
      interpolated:
        "SELECT * FROM foo WHERE cast(col as timestamp) >= fromUnixTimestamp64Milli(1739360726123) AND cast(col as timestamp) <= fromUnixTimestamp64Milli(1739447126456)",
      name: "timeFilter_ms",
    },
    {
      origin:
        "SELECT * FROM foo WHERE $__timeFilter_ms( cast(col as timestamp) )",
      interpolated:
        "SELECT * FROM foo WHERE  cast(col as timestamp)  >= fromUnixTimestamp64Milli(1739360726123) AND  cast(col as timestamp)  <= fromUnixTimestamp64Milli(1739447126456)",
      name: "timeFilter_ms with whitespaces",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime and col <= $__toTime ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= toDateTime(1739360726.123) and col <= toDateTime(1739447126.456) ) limit 100",
      name: "fromTime and toTime",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime ) and ( col <= $__toTime ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= toDateTime(1739360726.123) ) and ( col <= toDateTime(1739447126.456) ) limit 100",
      name: "fromTime and toTime condition #2",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime_ms and col <= $__toTime_ms ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= fromUnixTimestamp64Milli(1739360726123) and col <= fromUnixTimestamp64Milli(1739447126456) ) limit 100",
      name: "fromTime_ms and toTime_ms",
    },
    {
      origin:
        "SELECT * FROM foo WHERE ( col >= $__fromTime_ms ) and ( col <= $__toTime_ms ) limit 100",
      interpolated:
        "SELECT * FROM foo WHERE ( col >= fromUnixTimestamp64Milli(1739360726123) ) and ( col <= fromUnixTimestamp64Milli(1739447126456) ) limit 100",
      name: "fromTime_ms and toTime_ms condition #2",
    },
  ];

  it.each(cases)("$name", async ({ origin, interpolated }) => {
    const actual = await macrosService.applyMacros(origin, {
      ...emptyContext,
      timeRange: {
        from: dateTime("2025-02-12T11:45:26.123Z"),
        to: dateTime("2025-02-13T11:45:26.456Z"),
        raw: {
          from: "",
          to: "",
        },
      },
    });
    expect(actual).toEqual(interpolated);
  });
});
