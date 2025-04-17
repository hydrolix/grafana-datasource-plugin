import { getFirstValidRound, roundTimeRange } from "./timeRangeUtils";
import { dateTime } from "@grafana/data";

describe("getFirstValidRound", () => {
  test("should return first statement", () => {
    let result = getFirstValidRound(["1s", "1m"]);
    expect(result).toBe("1s");
  });
  test("should return second statement", () => {
    let result = getFirstValidRound(["", "1m"]);
    expect(result).toBe("1m");
  });
  test("should return 0", () => {
    let result = getFirstValidRound(["0", "1m"]);
    expect(result).toBe("0");
  });
  test("should skip invalid", () => {
    let result = getFirstValidRound(["1a", "1m"]);
    expect(result).toBe("1m");
  });
});

const timeRange = {
  from: dateTime("2025-02-12T11:45:26.123Z"),
  to: dateTime("2025-02-13T11:45:26.456Z"),
  raw: {
    from: "",
    to: "",
  },
};
describe("roundTimeRange", () => {
  test("should round to 2m", () => {
    let result = roundTimeRange(timeRange, "2m");

    expect(result.to.unix()).toEqual(dateTime("2025-02-13T11:44:00Z").unix());
    expect(result.from.unix()).toEqual(dateTime("2025-02-12T11:44:00Z").unix());
  });

  test("should round to 1m", () => {
    let result = roundTimeRange(timeRange, "1m");

    expect(result.to.unix()).toEqual(dateTime("2025-02-13T11:45:00Z").unix());
    expect(result.from.unix()).toEqual(dateTime("2025-02-12T11:45:00Z").unix());
  });

  test("should round to 1s", () => {
    let result = roundTimeRange(timeRange, "1s");

    expect(result.to.unix()).toEqual(dateTime("2025-02-13T11:45:26Z").unix());
    expect(result.from.unix()).toEqual(dateTime("2025-02-12T11:45:26Z").unix());
  });

  test("should round to 1h", () => {
    let result = roundTimeRange(timeRange, "1h");

    expect(result.to.unix()).toEqual(dateTime("2025-02-13T11:00:00Z").unix());
    expect(result.from.unix()).toEqual(dateTime("2025-02-12T11:00:00Z").unix());
  });

  test("should not round to invalid", () => {
    let result = roundTimeRange(timeRange, "1a");

    expect(result.to.unix()).toEqual(
      dateTime("2025-02-13T11:45:26.456Z").unix()
    );
    expect(result.from.unix()).toEqual(
      dateTime("2025-02-12T11:45:26.123Z").unix()
    );
  });
  test("should not round to 0", () => {
    let result = roundTimeRange(timeRange, "0");

    expect(result.to.unix()).toEqual(
      dateTime("2025-02-13T11:45:26.456Z").unix()
    );
    expect(result.from.unix()).toEqual(
      dateTime("2025-02-12T11:45:26.123Z").unix()
    );
  });
  test("should not round", () => {
    let result = roundTimeRange(timeRange, "");

    expect(result.to.unix()).toEqual(
      dateTime("2025-02-13T11:45:26.456Z").unix()
    );
    expect(result.from.unix()).toEqual(
      dateTime("2025-02-12T11:45:26.123Z").unix()
    );
  });
});
