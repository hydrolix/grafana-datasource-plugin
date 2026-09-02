import {
  matchSolutionTemplate,
  renderSolutionTemplate,
} from "./errorSolution";

describe("matchSolutionTemplate", () => {
  it("classifies the time-range guard rejection", () => {
    const match = matchSolutionTemplate(
      "<HdxStorageError hdx_query_timerange_required is set to true. Your query needs a time range filter in a WHERE clause>"
    );

    expect(match?.name).toBe("TIMERANGE_REQUIRED");
    expect(match?.groups).toEqual({});
    expect(match?.solution).toContain("must include a time range filter");
  });

  it("extracts named groups and renders them into the solution", () => {
    const match = matchSolutionTemplate(
      "Syntax error: failed at position 21 ('FRM') (line 2, col 3): FRM logs. Expected one of: FROM, WHERE"
    );

    expect(match?.name).toBe("SYNTAX_ERROR");
    expect(match?.groups).toMatchObject({
      position: "21",
      token: "FRM",
      line: "2",
      column: "3",
    });
    expect(match?.solution).toContain("around line 2, column 3");
    expect(match?.solution).toContain("Check the token 'FRM'");
  });

  it("returns undefined for an unrecognized message", () => {
    expect(matchSolutionTemplate("something entirely novel")).toBeUndefined();
  });
});

describe("renderSolutionTemplate", () => {
  it("substitutes captured groups", () => {
    expect(renderSolutionTemplate("limit is {max} seconds", { max: "30" })).toBe(
      "limit is 30 seconds"
    );
  });

  it("leaves placeholders without a captured value verbatim", () => {
    expect(renderSolutionTemplate("use DESCRIBE {table_name}", {})).toBe(
      "use DESCRIBE {table_name}"
    );
  });

  it("keeps {{...}} escapes literal", () => {
    // Templates escape literals as {{name}}; groups never carry those keys,
    // so the inner braces must survive untouched.
    expect(
      renderSolutionTemplate("Example: `WHERE t >= toDateTime('{{start_time}}')`", {})
    ).toBe("Example: `WHERE t >= toDateTime('{{start_time}}')`");
  });
});
