import { dateTime, makeTimeRange } from "@grafana/data";
import {
  deriveInterpolationInterval,
  parseLookbackSeconds,
} from "./timeRangeUtils";
import { DEFAULT_INTERPOLATION_RESOLUTION } from "../constants";

const TO_MS = 1_700_000_000_000;
const SECOND = 1000;
const HOUR = 60 * 60 * SECOND;
const DAY = 24 * HOUR;

const rangeSpanning = (spanMs: number) =>
  makeTimeRange(dateTime(TO_MS - spanMs), dateTime(TO_MS));

// Go's time.ParseDuration accepts ns, us/µs, ms, s, m, h — nothing coarser.
// Grafana's own interval formatting emits d / w / M / y at low resolutions,
// which is why the derived value is milliseconds rather than that string.
const GO_PARSEABLE = /^\d+ms$/;

describe("deriveInterpolationInterval", () => {
  const spans: Array<[string, number]> = [
    ["6 hours", 6 * HOUR],
    ["19 days", 19 * DAY],
    ["90 days", 90 * DAY],
    ["1 year", 365 * DAY],
    ["5 years", 5 * 365 * DAY],
  ];
  const resolutions = [1000, 100, 20];

  it.each(spans)(
    "emits a Go-parseable millisecond duration for a %s range",
    (_label, span) => {
      for (const resolution of resolutions) {
        const interval = deriveInterpolationInterval(
          rangeSpanning(span),
          resolution
        );
        expect(interval).toMatch(GO_PARSEABLE);
      }
    }
  );

  it("never emits day, week, month or year units", () => {
    for (const [, span] of spans) {
      for (const resolution of resolutions) {
        const interval = deriveInterpolationInterval(
          rangeSpanning(span),
          resolution
        );
        expect(interval).not.toMatch(/[dwMy]/);
      }
    }
  });

  it("widens the interval as the range widens at a fixed resolution", () => {
    const narrow = deriveInterpolationInterval(rangeSpanning(6 * HOUR), 100);
    const wide = deriveInterpolationInterval(rangeSpanning(90 * DAY), 100);
    expect(parseInt(wide, 10)).toBeGreaterThan(parseInt(narrow, 10));
  });

  it("widens the interval as resolution drops at a fixed range", () => {
    const fine = deriveInterpolationInterval(rangeSpanning(90 * DAY), 1000);
    const coarse = deriveInterpolationInterval(rangeSpanning(90 * DAY), 20);
    expect(parseInt(coarse, 10)).toBeGreaterThan(parseInt(fine, 10));
  });

  it("falls back to the default resolution when none is supplied", () => {
    const range = rangeSpanning(90 * DAY);
    expect(deriveInterpolationInterval(range)).toBe(
      deriveInterpolationInterval(range, DEFAULT_INTERPOLATION_RESOLUTION)
    );
  });

  it.each([0, -1])(
    "falls back to the default resolution for a non-positive value (%s)",
    (resolution) => {
      const range = rangeSpanning(6 * HOUR);
      expect(deriveInterpolationInterval(range, resolution)).toBe(
        deriveInterpolationInterval(range, DEFAULT_INTERPOLATION_RESOLUTION)
      );
    }
  );

  // The case that broke the preview before this change: Grafana's formatting
  // returns "1d" here, which the backend rejects outright.
  it("expresses a 90-day range at low resolution in milliseconds, not days", () => {
    expect(deriveInterpolationInterval(rangeSpanning(90 * DAY), 20)).toBe(
      `${DAY}ms`
    );
  });
});

describe("parseLookbackSeconds", () => {
  it.each([
    ["24h", 86400],
    ["30m", 1800],
    ["600s", 600],
    [" 6h ", 21600],
    // Not advertised, but accepted so provisioned and existing values resolve.
    ["7d", 604800],
  ])("parses %p to %p seconds", (value, seconds) => {
    expect(parseLookbackSeconds(value)).toBe(seconds);
  });

  it.each<[string, unknown]>([
    ["empty", ""],
    ["whitespace", "   "],
    ["undefined", undefined],
    ["garbage", "abc"],
    ["zero", "0"],
    ["zero with a unit", "0h"],
    ["negative", "-5m"],
    ["uppercase unit", "24H"],
    // rangeUtil alone would misread these rather than reject them.
    ["bare number (24 seconds)", "24"],
    ["exponent", "1e3"],
    ["trailing junk", "24hfoo"],
    ["fractional count", "1.5h"],
    ["fractional count under one", "0.5h"],
    ["compound duration", "1h30m"],
    ["milliseconds", "500ms"],
    // Overflow: `\d+` admits any length, so these must not reach the SQL.
    ["a count that overflows to Infinity", "1" + "0".repeat(320) + "s"],
    ["a count beyond safe integers", "99999999999999999999d"],
    // Units rangeUtil knows but the lookback does not accept.
    ["weeks", "1w"],
    ["months", "1M"],
    ["years", "1y"],
    // Provisioned YAML can deliver non-strings.
    ["number", 86400],
    ["null", null],
  ])("rejects %s (%p)", (_label, value) => {
    expect(parseLookbackSeconds(value)).toBeUndefined();
  });
});
