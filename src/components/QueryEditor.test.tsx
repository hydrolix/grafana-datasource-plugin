/**
 * QueryEditor – covered cases
 *
 * Scope of mocks (kept aggressive so the unit test stays focused on
 * QueryEditor's own logic):
 *   - @grafana/plugin-ui SQLEditor → a stub that renders its render-prop
 *     children with a no-op formatQuery and exposes a textarea so we can
 *     observe `onQueryTextChange`. Monaco itself is never instantiated.
 *   - ./QuerySettings, ./InterpolatedQuery, ./ValidationBar → stubs that
 *     surface their props as data attributes. Their own behavior is
 *     covered by their dedicated test files.
 *   - props.datasource → a minimal shape with `templateSrv.getVariables()`
 *     returning [] and a jest.fn() for interpolateQuery.
 *
 * Stateful harness:
 *   Grafana's Input is fully controlled by props.query, so a static jest.fn()
 *   for onChange leaves the DOM out of sync after each keystroke. Tests that
 *   need typed characters to persist (round/rawSql edits, invalid-duration
 *   error) use <StatefulHarness>, which feeds onChange back into local state.
 *   Tests that only need to observe a single call (defaults, run button)
 *   render <QueryEditor> directly.
 *
 * Covered behavior:
 *   - On first mount with `format` undefined, useEffect calls onChange with
 *     format = QueryType.Table (default render format).
 *   - Editing the rawSql textarea (through the stubbed SQLEditor) calls
 *     onChange with the new rawSql.
 *   - Editing the Round input calls onChange with the new round string.
 *   - An invalid round duration ("abc") flips invalidDuration → the
 *     InlineField renders the "invalid duration" error message.
 *   - A valid round duration ("5m") leaves the error message hidden.
 *   - Clicking "Show Interpolated Query" toggles the button label to
 *     "Hide Interpolated Query".
 *   - Clicking the run toolbar button invokes props.onRunQuery.
 *   - QuerySettings receives the current querySettings array as a prop,
 *     and InterpolatedQuery receives showSQL=false by default.
 *
 * Not covered here:
 *   - The Query Type Select dropdown change (react-select portal — better
 *     in e2e).
 *   - The debounced interpolateQuery side-effect (timing-dependent;
 *     covered indirectly by the datasource unit tests).
 *   - Format-query toolbar button (delegates to SQLEditor's render-prop;
 *     the stubbed formatQuery has nothing to verify).
 */
import React, { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";

jest.mock("@grafana/plugin-ui", () => {
  const actual = jest.requireActual("@grafana/plugin-ui");
  const React = require("react");
  return {
    ...actual,
    SQLEditor: ({ query, onChange, children }: any) => (
      <div data-testid="sql-editor">
        <textarea
          data-testid="sql-editor-input"
          value={query ?? ""}
          onChange={(e) => onChange(e.target.value)}
        />
        {typeof children === "function"
          ? children({ formatQuery: jest.fn() })
          : null}
      </div>
    ),
  };
});

jest.mock("./QuerySettings", () => ({
  QuerySettings: ({ settings }: any) => (
    <div
      data-testid="query-settings-stub"
      data-settings-count={settings?.length ?? 0}
    />
  ),
}));

jest.mock("./InterpolatedQuery", () => ({
  InterpolatedQuery: ({ showSQL, sql, dirty }: any) => (
    <div
      data-testid="interpolated-query-stub"
      data-show-sql={String(showSQL)}
      data-dirty={String(dirty)}
      data-sql={sql}
    />
  ),
}));

jest.mock("./ValidationBar", () => ({
  ValidationBar: () => <div data-testid="validation-bar-stub" />,
}));

// Imports must come after jest.mock calls so the mocks are applied.
import { QueryEditor, Props } from "./QueryEditor";
import { HdxQuery, QueryType } from "../types";

function makeProps(overrides: Partial<HdxQuery> = {}): Props {
  const query: HdxQuery = {
    refId: "A",
    rawSql: "SELECT 1",
    round: "1m",
    querySettings: [],
    ...overrides,
  } as HdxQuery;

  const datasource: any = {
    options: { jsonData: {} },
    templateSrv: { getVariables: () => [] },
    interpolateQuery: jest.fn().mockResolvedValue({
      originalSql: query.rawSql,
      interpolationId: "1",
      interpolatedSql: query.rawSql,
      hasError: false,
      hasWarning: false,
    }),
  };

  return {
    query,
    datasource,
    onChange: jest.fn(),
    onRunQuery: jest.fn(),
  } as unknown as Props;
}

// Stateful wrapper used when a test needs the controlled inputs to actually
// reflect typed characters (Grafana's Input is controlled by props.query.X,
// so a static jest.fn() leaves the DOM out of sync after each keystroke).
function StatefulHarness({
  initial,
  onChangeSpy,
}: {
  initial: Partial<HdxQuery>;
  onChangeSpy: jest.Mock;
}) {
  const baseProps = makeProps(initial);
  const [query, setQuery] = useState<HdxQuery>(baseProps.query);
  return (
    <QueryEditor
      {...baseProps}
      query={query}
      onChange={(q: HdxQuery) => {
        onChangeSpy(q);
        setQuery(q);
      }}
    />
  );
}

describe("QueryEditor", () => {
  it("defaults format to QueryType.Table on first render when format is undefined", () => {
    const props = makeProps({ format: undefined });
    render(<QueryEditor {...props} />);
    expect(props.onChange).toHaveBeenCalledWith(
      expect.objectContaining({ format: QueryType.Table })
    );
  });

  it("does not override format when one is already set", () => {
    const props = makeProps({ format: QueryType.TimeSeries });
    render(<QueryEditor {...props} />);
    // No call with a different format
    const calls = (props.onChange as jest.Mock).mock.calls;
    const formatChange = calls.find(
      ([arg]) => arg.format !== QueryType.TimeSeries && arg.format !== undefined
    );
    expect(formatChange).toBeUndefined();
  });

  it("propagates rawSql edits through onChange", () => {
    const onChangeSpy = jest.fn();
    render(
      <StatefulHarness
        initial={{ format: QueryType.Table, rawSql: "" }}
        onChangeSpy={onChangeSpy}
      />
    );
    const input = screen.getByTestId("sql-editor-input");
    fireEvent.change(input, { target: { value: "SELECT 2" } });
    expect(onChangeSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ rawSql: "SELECT 2" })
    );
  });

  it("propagates round edits through onChange", () => {
    const onChangeSpy = jest.fn();
    render(
      <StatefulHarness
        initial={{ format: QueryType.Table, round: "" }}
        onChangeSpy={onChangeSpy}
      />
    );
    const round = screen.getByTestId("data-testid round input");
    fireEvent.change(round, { target: { value: "5m" } });
    expect(onChangeSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ round: "5m" })
    );
  });

  it("shows the 'invalid duration' error when the round input is not a valid duration", () => {
    const onChangeSpy = jest.fn();
    render(
      <StatefulHarness
        initial={{ format: QueryType.Table, round: "" }}
        onChangeSpy={onChangeSpy}
      />
    );
    const round = screen.getByTestId("data-testid round input");
    fireEvent.change(round, { target: { value: "abc" } });
    expect(screen.getByText("invalid duration")).toBeInTheDocument();
  });

  it("does not show the 'invalid duration' error for a valid duration", () => {
    const onChangeSpy = jest.fn();
    render(
      <StatefulHarness
        initial={{ format: QueryType.Table, round: "" }}
        onChangeSpy={onChangeSpy}
      />
    );
    const round = screen.getByTestId("data-testid round input");
    fireEvent.change(round, { target: { value: "5m" } });
    expect(screen.queryByText("invalid duration")).not.toBeInTheDocument();
  });

  it("toggles the show/hide interpolated query button label", async () => {
    const props = makeProps({ format: QueryType.Table });
    render(<QueryEditor {...props} />);
    const showButton = screen.getByRole("button", {
      name: /Show Interpolated Query/i,
    });
    await userEvent.click(showButton);
    expect(
      screen.getByRole("button", { name: /Hide Interpolated Query/i })
    ).toBeInTheDocument();
  });

  it("calls onRunQuery when the run toolbar button is clicked", async () => {
    const props = makeProps({ format: QueryType.Table });
    render(<QueryEditor {...props} />);
    const runButton = screen.getByLabelText(
      /Click or hit CTRL\/CMD\+Return to run query/i
    );
    await userEvent.click(runButton);
    expect(props.onRunQuery).toHaveBeenCalled();
  });

  it("passes the current querySettings to QuerySettings and starts with showSQL=false", () => {
    const props = makeProps({
      format: QueryType.Table,
      querySettings: [
        { setting: "hdx_query_max_rows", value: "10" },
        { setting: "hdx_query_admin_comment", value: "hi" },
      ],
    });
    render(<QueryEditor {...props} />);
    expect(screen.getByTestId("query-settings-stub")).toHaveAttribute(
      "data-settings-count",
      "2"
    );
    expect(screen.getByTestId("interpolated-query-stub")).toHaveAttribute(
      "data-show-sql",
      "false"
    );
  });
});
