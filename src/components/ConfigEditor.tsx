import React, { FormEvent, useRef } from "react";
import {
  DataSourcePluginOptionsEditorProps,
  onUpdateDatasourceJsonDataOption,
  onUpdateDatasourceSecureJsonDataOption,
  TimeRange,
} from "@grafana/data";
import {
  Divider,
  Field,
  InlineField,
  InlineSwitch,
  Input,
  RadioButtonGroup,
  SecretInput,
  Stack,
  Switch,
  TimeRangeInput,
} from "@grafana/ui";
import { ConfigSection } from "@grafana/plugin-ui";
import { HdxDataSourceOptions, HdxSecureJsonData, Protocol } from "../types";
import allLabels from "labels";
import defaultConfigs from "defaultConfigs";
import { QUERY_DURATION_REGEX } from "../editor/timeRangeUtils";

export interface Props
  extends DataSourcePluginOptionsEditorProps<
    HdxDataSourceOptions,
    HdxSecureJsonData
  > {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  if (!Object.keys(options.jsonData).length) {
    options.jsonData = defaultConfigs;
  }
  const { jsonData, secureJsonFields } = options;

  if (!jsonData.adHocDefaultTimeRange) {
    jsonData.adHocDefaultTimeRange = defaultConfigs.adHocDefaultTimeRange;
  }

  const labels = allLabels.components.config.editor;
  const secureJsonData = (options.secureJsonData || {}) as HdxSecureJsonData;
  const protocolOptions = [
    { label: "Native", value: Protocol.Native },
    { label: "HTTP", value: Protocol.Http },
  ];

  const getDefaultPort = (protocol: Protocol, secure: boolean) =>
    secure
      ? protocol === Protocol.Native
        ? labels.port.secureNativePort
        : labels.port.secureHttpPort
      : protocol === Protocol.Native
      ? labels.port.insecureNativePort
      : labels.port.insecureHttpPort;

  const defaultPort = getDefaultPort(jsonData.protocol!, jsonData.secure!);
  const portDescription = `${labels.port.description} (default for ${
    jsonData.secure ? "secure" : ""
  } ${jsonData.protocol}: ${defaultPort})`;

  const onPortChange = (port: string) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        port: +port,
      },
    });
  };

  const onUseDefaultPortChange = (useDefault: boolean) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        useDefaultPort: useDefault,
        port: +defaultPort,
      },
    });
  };
  const onProtocolToggle = (protocol: Protocol) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        protocol: protocol,
        port: jsonData.useDefaultPort
          ? +getDefaultPort(protocol, jsonData.secure!)
          : jsonData.port,
      },
    });
  };
  const onSecureChange = (value: boolean) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        secure: value,
        port: jsonData.useDefaultPort
          ? +getDefaultPort(jsonData.protocol!, value)
          : jsonData.port,
      },
    });
  };
  const onTlsSettingsChange = (value: boolean) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        skipTlsVerify: value,
      },
    });
  };

  const onResetPassword = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        password: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        password: "",
      },
    });
  };

  const onUpdateTimeRange = (e: TimeRange) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        adHocDefaultTimeRange: e,
      },
    });
  };

  let invalidDuration = useRef(false);
  const onRoundChange = (e: FormEvent<HTMLInputElement>) => {
    let round = e.currentTarget.value;

    invalidDuration.current = !QUERY_DURATION_REGEX.test(round);
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        defaultRound: round,
      },
    });
  };

  return (
    <>
      <div data-testid="data-testid hydrolix_config_page">
        <ConfigSection title={"Server"}>
          <Field
            data-testid={labels.host.testId}
            required
            label={labels.host.label}
            description={labels.host.description}
            invalid={!jsonData.host}
            error={labels.host.error}
          >
            <Input
              name="host"
              width={80}
              value={jsonData.host || ""}
              onChange={onUpdateDatasourceJsonDataOption(props, "host")}
              label={labels.host.label}
              aria-label={labels.host.label}
              placeholder={labels.host.placeholder}
            />
          </Field>

          <Field
            data-testid={labels.port.testId}
            required
            label={labels.port.label}
            description={portDescription}
            invalid={!jsonData.port}
            error={labels.port.error}
          >
            <Stack direction="row">
              <Input
                name="port"
                width={40}
                type="number"
                value={jsonData.port!}
                disabled={jsonData.useDefaultPort}
                onChange={(e) => onPortChange(e.currentTarget.value)}
                label={labels.port.label}
                aria-label={labels.port.label}
              />
              <InlineField
                data-testId={labels.useDefaultPort.testId}
                label={labels.useDefaultPort.label}
                interactive
              >
                <InlineSwitch
                  onChange={(e) =>
                    onUseDefaultPortChange(e.currentTarget.checked)
                  }
                  value={jsonData.useDefaultPort}
                />
              </InlineField>
            </Stack>
          </Field>

          <Field
            data-testid={labels.protocol.testId}
            label={labels.protocol.label}
            description={labels.protocol.description}
          >
            <RadioButtonGroup<Protocol>
              options={protocolOptions}
              disabledOptions={[]}
              value={jsonData.protocol!}
              onChange={(e) => onProtocolToggle(e!)}
            />
          </Field>

          <Field
            data-testid={labels.secure.testId}
            label={labels.secure.label}
            description={labels.secure.description}
          >
            <Switch
              id="secure"
              className="gf-form"
              value={jsonData.secure}
              onChange={(e) => onSecureChange(e.currentTarget.checked)}
            />
          </Field>

          {jsonData.protocol === Protocol.Http && (
            <Field
              data-testid={labels.path.testId}
              label={labels.path.label}
              description={labels.path.description}
            >
              <Input
                value={jsonData.path}
                name="path"
                width={80}
                onChange={onUpdateDatasourceJsonDataOption(props, "path")}
                label={labels.path.label}
                aria-label={labels.path.label}
                placeholder={labels.path.placeholder}
              />
            </Field>
          )}
        </ConfigSection>

        {jsonData.secure && (
          <>
            <Divider />
            <ConfigSection title="TLS / SSL Settings">
              <Field
                data-testid={labels.skipTlsVerify.testId}
                label={labels.skipTlsVerify.label}
                description={labels.skipTlsVerify.description}
              >
                <Switch
                  className="gf-form"
                  value={jsonData.skipTlsVerify}
                  onChange={(e) => onTlsSettingsChange(e.currentTarget.checked)}
                />
              </Field>
            </ConfigSection>
          </>
        )}

        <Divider />

        <ConfigSection title="Credentials">
          <Field
            data-testid={labels.username.testId}
            label={labels.username.label}
            description={labels.username.description}
          >
            <Input
              name={"username"}
              width={40}
              value={jsonData.username}
              onChange={onUpdateDatasourceJsonDataOption(props, "username")}
              label={labels.username.label}
              aria-label={labels.username.label}
              placeholder={labels.username.placeholder}
            />
          </Field>
          <Field
            data-testid={labels.password.testId}
            label={labels.password.label}
            description={labels.password.description}
          >
            <SecretInput
              name={"password"}
              width={40}
              label={labels.password.label}
              aria-label={labels.password.label}
              placeholder={labels.password.placeholder}
              value={secureJsonData.password || ""}
              isConfigured={
                (secureJsonFields && secureJsonFields.password) as boolean
              }
              onReset={onResetPassword}
              onChange={onUpdateDatasourceSecureJsonDataOption(
                props,
                "password"
              )}
            />
          </Field>
        </ConfigSection>
        <Divider />
        <ConfigSection
          data-testid={labels.additionalSettings.testId}
          title={labels.additionalSettings.label}
          isCollapsible
          isInitiallyOpen={false}
        >
          <Field
            data-testid={labels.defaultDatabase.testId}
            label={labels.defaultDatabase.label}
            description={labels.defaultDatabase.description}
          >
            <Input
              name={"defaultDatabase"}
              width={40}
              value={jsonData.defaultDatabase || ""}
              onChange={onUpdateDatasourceJsonDataOption(
                props,
                "defaultDatabase"
              )}
              label={labels.defaultDatabase.label}
              aria-label={labels.defaultDatabase.label}
              placeholder={labels.defaultDatabase.placeholder}
            />
          </Field>
          <Field
            data-testid={labels.defaultRound.testId}
            error={"invalid duration"}
            label={labels.defaultRound.label}
            description={labels.defaultRound.description}
            invalid={invalidDuration.current}
          >
            <Input
              width={40}
              onChange={onRoundChange}
              value={jsonData.defaultRound}
            />
          </Field>
          <Field
            data-testid={labels.adHocTableVariable.testId}
            label={labels.adHocTableVariable.label}
            description={labels.adHocTableVariable.description}
          >
            <Input
              name={"adHocTableVariable"}
              width={40}
              value={jsonData.adHocTableVariable || ""}
              onChange={onUpdateDatasourceJsonDataOption(
                props,
                "adHocTableVariable"
              )}
              label={labels.adHocTableVariable.label}
              aria-label={labels.adHocTableVariable.label}
            />
          </Field>
          <Field
            data-testid={labels.adHocTimeColumnVariable.testId}
            label={labels.adHocTimeColumnVariable.label}
            description={labels.adHocTimeColumnVariable.description}
          >
            <Input
              name={"adHocTimeFilterVariable"}
              width={40}
              value={jsonData.adHocTimeColumnVariable || ""}
              onChange={onUpdateDatasourceJsonDataOption(
                props,
                "adHocTimeColumnVariable"
              )}
              label={labels.adHocTimeColumnVariable.label}
              aria-label={labels.adHocTimeColumnVariable.label}
            />
          </Field>
          <Field
            data-testid={labels.adHocDefaultTimeRange.testId}
            label={labels.adHocDefaultTimeRange.label}
            description={labels.adHocDefaultTimeRange.description}
          >
            <div style={{ width: "23em" }}>
              <TimeRangeInput
                value={jsonData.adHocDefaultTimeRange!}
                onChange={onUpdateTimeRange}
                aria-label={labels.adHocDefaultTimeRange.label}
              />
            </div>
          </Field>
          <Field
            data-testid={labels.dialTimeout.testId}
            label={labels.dialTimeout.label}
            description={labels.dialTimeout.description}
          >
            <Input
              name={"dialTimeout"}
              width={40}
              value={jsonData.dialTimeout || ""}
              onChange={onUpdateDatasourceJsonDataOption(props, "dialTimeout")}
              label={labels.dialTimeout.label}
              aria-label={labels.dialTimeout.label}
              placeholder={labels.dialTimeout.placeholder}
              type="number"
            />
          </Field>
          <Field
            data-testid={labels.queryTimeout.testId}
            label={labels.queryTimeout.label}
            description={labels.queryTimeout.description}
          >
            <Input
              name={"queryTimeout"}
              width={40}
              value={jsonData.queryTimeout || ""}
              onChange={onUpdateDatasourceJsonDataOption(props, "queryTimeout")}
              label={labels.queryTimeout.label}
              aria-label={labels.queryTimeout.label}
              placeholder={labels.queryTimeout.placeholder}
              type="number"
            />
          </Field>
        </ConfigSection>
      </div>
    </>
  );
}
