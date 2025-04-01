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
  TextArea,
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

  const onUpdateAdHocKeysQuery = (e: FormEvent<HTMLTextAreaElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        adHocKeysQuery: (e.target as HTMLTextAreaElement).value,
      },
    });
  };

  const onUpdateAdHocValuesQuery = (e: FormEvent<HTMLTextAreaElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        adHocValuesQuery: (e.target as HTMLTextAreaElement).value,
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
      <ConfigSection title={"Server"}>
        <Field
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
            <InlineField label={labels.useDefaultPort.label} interactive>
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
            onChange={onUpdateDatasourceSecureJsonDataOption(props, "password")}
          />
        </Field>
      </ConfigSection>
      <Divider />
      <ConfigSection
        title="Additional Settings"
        isCollapsible
        isInitiallyOpen={false}
      >
        <Field
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
          error={"invalid duration"}
          label={labels.defaultQueryRound.label}
          description={labels.defaultQueryRound.description}
          invalid={invalidDuration.current}
        >
          <Input
            width={40}
            onChange={onRoundChange}
            value={jsonData.defaultRound}
          />
        </Field>
        <Field
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
          label={labels.adHocKeysQuery.label}
          description={labels.adHocKeysQuery.description}
        >
          <div style={{ width: "50em" }}>
            <TextArea
              name={"adHocKeyQuery"}
              cols={40}
              rows={4}
              value={jsonData.adHocKeysQuery}
              onChange={onUpdateAdHocKeysQuery}
              label={labels.adHocKeysQuery.label}
              aria-label={labels.adHocKeysQuery.label}
              placeholder={labels.adHocKeysQuery.placeholder}
            />
          </div>
        </Field>
        <Field
          label={labels.adHocValuesQuery.label}
          description={labels.adHocValuesQuery.description}
        >
          <div style={{ width: "50em" }}>
            <TextArea
              name={"adHocKeyQuery"}
              cols={40}
              rows={4}
              value={jsonData.adHocValuesQuery}
              onChange={onUpdateAdHocValuesQuery}
              label={labels.adHocValuesQuery.label}
              aria-label={labels.adHocValuesQuery.label}
              placeholder={labels.adHocValuesQuery.placeholder}
            />
          </div>
        </Field>
        <Field
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
    </>
  );
}
