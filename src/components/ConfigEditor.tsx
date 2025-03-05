import React from 'react';
import {
    DataSourcePluginOptionsEditorProps,
    onUpdateDatasourceJsonDataOption,
    onUpdateDatasourceSecureJsonDataOption
} from '@grafana/data';
import {
    Divider,
    Field,
    InlineField,
    InlineSwitch,
    Input,
    RadioButtonGroup,
    SecretInput,
    Stack,
    Switch
} from '@grafana/ui';
import {ConfigSection, DataSourceDescription} from '@grafana/plugin-ui';
import {HdxDataSourceOptions, HdxSecureJsonData, Protocol} from '../types';
import allLabels from 'labels';

export interface Props extends DataSourcePluginOptionsEditorProps<HdxDataSourceOptions, HdxSecureJsonData> {
}

export function ConfigEditor(props: Props) {
    const {onOptionsChange, options} = props;
    const {jsonData, secureJsonFields} = options;

    const labels = allLabels.components.config.editor;
    const secureJsonData = (options.secureJsonData || {}) as HdxSecureJsonData;
    const protocolOptions = [
        {label: "Native", value: Protocol.Native},
        {label: "HTTP", value: Protocol.Http},
    ];

    const getDefaultPort = (protocol: Protocol, secure: boolean) =>
        secure ?
            (protocol === Protocol.Native ? labels.port.secureNativePort : labels.port.secureHttpPort) :
            (protocol === Protocol.Native ? labels.port.insecureNativePort : labels.port.insecureHttpPort);

    const defaultPort = getDefaultPort(jsonData.protocol!, jsonData.secure!);
    const portDescription = `${labels.port.description} (default for ${jsonData.secure ? "secure" : ""} ${jsonData.protocol}: ${defaultPort})`


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
                port: +defaultPort
            },
        });
    };
    const onProtocolToggle = (protocol: Protocol) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...options.jsonData,
                protocol: protocol,
                port: jsonData.useDefaultPort ? +getDefaultPort(protocol, jsonData.secure!) : jsonData.port
            },
        });
    };
    const onSecureChange = (
        value: boolean
    ) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...options.jsonData,
                secure: value,
                port: jsonData.useDefaultPort ? +getDefaultPort(jsonData.protocol!, value) : jsonData.port
            },
        });
    };
    const onTlsSettingsChange = (
        value: boolean
    ) => {
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
                password: '',
            },
        });
    };


    return (
        <>
            <DataSourceDescription
                dataSourceName="Hydrolix"
                docsLink="https://hydrolix.io"
                hasRequiredFields
            />
            <Divider/>
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
                            value={jsonData.port || ''}
                            disabled={jsonData.useDefaultPort}
                            onChange={e => onPortChange(e.currentTarget.value)}
                            label={labels.port.label}
                            aria-label={labels.port.label}
                        />
                        <InlineField label={labels.useDefaultPort.label} interactive>

                            <InlineSwitch
                                onChange={e => onUseDefaultPortChange(e.currentTarget.checked)}
                                value={jsonData.useDefaultPort}/>
                        </InlineField>
                    </Stack>

                </Field>

                <Field label={labels.protocol.label} description={labels.protocol.description}>
                    <RadioButtonGroup<Protocol>
                        options={protocolOptions}
                        disabledOptions={[]}
                        value={jsonData.protocol || Protocol.Native}
                        onChange={(e) => onProtocolToggle(e!)}
                    />
                </Field>

                <Field label={labels.secure.label} description={labels.secure.description}>
                    <Switch
                        id="secure"
                        className="gf-form"
                        value={jsonData.secure || false}
                        onChange={(e) => onSecureChange(e.currentTarget.checked)}
                    />
                </Field>

                {jsonData.protocol === Protocol.Http &&
                    <Field label={labels.path.label} description={labels.path.description}>
                        <Input
                            value={jsonData.path || ''}
                            name="path"
                            width={80}
                            onChange={onUpdateDatasourceJsonDataOption(props, "path")}
                            label={labels.path.label}
                            aria-label={labels.path.label}
                            placeholder={labels.path.placeholder}
                        />
                    </Field>
                }
            </ConfigSection>


            {jsonData.secure &&
                <>
                    <Divider/>
                    <ConfigSection title="TLS / SSL Settings">
                        <Field
                            label={labels.skipTlsVerify.label}
                            description={labels.skipTlsVerify.description}
                        >
                            <Switch
                                className="gf-form"
                                value={jsonData.skipTlsVerify || false}
                                onChange={(e) => onTlsSettingsChange(e.currentTarget.checked)}
                            />
                        </Field>
                    </ConfigSection>
                </>
            }

            <Divider/>

            <ConfigSection title="Credentials">
                <Field
                    label={labels.username.label}
                    description={labels.username.description}
                >
                    <Input
                        name={"username"}
                        width={40}
                        value={jsonData.username || ""}
                        onChange={onUpdateDatasourceJsonDataOption(props, "username")}
                        label={labels.username.label}
                        aria-label={labels.username.label}
                        placeholder={labels.username.placeholder}
                    />
                </Field>
                <Field label={labels.password.label} description={labels.password.description}>
                    <SecretInput
                        name={"password"}
                        width={40}
                        label={labels.password.label}
                        aria-label={labels.password.label}
                        placeholder={labels.password.placeholder}
                        value={secureJsonData.password || ''}
                        isConfigured={(secureJsonFields && secureJsonFields.password) as boolean}
                        onReset={onResetPassword}
                        onChange={onUpdateDatasourceSecureJsonDataOption(props, "password")}
                    />
                </Field>
            </ConfigSection>
            <Divider/>
            <ConfigSection title="Additional Settings" isCollapsible isInitiallyOpen={false}>
                <Field label={labels.defaultDatabase.label} description={labels.defaultDatabase.description}>
                    <Input
                        name={"defaultDatabase"}
                        width={40}
                        value={jsonData.defaultDatabase || ""}
                        onChange={onUpdateDatasourceJsonDataOption(props, "defaultDatabase")}
                        label={labels.defaultDatabase.label}
                        aria-label={labels.defaultDatabase.label}
                        placeholder={labels.defaultDatabase.placeholder}
                        type="number"
                    />
                </Field>

                <Field label={labels.dialTimeout.label} description={labels.dialTimeout.description}>
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
                <Field label={labels.queryTimeout.label} description={labels.queryTimeout.description}>
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
