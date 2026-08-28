import { rangeUtil, TimeRange } from "@grafana/data";
import { DEFAULT_INTERPOLATION_RESOLUTION } from "../constants";

export const QUERY_DURATION_REGEX = /^$|^0$|^(\d+)([smh])$/;

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
