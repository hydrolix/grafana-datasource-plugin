import React from "react";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { toValidationState, ValidationBar } from "./ValidationBar";

describe("ValidationBar", () => {
  it("renders an empty bar when idle", () => {
    render(<ValidationBar state={{ status: "idle" }} />);
    expect(screen.getByTestId("query-validation-bar")).toBeEmptyDOMElement();
  });

  it("shows a spinner while validating", () => {
    render(<ValidationBar state={{ status: "validating" }} />);
    expect(screen.getByText(/Validating query/i)).toBeInTheDocument();
  });

  it("shows the success state", () => {
    render(<ValidationBar state={{ status: "valid" }} />);
    expect(screen.getByText("Query is valid")).toBeInTheDocument();
  });

  it("shows the error message", () => {
    render(
      <ValidationBar state={{ status: "error", message: "Missing columns" }} />
    );
    expect(screen.getByText("Missing columns")).toBeInTheDocument();
    expect(screen.queryByText("Query is valid")).not.toBeInTheDocument();
  });

  it("shows the warning message", () => {
    render(
      <ValidationBar
        state={{ status: "warning", message: "Primary key not filtered" }}
      />
    );
    expect(screen.getByText("Primary key not filtered")).toBeInTheDocument();
  });
});

describe("toValidationState", () => {
  it.each([
    [{}, { status: "valid" }],
    [{ skipped: true }, { status: "idle" }],
    [{ error: "bad" }, { status: "error", message: "bad" }],
    [{ warning: "careful" }, { status: "warning", message: "careful" }],
    [
      { error: "bad", warning: "careful" },
      { status: "error", message: "bad" },
    ],
  ])("maps %j to %j", (result, expected) => {
    expect(toValidationState(result)).toEqual(expected);
  });
});
