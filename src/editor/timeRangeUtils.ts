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

/**
 * Parses an ad-hoc lookback duration (`30m`, `24h`, `7d`, …) to seconds.
 * Returns undefined for empty, unparseable, or non-positive input so callers
 * can fall back to the default. Shared by the datasource (resolution) and the
 * config editor (validation) so the two agree on what is valid.
 */
export const parseLookbackSeconds = (value?: string): number | undefined => {
  const trimmed = value?.trim();
  if (!trimmed) {
    return undefined;
  }
  try {
    const seconds = rangeUtil.intervalToSeconds(trimmed);
    return Number.isFinite(seconds) && seconds > 0 ? seconds : undefined;
  } catch {
    return undefined;
  }
};
