import React, { FormEvent, useMemo, useRef, useState } from "react";
import {
  DataSourcePluginOptionsEditorProps,
  onUpdateDatasourceJsonDataOption,
  onUpdateDatasourceSecureJsonDataOption,
  TimeRange,
} from "@grafana/data";
import {
  Button,
  Divider,
  Field,
  InlineField,
  InlineSwitch,
  Input,
  RadioButtonGroup,
  SecretInput,
  Select,
  Stack,
  Switch,
  TextArea,
  TimeRangeInput,
} from "@grafana/ui";
import { ConfigSection } from "@grafana/plugin-ui";
import {
  CredentialsType,
  HdxDataSourceOptions,
  HdxSecureJsonData,
  Protocol,
} from "../types";
import allLabels from "labels";
import defaultConfigs from "defaultConfigs";
import { QUERY_DURATION_REGEX } from "../editor/timeRangeUtils";
import { getDefaultValue } from "../editor/editorUtils";

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
  const credentialsTypesOptions = [
    { label: "User Account", value: CredentialsType.UserAccount },
    { label: "Service Account", value: CredentialsType.ServiceAccount },
  ];
  let querySettingDefinitions = useMemo(() => {
    return labels.querySettings.values.reduce((acc, cur) => {
      acc[cur.setting] = cur;
      return acc;
    }, {} as { [setting: string]: any });
  }, [labels]);

  const existingSettings = (jsonData?.querySettings ?? []).map(
    (v) => v.setting
  );
  let queryOptions = labels.querySettings.values
    .filter((v) => !existingSettings.includes(v.setting))
    .map((v) => ({
      label: v.setting,
      value: v.setting,
      description: v.description,
    }));

  let [settingErrors, setSettingErrors] = useState<{
    [setting: string]: string;
  }>({});

  useMemo(() => {
    const errors: { [setting: string]: string } = {};
    for (const querySettings of jsonData?.querySettings ?? []) {
      const option = querySettingDefinitions[querySettings.setting];

      const type = option?.type;
      const min = option?.min;
      if (
        type &&
        ["number", "text", "duration"].includes(type) &&
        (querySettings.value === "" || querySettings.value === undefined)
      ) {
        errors[querySettings.setting] = "setting value is required";
      } else if (
        type &&
        type === "number" &&
        isNaN(querySettings.value as any)
      ) {
        errors[querySettings.setting] = `value is not a valid number`;
      } else if (
        type &&
        type === "duration" &&
        isNaN(querySettings.value as any) &&
        !QUERY_DURATION_REGEX.test(querySettings.value)
      ) {
        errors[querySettings.setting] = `value is not a valid duration`;
      } else if (
        type &&
        type === "number" &&
        min !== undefined &&
        min > +querySettings.value &&
        +querySettings.value !== option.default
      ) {
        errors[querySettings.setting] = `value cannot be less than ${min}`;
      }
    }
    setSettingErrors(errors);
  }, [jsonData, querySettingDefinitions]);

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
  const onCredentialsTypeToggle = (credentialsType: CredentialsType) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        credentialsType,
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

  const onResetToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        token: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        token: "",
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
  const onQuerySettingsChange = (key: string) => {
    return (value: string) => {
      let querySettings = jsonData?.querySettings ?? [];
      querySettings
        .filter((s) => s.setting === key)
        .forEach((s) => (s.value = value));
      onOptionsChange({
        ...options,
        jsonData: {
          ...jsonData,
          querySettings,
        },
      });
    };
  };

  const addQuerySetting = (setting: string) => {
    let querySettings = jsonData?.querySettings ?? [];
    let defaultValue = querySettingDefinitions[setting]?.default;
    let type: string = querySettingDefinitions[setting]?.type;
    querySettings.push({
      setting,
      value: getDefaultValue(defaultValue, type),
    });
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        querySettings,
      },
    });
  };

  const deleteQuerySetting = (setting: string) => {
    let querySettings = (jsonData?.querySettings ?? []).filter(
      (s) => s.setting !== setting
    );
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        querySettings,
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
  const settingInput = (key: string, value: string) => {
    let type = querySettingDefinitions[key].type;
    if (type === "boolean") {
      return (
        <Select
          width={80}
          options={[
            { value: "yes", label: "Yes" },
            { value: "no", label: "No" },
          ]}
          onChange={(v) =>
            onQuerySettingsChange(key)(v.value === "yes" ? "1" : "0")
          }
          value={value === "1" ? "yes" : "no"}
        ></Select>
      );
    } else if (type === "textarea") {
      return (
        <div style={{ width: "45.8em" }}>
          <TextArea
            name={key}
            cols={40}
            rows={2}
            value={value ?? ""}
            onChange={(e) => onQuerySettingsChange(key)(e.currentTarget.value)}
            label={key}
            aria-label={key}
          />
        </div>
      );
    } else {
      return (
        <Input
          name={key}
          width={80}
          value={value ?? ""}
          //type={querySettingDefinitions[key].type}
          min={querySettingDefinitions[key].min}
          onChange={(e) => onQuerySettingsChange(key)(e.currentTarget.value)}
          label={key}
          aria-label={key}
        />
      );
    }
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
            data-testid={labels.credentialsType.testId}
            label={labels.credentialsType.label}
            description={labels.credentialsType.description}
          >
            <RadioButtonGroup<CredentialsType>
              options={credentialsTypesOptions}
              disabledOptions={[]}
              value={jsonData.credentialsType ?? CredentialsType.UserAccount}
              onChange={(e) => onCredentialsTypeToggle(e!)}
            />
          </Field>
          {jsonData.credentialsType !== CredentialsType.ServiceAccount && (
            <>
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
            </>
          )}
          {jsonData.credentialsType === CredentialsType.ServiceAccount && (
            <>
              <Field
                data-testid={labels.token.testId}
                label={labels.token.label}
                description={labels.token.description}
              >
                <SecretInput
                  name={"token"}
                  width={40}
                  label={labels.token.label}
                  aria-label={labels.token.label}
                  placeholder={labels.token.placeholder}
                  value={secureJsonData.token || ""}
                  isConfigured={
                    (secureJsonFields && secureJsonFields.token) as boolean
                  }
                  onReset={onResetToken}
                  onChange={onUpdateDatasourceSecureJsonDataOption(
                    props,
                    "token"
                  )}
                />
              </Field>
            </>
          )}
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
            data-testid={labels.adHocConditionVariable.testId}
            label={labels.adHocConditionVariable.label}
            description={labels.adHocConditionVariable.description}
          >
            <Input
              name={"adHocTableVariable"}
              width={40}
              value={jsonData.adHocConditionVariable || ""}
              onChange={onUpdateDatasourceJsonDataOption(
                props,
                "adHocConditionVariable"
              )}
              label={labels.adHocConditionVariable.label}
              aria-label={labels.adHocConditionVariable.label}
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
          <Divider />
          <ConfigSection title="Query Settings">
            <div style={{ marginBottom: "20px" }}>
              <Select
                options={queryOptions}
                onChange={(v) => addQuerySetting(v.value ?? "")}
                value={null}
              ></Select>
            </div>
            {(jsonData?.querySettings ?? []).map((s) => {
              return (
                <Field
                  key={s.setting}
                  label={s.setting}
                  required
                  error={settingErrors[s.setting]}
                  invalid={!!settingErrors[s.setting]}
                  description={querySettingDefinitions[s.setting].description}
                >
                  <Stack direction={"row"}>
                    {settingInput(s.setting, s.value)}
                    <Button
                      style={{ marginTop: "4.5px" }}
                      variant="destructive"
                      icon="times"
                      size={"sm"}
                      onClick={() => deleteQuerySetting(s.setting)}
                    />
                  </Stack>
                </Field>
              );
            })}
          </ConfigSection>
        </ConfigSection>
      </div>
    </>
  );
}
