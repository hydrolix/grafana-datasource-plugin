import { getFirstValidRound } from "./timeRangeUtils";

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
