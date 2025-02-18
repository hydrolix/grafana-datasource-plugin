import React from 'react';
import {
    DataSourcePluginOptionsEditorProps,
    onUpdateDatasourceJsonDataOption,
    onUpdateDatasourceSecureJsonDataOption
} from '@grafana/data';
import {Divider, Field, Input, RadioButtonGroup, SecretInput, Switch} from '@grafana/ui';
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

    const onPortChange = (port: string) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...options.jsonData,
                port: +port,
            },
        });
    };
    const onProtocolToggle = (protocol: Protocol) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...options.jsonData,
                protocol: protocol,
            },
        });
    };
    const onSwitchToggle = (
        key: keyof Pick<HdxDataSourceOptions, "secure">,
        value: boolean
    ) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...options.jsonData,
                [key]: value,
            },
        });
    }
    const onTlsSettingsChange = (
        key: keyof Pick<HdxDataSourceOptions, "skipTlsVerify">,
        value: boolean
    ) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...options.jsonData,
                [key]: value,
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

    const defaultPort = jsonData.secure ?
        (jsonData.protocol === Protocol.Native ? labels.port.secureNativePort : labels.port.secureHttpPort) :
        (jsonData.protocol === Protocol.Native ? labels.port.insecureNativePort : labels.port.insecureHttpPort);
    const portDescription = `${labels.port.description} (default for ${jsonData.secure ? "secure" : ""} ${jsonData.protocol}: ${defaultPort})`


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
                    <Input
                        name="port"
                        width={40}
                        type="number"
                        value={jsonData.port || ''}
                        onChange={e => onPortChange(e.currentTarget.value)}
                        label={labels.port.label}
                        aria-label={labels.port.label}
                        placeholder={defaultPort}
                    />
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
                        onChange={(e) => onSwitchToggle("secure", e.currentTarget.checked)}
                    />
                </Field>

                { jsonData.protocol === Protocol.Http &&
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

            <Divider/>

            <ConfigSection title="TLS / SSL Settings">
                <Field
                    label={labels.skipTlsVerify.label}
                    description={labels.skipTlsVerify.description}
                >
                    <Switch
                        className="gf-form"
                        value={jsonData.skipTlsVerify || false}
                        onChange={(e) => onTlsSettingsChange("skipTlsVerify", e.currentTarget.checked)}
                    />
                </Field>
            </ConfigSection>

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

            <ConfigSection title={"Default Database"}>
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
            </ConfigSection>

            <Divider/>

            <ConfigSection title={"Query Settings"}>
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
