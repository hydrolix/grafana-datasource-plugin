import { resolveExposeErrors } from "./defaultConfigs";

const DEFAULTS = {
  enabled: true,
  variableName: "hdx_query_errors",
  maxCount: 5,
  ttl: 300,
};

describe("resolveExposeErrors", () => {
  it("uses the defaults when the block is missing", () => {
    expect(resolveExposeErrors(undefined)).toEqual(DEFAULTS);
    expect(resolveExposeErrors({})).toEqual(DEFAULTS);
  });

  it("keeps stored keys and fills the rest", () => {
    expect(resolveExposeErrors({ enabled: false })).toEqual({
      ...DEFAULTS,
      enabled: false,
    });
  });

  it("treats a block without enabled as enabled", () => {
    expect(resolveExposeErrors({ variableName: "errs" })).toEqual({
      ...DEFAULTS,
      variableName: "errs",
    });
  });

  it("keeps explicit zero and empty values", () => {
    expect(resolveExposeErrors({ maxCount: 0, ttl: 0, variableName: "" })).toEqual({
      enabled: true,
      variableName: "",
      maxCount: 0,
      ttl: 0,
    });
  });

  it("does not let an explicit undefined override a default", () => {
    expect(resolveExposeErrors({ ttl: undefined })).toEqual(DEFAULTS);
  });

  it("treats a stored null as missing", () => {
    expect(resolveExposeErrors(null)).toEqual(DEFAULTS);
    expect(
      resolveExposeErrors(
        JSON.parse(
          '{"enabled":null,"variableName":null,"maxCount":null,"ttl":null}'
        )
      )
    ).toEqual(DEFAULTS);
    expect(
      resolveExposeErrors(JSON.parse('{"enabled":false,"ttl":null}'))
    ).toEqual({ ...DEFAULTS, enabled: false });
  });
});
