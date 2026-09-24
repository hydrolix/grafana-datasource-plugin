import React from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import { ConfigEditor, Props } from "./ConfigEditor";
import "@testing-library/jest-dom";
import fs from "fs";
import { HdxDataSourceOptions } from "types";
import allLabels from "labels";

const pluginJson = JSON.parse(fs.readFileSync("./src/plugin.json", "utf-8"));

jest.mock("@grafana/runtime", () => {
  const original = jest.requireActual("@grafana/runtime");
  return {
    ...original,
    config: {
      buildInfo: { version: "10.0.0" },
      secureSocksDSProxyEnabled: true,
    },
  };
});

function getDefaultProps(overrides: HdxDataSourceOptions) {
  return {
    ...pluginJson,
    options: {
      jsonData: {
        host: "https://domain.com",
        port: 433,
        useDefaultPort: false,
        username: "use",
        ...overrides,
      },
      secureJsonData: { password: "pass" },
      secureJsonFields: { password: true },
    },
  } as Props;
}

describe("ConfigEditor", () => {
  let labels = allLabels.components.config.editor;

  it("new editor", () => {
    render(<ConfigEditor {...getDefaultProps({})} />);
    expect(screen.getByLabelText(labels.host.label)).toBeInTheDocument();
    expect(screen.getByLabelText(labels.username.label)).toBeInTheDocument();
    expect(screen.getByLabelText(labels.password.label)).toBeInTheDocument();
  });

  // The Attribution section lives inside the collapsible "Additional
  // Settings" config section. ConfigSection's collapse button is an
  // IconButton with aria-label "Expand section <title>" (or "Collapse …"
  // when open); click it to reveal the nested fields.
  // @ts-ignore
  function expandAdditionalSettings() {
    fireEvent.click(
        screen.getByLabelText(`Expand section ${labels.additionalSettings.label}`)
    );
  }

  it("resets an invalid default round to '' and clears the error on blur", () => {
    const onOptionsChange = jest.fn();
    const props = getDefaultProps({ defaultRound: "" });
    render(<ConfigEditor {...props} onOptionsChange={onOptionsChange} />);
    expandAdditionalSettings();
    const round = screen
      .getByTestId(labels.defaultRound.testId)
      .querySelector("input")!;
    fireEvent.change(round, { target: { value: "5x" } });
    expect(screen.getByText("invalid duration")).toBeInTheDocument();
    expect(onOptionsChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        jsonData: expect.objectContaining({ defaultRound: "5x" }),
      })
    );
    fireEvent.blur(round);
    expect(onOptionsChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        jsonData: expect.objectContaining({ defaultRound: "" }),
      })
    );
    expect(screen.queryByText("invalid duration")).not.toBeInTheDocument();
  });

  it("keeps a valid default round on blur", () => {
    const onOptionsChange = jest.fn();
    const props = getDefaultProps({ defaultRound: "5m" });
    render(<ConfigEditor {...props} onOptionsChange={onOptionsChange} />);
    expandAdditionalSettings();
    const round = screen
      .getByTestId(labels.defaultRound.testId)
      .querySelector("input")!;
    fireEvent.blur(round);
    expect(onOptionsChange).not.toHaveBeenCalled();
  });

  it("does not backfill jsonData when opening an existing datasource", () => {
    const onOptionsChange = jest.fn();
    render(
      <ConfigEditor {...getDefaultProps({})} onOptionsChange={onOptionsChange} />
    );
    expect(onOptionsChange).not.toHaveBeenCalled();
  });

  it("renders the ad hoc lookback input with the stored value", () => {
    render(
      <ConfigEditor {...getDefaultProps({ adHocTimeRangeLookback: "6h" })} />
    );
    expandAdditionalSettings();
    const lookback = screen.getByLabelText(
      labels.adHocTimeRangeLookback.label
    ) as HTMLInputElement;
    expect(lookback.value).toBe("6h");
    expect(lookback.placeholder).toBe("24h");
    expect(screen.queryByText("invalid duration")).not.toBeInTheDocument();
  });

  it("flags an invalid ad hoc lookback without clearing it on blur", () => {
    const onOptionsChange = jest.fn();
    render(
      <ConfigEditor
        {...getDefaultProps({ adHocTimeRangeLookback: "abc" })}
        onOptionsChange={onOptionsChange}
      />
    );
    expandAdditionalSettings();
    expect(screen.getByText("invalid duration")).toBeInTheDocument();
    fireEvent.blur(screen.getByLabelText(labels.adHocTimeRangeLookback.label));
    expect(onOptionsChange).not.toHaveBeenCalled();
  });

  it("writes the ad hoc lookback to jsonData on change", () => {
    const onOptionsChange = jest.fn();
    render(
      <ConfigEditor {...getDefaultProps({})} onOptionsChange={onOptionsChange} />
    );
    expandAdditionalSettings();
    fireEvent.change(
      screen.getByLabelText(labels.adHocTimeRangeLookback.label),
      { target: { value: "7d" } }
    );
    expect(onOptionsChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        jsonData: expect.objectContaining({ adHocTimeRangeLookback: "7d" }),
      })
    );
  });

  // it('port input is enabled', () => {
  //     let component = render(<ConfigEditor {...getDefaultProps({})} />);
  //     expect(component.container.querySelector('#config-editor-port')?.getAttribute("disabled")).toBeNull();
  // });
  //
  // it('port input is disabled', () => {
  //     let component = render(<ConfigEditor {...getDefaultProps({})} />);
  //     expect(component.container.querySelector('#config-editor-port')?.getAttribute("disabled")).toBe("");
  // });
});
