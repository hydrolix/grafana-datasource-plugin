import {
  DateTime,
  dateTime,
  durationToMilliseconds,
  parseDuration,
  TimeRange,
} from "@grafana/data";

export const QUERY_DURATION_REGEX = /^$|^0$|^(\d+)([smh])$/;

export const getFirstValidRound = (rounds: string[]): string =>
  rounds.find(
    (round) =>
      round !== undefined && round !== "" && QUERY_DURATION_REGEX.test(round)
  ) || "";

export const roundTimeRange = (
  timeRange: TimeRange,
  round: string
): TimeRange => {
  let duration = durationToMilliseconds(parseDuration(round)) / 1000;
  if (!duration) {
    return timeRange;
  } else {
    return {
      from: roundDateTime(timeRange.from, duration),
      to: roundDateTime(timeRange.to, duration),
      raw: timeRange.raw,
    };
  }
};

const roundDateTime = (time: DateTime, duration: number) =>
  dateTime((time.unix() - (time.unix() % duration)) * 1000);
