import React, {FormEvent, useEffect, useMemo, useState} from "react";
import {
  DataSourcePluginOptionsEditorProps,
  onUpdateDatasourceJsonDataOption,
  onUpdateDatasourceSecureJsonDataOption,
} from "@grafana/data";
import {
  Alert,
  Button,
  Divider,
  Field,
  IconButton,
  InlineField,
  InlineSwitch,
  Input,
  RadioButtonGroup,
  SecretInput,
  Select,
  Stack,
  Switch,
  TextArea,
} from "@grafana/ui";
import { ConfigSection } from "@grafana/plugin-ui";
import {
  CredentialsType,
  HdxDataSourceOptions,
  HdxSecureJsonData,
  Protocol,
  QuerySetting,
} from "../types";
import allLabels from "labels";
import defaultConfigs from "defaultConfigs";
import {
  parseLookbackSeconds,
  QUERY_DURATION_REGEX,
} from "../editor/timeRangeUtils";
import { getDefaultValue, isBooleanSettingOn } from "../editor/editorUtils";
import { SOLUTION_TEMPLATES } from "../errors/solutionTemplates";

export interface Props
  extends DataSourcePluginOptionsEditorProps<
    HdxDataSourceOptions,
    HdxSecureJsonData
  > {}

type QuerySettingDefinition =
  (typeof allLabels.components.querySettings.values)[number];

const isQuerySetting = (entry: unknown): entry is QuerySetting =>
  typeof entry === "object" && entry !== null;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields } = options;

  // Apply defaults via effect (not inline) — Grafana 13 / React 18 drops
  // setState during render. Detect a fresh datasource by checking the domain
  // fields rather than `Object.keys(jsonData).length`, since Grafana 13 seeds
  // jsonData with `{ pdcInjected: false }`.
  useEffect(() => {
    const isUninitialized =
      options.jsonData.protocol === undefined &&
      options.jsonData.port === undefined &&
      options.jsonData.host === undefined;
    if (isUninitialized) {
      onOptionsChange({
        ...options,
        jsonData: { ...options.jsonData, ...defaultConfigs },
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const labels = allLabels.components.config.editor;
  const querySettings = allLabels.components.querySettings;
  const secureJsonData = (options.secureJsonData || {}) as HdxSecureJsonData;
  const protocolOptions = [
    { label: "Native", value: Protocol.Native },
    { label: "HTTP", value: Protocol.Http },
  ];
  const credentialsTypesOptions = [
    { label: "User Account", value: CredentialsType.UserAccount },
    { label: "Service Account", value: CredentialsType.ServiceAccount },
    { label: "Forward OAuth Identity", value: CredentialsType.ForwardOAuth },
    {
      label: "Forward OAuth + Exchange",
      value: CredentialsType.ForwardOAuthExchange,
    },
  ];
  // A Map, not an object: a stored name such as "constructor" must not
  // resolve to an Object.prototype member.
  let querySettingDefinitions = useMemo(
    () =>
      new Map<string, QuerySettingDefinition>(
        querySettings.values.map((v) => [v.setting, v])
      ),
    [querySettings]
  );

  // jsonData comes from provisioning or the HTTP API as well as this editor,
  // so it can hold anything: a non-array, or entries that are not objects.
  // The backend rejects the data source for those and they cannot be shown
  // as a row, so the first edit of any row writes the list without them.
  const rawQuerySettings: unknown = jsonData?.querySettings;
  const storedSettings = useMemo<QuerySetting[]>(
    () =>
      Array.isArray(rawQuerySettings)
        ? rawQuerySettings.filter(isQuerySetting)
        : [],
    [rawQuerySettings]
  );
  const writeQuerySettings = (querySettings: QuerySetting[]) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        querySettings,
      },
    });
  };

  const existingSettings = storedSettings.map((v) => v.setting);
  let queryOptions = querySettings.values
    .filter((v) => !existingSettings.includes(v.setting))
    .map((v) => ({
      label: v.setting,
      value: v.setting,
      description: v.description,
    }));

  const settingErrors = useMemo(() => {
    // Indexed like storedSettings, so two entries with one name keep
    // separate errors.
    const errors: Array<string | undefined> = [];
    for (const [index, querySettings] of storedSettings.entries()) {
      const option = querySettingDefinitions.get(querySettings.setting);

      const type = option?.type;
      const min = option?.min;
      if (
        type &&
        ["number", "text", "duration"].includes(type) &&
        (querySettings.value === "" || querySettings.value === undefined)
      ) {
        errors[index] = "setting value is required";
      } else if (
        type &&
        type === "number" &&
        isNaN(querySettings.value as any)
      ) {
        errors[index] = `value is not a valid number`;
      } else if (
        type &&
        type === "duration" &&
        isNaN(querySettings.value as any) &&
        !QUERY_DURATION_REGEX.test(querySettings.value)
      ) {
        errors[index] = `value is not a valid duration`;
      } else if (
        type &&
        type === "number" &&
        min !== undefined &&
        min > +querySettings.value &&
        +querySettings.value !== option.default
      ) {
        errors[index] = `value cannot be less than ${min}`;
      }
    }
    return errors;
  }, [storedSettings, querySettingDefinitions]);

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
        // Grafana forwards the sign-in token only when asked. The exchanging
        // mode then swaps it for a cluster token before querying, so it needs
        // the forward just as much as plain forwarding does.
        oauthPassThru:
          credentialsType === CredentialsType.ForwardOAuth ||
          credentialsType === CredentialsType.ForwardOAuthExchange,
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

  // Entries are addressed by position: names are not unique in a
  // provisioned data source.
  const onQuerySettingsChange = (index: number) => {
    return (value: string) => {
      writeQuerySettings(
        storedSettings.map((s, i) => (i === index ? { ...s, value } : s))
      );
    };
  };

  const addQuerySetting = (setting: string) => {
    const definition = querySettingDefinitions.get(setting);
    writeQuerySettings([
      ...storedSettings,
      {
        setting,
        value: getDefaultValue(definition?.default, definition?.type ?? ""),
      },
    ]);
  };

  const deleteQuerySetting = (index: number) => {
    writeQuerySettings(storedSettings.filter((_, i) => i !== index));
  };

  // A setting left empty has nothing to send, so its row goes away.
  const onQuerySettingBlur =
    (index: number) =>
    (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      if (e.currentTarget.value === "") {
        deleteQuerySetting(index);
      }
    };

  const downloadSolutionTemplates = () => {
    const dataStr = JSON.stringify(
      SOLUTION_TEMPLATES.map((t) => ({ name: t.name, template: t.template })),
      null,
      2
    );
    const dataBlob = new Blob([dataStr], { type: "application/json" });
    const url = URL.createObjectURL(dataBlob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "solution_templates.json";
    link.click();
    URL.revokeObjectURL(url);
  };

  const [invalidDuration, setInvalidDuration] = useState(false);
  const onRoundChange = (e: FormEvent<HTMLInputElement>) => {
    let round = e.currentTarget.value;

    setInvalidDuration(!QUERY_DURATION_REGEX.test(round));
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        defaultRound: round,
      },
    });
  };
  const onRoundBlur = () => {
    if (invalidDuration) {
      setInvalidDuration(false);
      onOptionsChange({
        ...options,
        jsonData: {
          ...options.jsonData,
          defaultRound: "",
        },
      });
    }
  };
  // Validity comes from the same parser the datasource resolves with, so blur
  // only clears values the runtime would ignore anyway. The advertised s/m/h
  // units are guidance: a stored value in another accepted unit (e.g. `1d`)
  // must survive someone merely viewing this page.
  const isInvalidLookback = (value: unknown) => {
    // Unset or blank means "use the default", which is valid.
    if (value === undefined || value === null) {
      return false;
    }
    if (typeof value === "string" && !value.trim()) {
      return false;
    }
    // Anything else, including a non-string provisioned from YAML, is up to
    // the shared parser.
    return parseLookbackSeconds(value) === undefined;
  };
  const [invalidLookback, setInvalidLookback] = useState(() =>
    isInvalidLookback(jsonData.adHocTimeRangeLookback)
  );
  const onLookbackChange = (e: FormEvent<HTMLInputElement>) => {
    const lookback = e.currentTarget.value;
    setInvalidLookback(isInvalidLookback(lookback));
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        adHocTimeRangeLookback: lookback,
      },
    });
  };
  const onLookbackBlur = () => {
    if (invalidLookback) {
      setInvalidLookback(false);
      onOptionsChange({
        ...options,
        jsonData: {
          ...options.jsonData,
          adHocTimeRangeLookback: "",
        },
      });
    }
  };
  const settingInput = (key: string, value: string, index: number) => {
    const definition = querySettingDefinitions.get(key);
    let type = definition?.type;
    if (type === "boolean") {
      return (
        <Select
          width={80}
          options={[
            { value: "yes", label: "Yes" },
            { value: "no", label: "No" },
          ]}
          onChange={(v) =>
            onQuerySettingsChange(index)(v.value === "yes" ? "1" : "0")
          }
          value={isBooleanSettingOn(value) ? "yes" : "no"}
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
            onChange={(e) => onQuerySettingsChange(index)(e.currentTarget.value)}
            onBlur={onQuerySettingBlur(index)}
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
          min={definition?.min}
          onChange={(e) => onQuerySettingsChange(index)(e.currentTarget.value)}
          onBlur={onQuerySettingBlur(index)}
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
          {jsonData.protocol === Protocol.Http &&
            !jsonData.secure &&
            secureJsonFields.password && (
              <Alert title={labels.secure.alertTitle} severity={"warning"}>
                {labels.secure.alertMessage}
              </Alert>
            )}
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
          {(!jsonData.credentialsType ||
            jsonData.credentialsType === CredentialsType.UserAccount) && (
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
            invalid={invalidDuration}
          >
            <Input
              width={40}
              onChange={onRoundChange}
              onBlur={onRoundBlur}
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
            data-testid={labels.adHocTimeRangeLookback.testId}
            error={"invalid duration"}
            label={labels.adHocTimeRangeLookback.label}
            description={labels.adHocTimeRangeLookback.description}
            invalid={invalidLookback}
          >
            <Input
              name={"adHocTimeRangeLookback"}
              width={40}
              value={jsonData.adHocTimeRangeLookback || ""}
              onChange={onLookbackChange}
              onBlur={onLookbackBlur}
              label={labels.adHocTimeRangeLookback.label}
              aria-label={labels.adHocTimeRangeLookback.label}
              placeholder={labels.adHocTimeRangeLookback.placeholder}
            />
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
          <ConfigSection title="Error Exposure">
            <Field
              data-testid={labels.exposeErrorsEnabled.testId}
              label={labels.exposeErrorsEnabled.label}
              description={labels.exposeErrorsEnabled.description}
            >
              <Switch
                id="exposeErrorsEnabled"
                className="gf-form"
                value={jsonData.exposeErrors?.enabled ?? false}
                onChange={(e) => {
                  onOptionsChange({
                    ...options,
                    jsonData: {
                      ...jsonData,
                      exposeErrors: {
                        ...jsonData.exposeErrors,
                        enabled: e.currentTarget.checked,
                      },
                    },
                  });
                }}
              />
            </Field>
            {jsonData.exposeErrors?.enabled && (
              <>
                <Field
                  data-testid={labels.exposeErrorsVariableName.testId}
                  label={labels.exposeErrorsVariableName.label}
                  description={labels.exposeErrorsVariableName.description}
                >
                  <Input
                    name={"exposeErrorsVariableName"}
                    width={40}
                    value={jsonData.exposeErrors?.variableName || ""}
                    onChange={(e) => {
                      onOptionsChange({
                        ...options,
                        jsonData: {
                          ...jsonData,
                          exposeErrors: {
                            ...jsonData.exposeErrors,
                            variableName: e.currentTarget.value,
                          },
                        },
                      });
                    }}
                    label={labels.exposeErrorsVariableName.label}
                    aria-label={labels.exposeErrorsVariableName.label}
                    placeholder={labels.exposeErrorsVariableName.placeholder}
                  />
                </Field>
                <Field
                  data-testid={labels.exposeErrorsMaxCount.testId}
                  label={labels.exposeErrorsMaxCount.label}
                  description={labels.exposeErrorsMaxCount.description}
                >
                  <Input
                    name={"exposeErrorsMaxCount"}
                    width={40}
                    value={jsonData.exposeErrors?.maxCount || ""}
                    onChange={(e) => {
                      onOptionsChange({
                        ...options,
                        jsonData: {
                          ...jsonData,
                          exposeErrors: {
                            ...jsonData.exposeErrors,
                            maxCount: +e.currentTarget.value,
                          },
                        },
                      });
                    }}
                    label={labels.exposeErrorsMaxCount.label}
                    aria-label={labels.exposeErrorsMaxCount.label}
                    placeholder={labels.exposeErrorsMaxCount.placeholder}
                    type="number"
                  />
                </Field>
                <Field
                  data-testid={labels.exposeErrorsTtl.testId}
                  label={labels.exposeErrorsTtl.label}
                  description={labels.exposeErrorsTtl.description}
                >
                  <Input
                    name={"exposeErrorsTtl"}
                    width={40}
                    value={jsonData.exposeErrors?.ttl || ""}
                    onChange={(e) => {
                      onOptionsChange({
                        ...options,
                        jsonData: {
                          ...jsonData,
                          exposeErrors: {
                            ...jsonData.exposeErrors,
                            ttl: +e.currentTarget.value,
                          },
                        },
                      });
                    }}
                    label={labels.exposeErrorsTtl.label}
                    aria-label={labels.exposeErrorsTtl.label}
                    placeholder={labels.exposeErrorsTtl.placeholder}
                    type="number"
                  />
                </Field>
                <Field
                  label="Solution Templates"
                  description="Download the error solution templates in JSON format"
                >
                  <IconButton
                    variant="secondary"
                    name="download-alt"
                    onClick={downloadSolutionTemplates}
                    aria-labelledby=""
                    size={"xl"}
                  >
                    Download Solution Templates
                  </IconButton>
                </Field>
              </>
            )}
          </ConfigSection>
          <Divider />
          <ConfigSection title="Query Settings">
            <div style={{ marginBottom: "20px" }}>
              <Select
                options={queryOptions}
                onChange={(v) => addQuerySetting(v.value ?? "")}
                value={null}
              ></Select>
            </div>
            {storedSettings.map((s, index) => {
              const definition = querySettingDefinitions.get(s.setting);
              return (
                <Field
                  key={`${index}-${s.setting}`}
                  label={s.setting}
                  required={!!definition}
                  error={settingErrors[index]}
                  invalid={!!settingErrors[index]}
                  description={
                    definition?.description ??
                    querySettings.unknownSettingWarning
                  }
                >
                  <Stack direction={"row"}>
                    {settingInput(s.setting, s.value, index)}
                    <Button
                      aria-label={querySettings.removeLabel(s.setting)}
                      style={{ marginTop: "4.5px" }}
                      variant="destructive"
                      icon="times"
                      size={"sm"}
                      onClick={() => deleteQuerySetting(index)}
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
