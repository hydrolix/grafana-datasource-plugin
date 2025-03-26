export const QUERY_DURATION_REGEX = /^$|^0$|^(\d+)(ms|s|m|h|d|w|M|y)$/;

export const getFirstValidRound = (rounds: string[]): string =>
  rounds.find(
    (round) =>
      round !== undefined && round !== "" && QUERY_DURATION_REGEX.test(round)
  ) || "";
