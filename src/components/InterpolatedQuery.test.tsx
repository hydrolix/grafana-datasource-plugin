/**
 * InterpolatedQuery – covered cases
 *
 * Visibility:
 *   - showSQL=false → renders nothing.
 *   - showSQL=true  → renders the "Interpolated Query" heading.
 *
 * Body branch (chosen by `(!sql || showErrors) && error`):
 *   - SQL branch (no error, or error suppressed by showErrors=false):
 *       → SQL text is shown and the copy IconButton is present
 *         (queried by aria-label "copy to clipboard" — Grafana's IconButton
 *         derives the aria-label from the `tooltip` prop).
 *   - Error branch (error present AND showErrors=true):
 *       → error text is shown (in red), no copy IconButton.
 *   - Error branch with showErrors=false but sql empty:
 *       → falls into the error <pre> but renders an empty body
 *         (showErrors? error : "").
 *
 * Icon swap:
 *   - dirty=true  → spinner icon, tooltip/aria-label "processing".
 *   - dirty=false → copy icon, tooltip/aria-label "copy to clipboard".
 *
 * Clipboard:
 *   - Clicking the copy IconButton writes sql to navigator.clipboard.
 */
import React from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { InterpolatedQuery } from "./InterpolatedQuery";

describe("InterpolatedQuery", () => {
  it("renders nothing when showSQL is false", () => {
    const { container } = render(
      <InterpolatedQuery
        sql="SELECT 1"
        error=""
        showSQL={false}
        dirty={false}
        showErrors={false}
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the heading and sql when showSQL is true and no error is shown", () => {
    render(
      <InterpolatedQuery
        sql="SELECT 1"
        error=""
        showSQL={true}
        dirty={false}
        showErrors={false}
      />
    );
    expect(screen.getByText("Interpolated Query")).toBeInTheDocument();
    expect(screen.getByText("SELECT 1")).toBeInTheDocument();
    expect(screen.getByLabelText("copy to clipboard")).toBeInTheDocument();
  });

  it("renders the error message and no copy button when error and showErrors are set", () => {
    render(
      <InterpolatedQuery
        sql="SELECT 1"
        error="bad token"
        showSQL={true}
        dirty={false}
        showErrors={true}
      />
    );
    expect(screen.getByText("bad token")).toBeInTheDocument();
    expect(screen.queryByLabelText("copy to clipboard")).not.toBeInTheDocument();
  });

  it("hides the error text when showErrors is false but the sql body is empty", () => {
    render(
      <InterpolatedQuery
        sql=""
        error="bad token"
        showSQL={true}
        dirty={false}
        showErrors={false}
      />
    );
    expect(screen.queryByText("bad token")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("copy to clipboard")).not.toBeInTheDocument();
  });

  it("uses the spinner icon with 'processing' tooltip when dirty", () => {
    render(
      <InterpolatedQuery
        sql="SELECT 1"
        error=""
        showSQL={true}
        dirty={true}
        showErrors={false}
      />
    );
    expect(screen.getByLabelText("processing")).toBeInTheDocument();
    expect(screen.queryByLabelText("copy to clipboard")).not.toBeInTheDocument();
  });

  it("copies the sql to the clipboard when the copy button is clicked", async () => {
    const writeText = jest.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });

    render(
      <InterpolatedQuery
        sql="SELECT 42"
        error=""
        showSQL={true}
        dirty={false}
        showErrors={false}
      />
    );

    await userEvent.click(screen.getByLabelText("copy to clipboard"));
    expect(writeText).toHaveBeenCalledWith("SELECT 42");
  });
});