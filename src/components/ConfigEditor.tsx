import React from 'react';
import {
    DataSourcePluginOptionsEditorProps,
    onUpdateDatasourceJsonDataOption,
    onUpdateDatasourceSecureJsonDataOption
} from '@grafana/data';
import {Divider, Field, Input, RadioButtonGroup, SecretInput, Switch} from '@grafana/ui';
import {ConfigSection, DataSourceDescription} from '@grafana/plugin-ui';
import {MyDataSourceOptions, MySecureJsonData, Protocol} from '../types';
import allLabels from 'labels';

export interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {
}

export function ConfigEditor(props: Props) {
    const {onOptionsChange, options} = props;
    const {jsonData, secureJsonFields} = options;
    const labels = allLabels.components.config.editor;
    const secureJsonData = (options.secureJsonData || {}) as MySecureJsonData;
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
        key: keyof Pick<MyDataSourceOptions, "secureConnection">,
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
        key: keyof Pick<MyDataSourceOptions, "skipTlsVerify">,
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

    const defaultPort = jsonData.secureConnection ?
        (jsonData.protocol === Protocol.Native ? labels.port.secureNativePort : labels.port.secureHttpPort) :
        (jsonData.protocol === Protocol.Native ? labels.port.insecureNativePort : labels.port.insecureHttpPort);
    const portDescription = `${labels.port.description} (default for ${jsonData.secureConnection ? "secure" : ""} ${jsonData.protocol}: ${defaultPort})`


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
                        value={jsonData.secureConnection || false}
                        onChange={(e) => onSwitchToggle("secureConnection", e.currentTarget.checked)}
                    />
                </Field>
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
                    description={labels.username.tooltip}
                >
                    <Input
                        name="username"
                        width={40}
                        value={jsonData.username || ""}
                        onChange={onUpdateDatasourceJsonDataOption(props, "username")}
                        label={labels.username.label}
                        aria-label={labels.username.label}
                        placeholder={labels.username.description}
                    />
                </Field>
                <Field label={labels.password.label} description={labels.password.description}>
                    <SecretInput
                        name="password"
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
        </>
    );
}
