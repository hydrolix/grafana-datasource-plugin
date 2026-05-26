/**
 * QuerySettings – covered cases
 *
 * Collapsed header summary:
 *   - When closed, each existing setting is shown inline as "key=value"
 *     spans in the header (so users see active settings without expanding).
 *   - The "Query Settings" title is always rendered.
 *
 * Expanded body — pre-populated settings:
 *   - A row is rendered per setting in props.settings.
 *   - The input widget is chosen by the type defined in labels.ts:
 *       number   → <Input> with aria-label = setting name
 *       textarea → <Input> with aria-label = setting name
 *       boolean  → <Select> with Yes/No options (not driven here — react-select
 *                  portals make jsdom interactions brittle; we only assert
 *                  presence by checking the row count).
 *
 * Mutation handlers (each call onSettingsChange):
 *   - Editing a number/textarea input fires onSettingsChange with the updated
 *     value, preserving the setting name and type.
 *   - Clicking the row's "delete setting" button fires onSettingsChange with
 *     the row removed.
 *   - Clicking "new setting" appends an empty-name row of type "string".
 *
 * showAdd guard:
 *   - The "new setting" button is hidden while any row has an empty
 *     `setting` (prevents the user from queuing multiple unnamed rows).
 *
 * Not covered here:
 *   - Driving the react-select dropdowns (setting picker, boolean value
 *     picker). Those are better exercised in e2e (Playwright).
 *   - Styling / theme output.
 */
import React from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { QuerySettings } from "./QuerySettings";
import { QuerySetting } from "../types";

function setup(settings: QuerySetting[] = []) {
  const onSettingsChange = jest.fn();
  const utils = render(
    <QuerySettings settings={settings} onSettingsChange={onSettingsChange} />
  );
  return { ...utils, onSettingsChange };
}

async function expand() {
  // The Collapse header is a button labeled with the title text.
  await userEvent.click(screen.getByText("Query Settings"));
}

describe("QuerySettings", () => {
  it("renders the title and the inline summary of existing settings when collapsed", () => {
    setup([
      { setting: "hdx_query_max_rows", value: "100" },
      { setting: "hdx_query_admin_comment", value: "from grafana" },
    ]);
    expect(screen.getByText("Query Settings")).toBeInTheDocument();
    expect(screen.getByText(/hdx_query_max_rows=100/)).toBeInTheDocument();
    expect(
      screen.getByText(/hdx_query_admin_comment=from grafana/)
    ).toBeInTheDocument();
  });

  it("renders a typed input per pre-populated setting once expanded", async () => {
    setup([
      { setting: "hdx_query_max_rows", value: "100" },
      { setting: "hdx_query_admin_comment", value: "from grafana" },
    ]);
    await expand();

    const numberInput = screen.getByLabelText("hdx_query_max_rows");
    const textInput = screen.getByLabelText("hdx_query_admin_comment");
    expect(numberInput).toHaveValue("100");
    expect(textInput).toHaveValue("from grafana");
  });

  it("calls onSettingsChange when a value is edited", async () => {
    const { onSettingsChange } = setup([
      { setting: "hdx_query_max_rows", value: "100" },
    ]);
    await expand();

    const input = screen.getByLabelText("hdx_query_max_rows");
    await userEvent.clear(input);
    await userEvent.type(input, "5");

    expect(onSettingsChange).toHaveBeenLastCalledWith([
      { setting: "hdx_query_max_rows", value: "5", type: "number" },
    ]);
  });

  it("removes a row when the delete button is clicked", async () => {
    const { onSettingsChange } = setup([
      { setting: "hdx_query_max_rows", value: "100" },
      { setting: "hdx_query_admin_comment", value: "note" },
    ]);
    await expand();

    const deleteButtons = screen.getAllByLabelText("delete setting");
    expect(deleteButtons).toHaveLength(2);
    await userEvent.click(deleteButtons[0]);

    expect(onSettingsChange).toHaveBeenLastCalledWith([
      { setting: "hdx_query_admin_comment", value: "note", type: "textarea" },
    ]);
  });

  it("appends an empty row when the 'new setting' button is clicked, then hides the button", async () => {
    const { onSettingsChange } = setup([
      { setting: "hdx_query_max_rows", value: "100" },
    ]);
    await expand();

    await userEvent.click(screen.getByLabelText("new setting"));

    expect(onSettingsChange).toHaveBeenLastCalledWith([
      { setting: "hdx_query_max_rows", value: "100", type: "number" },
      { setting: "", value: "", type: "string" },
    ]);
    expect(screen.queryByLabelText("new setting")).not.toBeInTheDocument();
  });

  it("renders the 'new setting' button when starting from an empty list", async () => {
    setup([]);
    await expand();
    expect(screen.getByLabelText("new setting")).toBeInTheDocument();
  });

  it("renders a row for a boolean-typed setting (widget presence only)", async () => {
    setup([{ setting: "hdx_query_timerange_required", value: "1" }]);
    await expand();

    // delete-button presence proves the row was rendered for the boolean type
    // without needing to drive react-select's portal-based dropdown.
    const rows = screen.getAllByLabelText("delete setting");
    expect(rows).toHaveLength(1);

    // Sanity: the "=" disabled separator input is rendered next to the row.
    const separators = screen
      .getAllByDisplayValue("=")
      .filter((el) => (el as HTMLInputElement).disabled);
    expect(separators.length).toBeGreaterThan(0);
  });
});
