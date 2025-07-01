import { getDefaultValue } from "./editorUtils";

describe("query settings getDefaultValue", () => {
  it("should return 0 number", () => {
    expect(getDefaultValue(0, "number")).toBe("0");
  });
  it("should return false boolean", () => {
    expect(getDefaultValue(false, "boolean")).toBe("0");
  });
  it("should return true boolean", () => {
    expect(getDefaultValue(true, "boolean")).toBe("1");
  });
  it("should return empty text value", () => {
    expect(getDefaultValue(undefined, "text")).toBe("");
  });
  it("should return empty textarea value", () => {
    expect(getDefaultValue(undefined, "textarea")).toBe("");
  });
  it("should return empty duration value", () => {
    expect(getDefaultValue(undefined, "duration")).toBe("");
  });
  it("should return 3 number", () => {
    expect(getDefaultValue(3, "number")).toBe("3");
  });
});
