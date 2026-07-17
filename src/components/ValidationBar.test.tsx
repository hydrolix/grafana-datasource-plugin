/**
 * ValidationBar – covered cases
 *
 * Early returns (empty container):
 *   - interpolationResult is null
 *   - interpolationResult.interpolatedSql is undefined
 *
 * Active render states (interpolatedSql is set):
 *   - Validating: query !== interpolationResult.originalSql
 *       → spinner + "Validating query..."
 *   - Success:    !validating && !hasError && !hasWarning
 *       → check icon + "No errors found"
 *   - Error:      !validating && hasError
 *       → exclamation-circle icon + interpolationResult.error
 *   - Warning:    !validating && !hasError && hasWarning
 *       → exclamation-triangle icon + interpolationResult.warning
 *   - Precedence: when both hasError and hasWarning are true, the error
 *                 branch wins and the warning text is not rendered.
 *
 * Not covered here (would belong elsewhere):
 *   - Theming / emotion class output (visual; better as a snapshot or e2e check).
 *   - Monaco prop wiring – ValidationBar accepts `monaco` but does not use it
 *     directly; it is passed through for future hooks.
 */
import React from "react";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { ValidationBar } from "./ValidationBar";
import { InterpolationResult } from "../types";

function makeResult(
  overrides: Partial<InterpolationResult> = {}
): InterpolationResult {
  return {
    originalSql: "SELECT 1",
    interpolationId: "id-1",
    interpolatedSql: "SELECT 1",
    finalSql: "SELECT 1",
    hasError: false,
    hasWarning: false,
    ...overrides,
  };
}

describe("ValidationBar", () => {
  it("renders nothing when there is no interpolation result", () => {
    const { container } = render(
      <ValidationBar monaco={null} query="SELECT 1" interpolationResult={null} />
    );
    expect(container.firstChild).toBeEmptyDOMElement();
  });

  it("renders nothing when interpolatedSql is absent", () => {
    const { container } = render(
      <ValidationBar
        monaco={null}
        query="SELECT 1"
        interpolationResult={makeResult({ interpolatedSql: undefined })}
      />
    );
    expect(container.firstChild).toBeEmptyDOMElement();
  });

  it("shows a validating indicator when the query has drifted from the interpolation result", () => {
    render(
      <ValidationBar
        monaco={null}
        query="SELECT 2"
        interpolationResult={makeResult({ originalSql: "SELECT 1" })}
      />
    );
    expect(screen.getByText(/Validating query/i)).toBeInTheDocument();
  });

  it("shows the success state when validation completes with no errors or warnings", () => {
    render(
      <ValidationBar
        monaco={null}
        query="SELECT 1"
        interpolationResult={makeResult()}
      />
    );
    expect(screen.getByText(/No errors found/i)).toBeInTheDocument();
  });

  it("shows the error message when hasError is true", () => {
    render(
      <ValidationBar
        monaco={null}
        query="SELECT 1"
        interpolationResult={makeResult({
          hasError: true,
          error: "Unexpected token",
        })}
      />
    );
    expect(screen.getByText("Unexpected token")).toBeInTheDocument();
    expect(screen.queryByText(/No errors found/i)).not.toBeInTheDocument();
  });

  it("shows the warning message when hasWarning is true and no error is present", () => {
    render(
      <ValidationBar
        monaco={null}
        query="SELECT 1"
        interpolationResult={makeResult({
          hasWarning: true,
          warning: "Missing time filter",
        })}
      />
    );
    expect(screen.getByText("Missing time filter")).toBeInTheDocument();
  });

  it("prefers the error over the warning when both are set", () => {
    render(
      <ValidationBar
        monaco={null}
        query="SELECT 1"
        interpolationResult={makeResult({
          hasError: true,
          error: "Syntax error",
          hasWarning: true,
          warning: "Missing time filter",
        })}
      />
    );
    expect(screen.getByText("Syntax error")).toBeInTheDocument();
    expect(screen.queryByText("Missing time filter")).not.toBeInTheDocument();
  });
});
