import { replace } from "./syntheticVariables";

describe("Synthetic Variables", () => {
  test("should replace synthetic variable", () => {
    const input = "some string with variables ${__hydrolix.var}";
    const result = replace(input, {
      var: () => "syntheticVar",
    });
    expect(result).toBe("some string with variables syntheticVar");
  });
  test("should replace synthetic variables", () => {
    const input =
      "some string with variables ${__hydrolix.var} and ${__hydrolix.var2}";
    const result = replace(input, {
      var: () => "syntheticVar",
      var2: () => "anotherSyntheticVar",
    });
    expect(result).toBe(
      "some string with variables syntheticVar and anotherSyntheticVar"
    );
  });
  test("should replace recurrent synthetic variables", () => {
    const input =
      "some string with variables ${__hydrolix.var} and ${__hydrolix.var}";
    const result = replace(input, {
      var: () => "syntheticVar",
    });
    expect(result).toBe(
      "some string with variables syntheticVar and syntheticVar"
    );
  });
  // test("should remove not-replaced variables", () => {
  //   const input =
  //     "dashboard_id=${__dashboard.uid}, dashboard_name=${__dashboard.name}, raw_query=${__hydrolix.raw_query}, user_email=${__user.email}, to_time=$__to, from_time=${__from}, query_source=${__hydrolix.query_source}, url_params=${__url.params}";
  //   const result = replace(input, {});
  //   expect(result).toBe(
  //     "dashboard_id=, dashboard_name=, raw_query=, user_email=, to_time=, from_time=, query_source=, url_params="
  //   );
  // });
});
