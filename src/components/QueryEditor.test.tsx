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
 *     returning [] and jest.fn()s for interpolateQuery and validateQuery.
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
 *   - Showing the interpolated query calls interpolateQuery with context built
 *     from props (range, derived interval, panel-request filters) and issues no
 *     preparatory panel run.
 *   - Validation (fake timers): debounce, result mapping, failures,
 *     cancellation, stale responses, and what triggers re-validation.
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
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { dateTime, makeTimeRange } from "@grafana/data";

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

jest.mock("@grafana/runtime", () => ({
  ...jest.requireActual("@grafana/runtime"),
  logError: jest.fn(),
}));

jest.mock("./ValidationBar", () => ({
  ...jest.requireActual("./ValidationBar"),
  ValidationBar: ({ state }: any) => (
    <div
      data-testid="validation-bar-stub"
      data-status={state.status}
      data-message={state.message ?? ""}
    />
  ),
}));

// Availability is the gate for page-context registration and the explain
// action. useProvidePageContext registers globally on mount, so "was the hook
// called" is the observable form of "did anything register" — the property
// mount-gating exists to guarantee on Assistant-less installs.
jest.mock("@grafana/assistant", () => {
  const React = require("react");
  return {
    useAssistant: jest.fn(() => ({ isAvailable: false })),
    useProvidePageContext: jest.fn(() => jest.fn()),
    createAssistantContextItem: jest.fn((type: string, params: any) => ({
      node: { id: type, name: type, navigable: false, data: params?.data },
      occurrences: [],
    })),
    OpenAssistantButton: ({ title }: any) => (
      <div data-testid="explain-error-stub" data-title={title} />
    ),
    QueryWithAssistantButton: () => (
      <div data-testid="query-with-assistant-stub" />
    ),
  };
});

// Imports must come after jest.mock calls so the mocks are applied.
import { useAssistant, useProvidePageContext } from "@grafana/assistant";
import { logError } from "@grafana/runtime";
import { QueryEditor, Props } from "./QueryEditor";
import { HdxQuery, QueryType } from "../types";
import { deriveInterpolationInterval } from "../editor/timeRangeUtils";
import { VALIDATION_DEBOUNCE_MS } from "../constants";

function makeProps(
  overrides: Partial<HdxQuery> = {},
  propOverrides: Partial<Props> = {}
): Props {
  const query: HdxQuery = {
    refId: "A",
    rawSql: "SELECT 1",
    round: "1m",
    querySettings: [],
    ...overrides,
  } as HdxQuery;

  // No cached request state on the mock: interpolation must work from props
  // alone, so a `datasource.options`-shaped field would be misleading here.
  const datasource: any = {
    uid: "hdx-uid",
    options: { jsonData: {} },
    instanceSettings: { jsonData: { host: "cluster.example" } },
    // Reached only by AssistantQueryContext's debounced publish; stubbed so an
    // available-Assistant render has nothing live to hit.
    getAst: jest.fn().mockResolvedValue([]),
    metadataProvider: {
      columns: jest.fn().mockResolvedValue([]),
      primaryKey: jest.fn().mockResolvedValue("timestamp"),
    },
    templateSrv: { getVariables: () => [] },
    validateQuery: jest.fn().mockResolvedValue({}),
    interpolateQuery: jest.fn().mockResolvedValue({
      originalSql: query.rawSql,
      interpolationId: "1",
      interpolatedSql: query.rawSql,
      hasError: false,
    }),
  };

  return {
    query,
    datasource,
    onChange: jest.fn(),
    onRunQuery: jest.fn(),
    ...propOverrides,
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

  it("resets an invalid round to '' and clears the error on blur", () => {
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
    fireEvent.blur(round);
    expect(onChangeSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ round: "" })
    );
    expect(screen.queryByText("invalid duration")).not.toBeInTheDocument();
  });

  it("keeps a valid round value on blur", () => {
    const onChangeSpy = jest.fn();
    render(
      <StatefulHarness
        initial={{ format: QueryType.Table, round: "" }}
        onChangeSpy={onChangeSpy}
      />
    );
    const round = screen.getByTestId("data-testid round input");
    fireEvent.change(round, { target: { value: "5m" } });
    fireEvent.blur(round);
    expect(onChangeSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ round: "5m" })
    );
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

  it("interpolates from props on a panel with no completed run", async () => {
    const to = 1_700_000_000_000;
    const from = to - 6 * 60 * 60 * 1000;
    const range = makeTimeRange(dateTime(from), dateTime(to));
    const filters = [{ key: "status", operator: "=", value: "ok" }];
    const props = makeProps(
      { format: QueryType.Table },
      {
        range,
        data: { request: { maxDataPoints: 1000, filters } } as any,
      }
    );
    render(<QueryEditor {...props} />);

    await userEvent.click(
      screen.getByRole("button", { name: /Show Interpolated Query/i })
    );

    const interpolateQuery = (props.datasource as any).interpolateQuery;
    await waitFor(() => expect(interpolateQuery).toHaveBeenCalled());

    const context = interpolateQuery.mock.calls[0][2];
    expect(context.range).toBe(range);
    expect(context.filters).toBe(filters);
    expect(context.interval).toBe(
      deriveInterpolationInterval(range, 1000)
    );

    // No preparatory query: interpolation must not need a panel run to have
    // populated anything first.
    expect(props.onRunQuery).not.toHaveBeenCalled();
  });

  // `undefined` is dropped by JSON.stringify, so the backend would decode
  // Interval as "" and time.ParseDuration("") fails the whole interpolate
  // request. A parseable zero degrades cleanly instead.
  it("sends a parseable zero interval when the panel has no range", async () => {
    const props = makeProps(
      { format: QueryType.Table },
      { range: undefined, data: { request: { maxDataPoints: 1000 } } as any }
    );
    render(<QueryEditor {...props} />);

    await userEvent.click(
      screen.getByRole("button", { name: /Show Interpolated Query/i })
    );

    const interpolateQuery = (props.datasource as any).interpolateQuery;
    await waitFor(() => expect(interpolateQuery).toHaveBeenCalled());

    const context = interpolateQuery.mock.calls[0][2];
    expect(context.range).toBeUndefined();
    expect(context.interval).toBe("0ms");
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

describe("QueryEditor validation", () => {
  beforeEach(() => {
    jest.useFakeTimers();
    jest.mocked(logError).mockClear();
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  const bar = () => screen.getByTestId("validation-bar-stub");

  async function flushDebounce() {
    await act(async () => {
      jest.advanceTimersByTime(VALIDATION_DEBOUNCE_MS);
    });
  }

  it("validates after the debounce and shows a valid result", async () => {
    const props = makeProps({ format: QueryType.Table });
    render(<QueryEditor {...props} />);
    const validateQuery = (props.datasource as any).validateQuery;

    expect(bar()).toHaveAttribute("data-status", "validating");
    expect(validateQuery).not.toHaveBeenCalled();

    await flushDebounce();

    expect(validateQuery).toHaveBeenCalledTimes(1);
    const [query, context, requestId] = validateQuery.mock.calls[0];
    expect(query.rawSql).toBe("SELECT 1");
    expect(context.interval).toBe("0ms");
    expect(typeof requestId).toBe("string");
    expect(bar()).toHaveAttribute("data-status", "valid");
  });

  it.each([
    [{ error: "Missing columns" }, "error", "Missing columns"],
    [
      { warning: "Primary key not filtered" },
      "warning",
      "Primary key not filtered",
    ],
    [{ skipped: true }, "idle", ""],
  ])("maps %j to the %s state", async (result, status, message) => {
    const props = makeProps({ format: QueryType.Table });
    (props.datasource as any).validateQuery.mockResolvedValue(result);
    render(<QueryEditor {...props} />);

    await flushDebounce();

    expect(bar()).toHaveAttribute("data-status", status);
    expect(bar()).toHaveAttribute("data-message", message);
  });

  it("turns a failed validation call into a warning that names the reason", async () => {
    const props = makeProps({ format: QueryType.Table });
    const failure = new Error("An error occurred within the plugin");
    (props.datasource as any).validateQuery.mockRejectedValue(failure);
    render(<QueryEditor {...props} />);

    await flushDebounce();

    expect(bar()).toHaveAttribute("data-status", "warning");
    expect(bar()).toHaveAttribute(
      "data-message",
      "Could not validate query: An error occurred within the plugin"
    );
    expect(logError).toHaveBeenCalledWith(
      failure,
      expect.objectContaining({ source: "query-validation" })
    );
  });

  it("ignores a cancelled validation", async () => {
    const props = makeProps({ format: QueryType.Table });
    (props.datasource as any).validateQuery.mockResolvedValue(undefined);
    render(<QueryEditor {...props} />);

    await flushDebounce();

    expect(bar()).toHaveAttribute("data-status", "validating");
    expect(logError).not.toHaveBeenCalled();
  });

  it("passes the panel's scoped variables to validation", async () => {
    const scopedVars = { __interval_ms: { text: "60000", value: "60000" } };
    const props = makeProps(
      { format: QueryType.Table },
      { data: { request: { scopedVars } } as any }
    );
    render(<QueryEditor {...props} />);

    await flushDebounce();

    const context = (props.datasource as any).validateQuery.mock.calls[0][1];
    expect(context.scopedVars).toBe(scopedVars);
  });

  it("stays idle and does not validate empty SQL", async () => {
    const props = makeProps({ format: QueryType.Table, rawSql: "  " });
    render(<QueryEditor {...props} />);

    await flushDebounce();

    expect(bar()).toHaveAttribute("data-status", "idle");
    expect((props.datasource as any).validateQuery).not.toHaveBeenCalled();
  });

  it("reuses the requestId and shows validating until the newer edit is checked", async () => {
    const props = makeProps({ format: QueryType.Table });
    const validateQuery = (props.datasource as any).validateQuery;
    let resolveFirst: (v: object) => void = () => {};
    validateQuery
      .mockImplementationOnce(
        () => new Promise((resolve) => (resolveFirst = resolve))
      )
      .mockResolvedValueOnce({});
    const { rerender } = render(<QueryEditor {...props} />);
    await flushDebounce();

    rerender(
      <QueryEditor {...props} query={{ ...props.query, rawSql: "SELECT 2" }} />
    );
    await act(async () => resolveFirst({ error: "stale" }));
    expect(bar()).toHaveAttribute("data-status", "validating");

    await flushDebounce();

    expect(validateQuery).toHaveBeenCalledTimes(2);
    expect(validateQuery.mock.calls[1][0].rawSql).toBe("SELECT 2");
    expect(validateQuery.mock.calls[1][2]).toBe(validateQuery.mock.calls[0][2]);
    expect(bar()).toHaveAttribute("data-status", "valid");
  });

  it("keeps the newer result when an older response arrives late", async () => {
    const props = makeProps({ format: QueryType.Table });
    const validateQuery = (props.datasource as any).validateQuery;
    let resolveFirst: (v: object) => void = () => {};
    validateQuery
      .mockImplementationOnce(
        () => new Promise((resolve) => (resolveFirst = resolve))
      )
      .mockResolvedValueOnce({});
    const { rerender } = render(<QueryEditor {...props} />);
    await flushDebounce();
    rerender(
      <QueryEditor {...props} query={{ ...props.query, rawSql: "SELECT 2" }} />
    );
    await flushDebounce();
    expect(bar()).toHaveAttribute("data-status", "valid");

    await act(async () => resolveFirst({ error: "stale" }));

    expect(bar()).toHaveAttribute("data-status", "valid");
  });

  it("re-validates when the query settings change", async () => {
    const props = makeProps({ format: QueryType.Table });
    const { rerender } = render(<QueryEditor {...props} />);
    await flushDebounce();

    rerender(
      <QueryEditor
        {...props}
        query={{
          ...props.query,
          querySettings: [{ setting: "max_threads", value: "4" }],
        }}
      />
    );
    await flushDebounce();

    expect((props.datasource as any).validateQuery).toHaveBeenCalledTimes(2);
  });

  it("re-validates when a dashboard variable changes", async () => {
    const props = makeProps({ format: QueryType.Table });
    const variables = [{ current: { value: "a" } }];
    (props.datasource as any).templateSrv.getVariables = () => variables;
    const { rerender } = render(<QueryEditor {...props} />);
    await flushDebounce();

    variables[0].current.value = "b";
    rerender(<QueryEditor {...props} />);
    await flushDebounce();

    expect((props.datasource as any).validateQuery).toHaveBeenCalledTimes(2);
  });

  it("does not re-validate when only the time range changes", async () => {
    const to = 1_700_000_000_000;
    const props = makeProps(
      { format: QueryType.Table },
      { range: makeTimeRange(dateTime(to - 3_600_000), dateTime(to)) }
    );
    const { rerender } = render(<QueryEditor {...props} />);
    await flushDebounce();

    rerender(
      <QueryEditor
        {...props}
        range={makeTimeRange(dateTime(to), dateTime(to + 3_600_000))}
      />
    );
    await flushDebounce();

    expect((props.datasource as any).validateQuery).toHaveBeenCalledTimes(1);
  });
});

describe("QueryEditor Assistant gating", () => {
  const mockUseAssistant = useAssistant as unknown as jest.Mock;
  const mockUseProvidePageContext =
    useProvidePageContext as unknown as jest.Mock;

  // A failed response for this editor's own refId — the precondition for the
  // explain action, so its absence below is the gate and not a missing error.
  const failedData: any = {
    state: "Error",
    series: [],
    errors: [{ refId: "A", message: "boom" }],
    timeRange: {},
  };

  beforeEach(() => {
    jest.clearAllMocks();
    mockUseAssistant.mockReturnValue({ isAvailable: false });
    mockUseProvidePageContext.mockReturnValue(jest.fn());
  });

  it("registers no page context and renders no Assistant UI when Assistant is unavailable", () => {
    const props = makeProps();
    render(<QueryEditor {...props} data={failedData} />);

    expect(mockUseProvidePageContext).not.toHaveBeenCalled();
    expect(screen.queryByTestId("explain-error-stub")).not.toBeInTheDocument();
    // QueryWithAssistantButton self-gates inside the SDK, but the spec puts
    // the availability decision on the plugin, so QueryEditor gates it too.
    expect(
      screen.queryByTestId("query-with-assistant-stub")
    ).not.toBeInTheDocument();
  });

  it("registers page context and renders both Assistant surfaces when available", () => {
    mockUseAssistant.mockReturnValue({ isAvailable: true });
    const props = makeProps();
    render(<QueryEditor {...props} data={failedData} />);

    expect(mockUseProvidePageContext).toHaveBeenCalled();
    expect(screen.getByTestId("explain-error-stub")).toBeInTheDocument();
    expect(screen.getByTestId("query-with-assistant-stub")).toBeInTheDocument();
  });

  it("keeps the editor itself intact when Assistant is unavailable", () => {
    const props = makeProps();
    render(<QueryEditor {...props} />);

    expect(screen.getByTestId("sql-editor")).toBeInTheDocument();
    expect(screen.getByTestId("query-settings-stub")).toBeInTheDocument();
  });
});
