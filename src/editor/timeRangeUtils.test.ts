import { dateTime, makeTimeRange } from "@grafana/data";
import { deriveInterpolationInterval } from "./timeRangeUtils";
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
