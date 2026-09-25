import { rangeUtil, TimeRange } from "@grafana/data";
import { DEFAULT_INTERPOLATION_RESOLUTION } from "../constants";

export const QUERY_DURATION_REGEX = /^$|^0$|^(\d+)([smh])$/;

// A whole number and one unit, matched in full. `rangeUtil.intervalToSeconds`
// alone is too lenient to validate with: it matches only a prefix, `parseInt`s
// the count, and reads unit-less input as seconds, so `24` is 24 seconds, `1e3`
// is 1 second, `1.5h` is 1h, `24hfoo` is 24h, and `500ms` is 0.5. Only s, m
// and h are advertised;
// `d` is accepted so provisioned and existing values keep resolving.
const LOOKBACK_REGEX = /^\d+[smhd]$/;

/**
 * Interval to send with an interpolation request, derived from the range it
 * accompanies so the two always describe the same window.
 *
 * Emitted in milliseconds on purpose. Grafana's interval formatting reaches for
 * `d` / `w` / `M` / `y` at low resolutions (a 5-year range at 20 points yields
 * `"1y"`), and the backend parses this field with Go's `time.ParseDuration`,
 * whose units stop at `h` — an unparseable value fails the whole interpolation
 * request rather than degrading.
 */
export const deriveInterpolationInterval = (
  range: TimeRange,
  resolution?: number
): string => {
  const { intervalMs } = rangeUtil.calculateInterval(
    range,
    resolution && resolution > 0 ? resolution : DEFAULT_INTERPOLATION_RESOLUTION
  );
  return `${intervalMs}ms`;
};

/**
 * Parses an ad-hoc lookback duration (`30m`, `6h`, `24h`, …) to whole seconds.
 * Returns undefined for anything that is not a positive `<count><unit>` string
 * so callers can fall back to the default. The parameter is `unknown` because
 * jsonData can be provisioned from YAML, where `86400` arrives as a number.
 * Shared by the datasource (resolution) and the config editor (validation) so
 * the two agree on what is valid.
 */
export const parseLookbackSeconds = (value?: unknown): number | undefined => {
  if (typeof value !== "string") {
    return undefined;
  }
  const trimmed = value.trim();
  if (!LOOKBACK_REGEX.test(trimmed)) {
    return undefined;
  }
  // Cannot throw: every string the regex admits reaches rangeUtil's unit
  // branch with a known unit.
  const seconds = rangeUtil.intervalToSeconds(trimmed);
  // `\d+` admits any number of digits, so a long count overflows to Infinity
  // or loses precision, and would reach the SQL SETTINGS clause as `Infinity`
  // or in exponent notation.
  return Number.isSafeInteger(seconds) && seconds > 0 ? seconds : undefined;
};
