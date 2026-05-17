import React from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import { ConfigEditor, Props } from "./ConfigEditor";
import "@testing-library/jest-dom";
import fs from "fs";
import { HdxDataSourceOptions } from "types";
import allLabels from "labels";
import defaultConfigs from "defaultConfigs";

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
        adHocDefaultTimeRange: defaultConfigs.adHocDefaultTimeRange,
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
  function expandAdditionalSettings() {
    fireEvent.click(
      screen.getByLabelText(`Expand section ${labels.additionalSettings.label}`)
    );
  }

  it("renders the user-identity attribution toggle in the Attribution section", () => {
    render(<ConfigEditor {...getDefaultProps({})} />);
    expandAdditionalSettings();
    expect(
      screen.getByTestId(labels.includeUserIdentityInAttribution.testId)
    ).toBeInTheDocument();
  });

  it("user-identity attribution toggle defaults to off", () => {
    render(<ConfigEditor {...getDefaultProps({})} />);
    expandAdditionalSettings();
    const input = document.getElementById(
      "includeUserIdentityInAttribution"
    ) as HTMLInputElement;
    expect(input).toBeTruthy();
    expect(input.checked).toBe(false);
  });

  it("user-identity attribution toggle reflects saved value", () => {
    render(
      <ConfigEditor
        {...getDefaultProps({ includeUserIdentityInAttribution: true })}
      />
    );
    expandAdditionalSettings();
    const input = document.getElementById(
      "includeUserIdentityInAttribution"
    ) as HTMLInputElement;
    expect(input.checked).toBe(true);
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
