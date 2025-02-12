import React, {ChangeEvent} from 'react';
import {InlineField, InlineSwitch, Input, Legend, RadioButtonGroup, SecretInput, Stack} from '@grafana/ui';
import {DataSourcePluginOptionsEditorProps} from '@grafana/data';
import {MyDataSourceOptions, MySecureJsonData, Protocol} from '../types';

export interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {
}

export function ConfigEditor(props: Props) {
    const {onOptionsChange, options} = props;
    const {jsonData, secureJsonFields, secureJsonData} = options;
    const protocolOptions = [
        {label: 'Native', value: Protocol.Native},
        {label: 'HTTP', value: Protocol.Http},
    ];

    const getDefaultPort = (protocol: Protocol, secureConnection: boolean) => {
        if (protocol === Protocol.Native && secureConnection) {
            return 9440;
        } else if (protocol === Protocol.Native && !secureConnection) {
            return 9000;
        } else if (protocol === Protocol.Http && secureConnection) {
            return 8443;
        } else {
            return 8123;
        }
    }

    const onUserNameChange = (event: ChangeEvent<HTMLInputElement>) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...jsonData,
                username: event.target.value,
            },
        });
    };
    const onHostChange = (event: ChangeEvent<HTMLInputElement>) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...jsonData,
                host: event.target.value,
            },
        });
    };
    const onPortChange = (event: ChangeEvent<HTMLInputElement>) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...jsonData,
                port: +event.target.value,
            },
        });
    };

    const onUseDefaultPortToggle = () => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...jsonData,
                port: getDefaultPort(jsonData.protocol!, jsonData.secureConnection!),
                useDefaultPort: !jsonData.useDefaultPort
            },
        });
    };
    const onSecureConnectionToggle = () => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...jsonData,
                port: jsonData.useDefaultPort ? getDefaultPort(jsonData.protocol!, !jsonData.secureConnection) : jsonData.port,
                secureConnection: !jsonData.secureConnection
            },
        });
    };    const onSkipVerifyToggle = () => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...jsonData,
                skipTlsVerify: !jsonData.skipTlsVerify
            },
        });
    };

    const onProtocolToggle = (protocol: Protocol) => {
        onOptionsChange({
            ...options,
            jsonData: {
                ...options.jsonData,
                protocol: protocol,
                port: jsonData.useDefaultPort ? getDefaultPort(protocol, jsonData.secureConnection!) : jsonData.port
            },
        });
    };

    // Secure field (only sent to the backend)
    const onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
        onOptionsChange({
            ...options,
            secureJsonData: {
                password: event.target.value,
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
    const legendStyle = {
        marginTop: 16,
        marginBottom:0
    }

    const labelWidth = 20;
    const defaultInputWidth = 47;
    return (
        <>
            <Legend style={legendStyle}>Server</Legend>
            <InlineField label="Host" labelWidth={labelWidth} interactive tooltip={'Server host'}>
                <Input
                    id="config-editor-host"
                    onChange={onHostChange}
                    value={jsonData.host}
                    placeholder="https://my.hydrolix.domain.com"
                    width={defaultInputWidth}
                />
            </InlineField>
            <InlineField label="Port" labelWidth={labelWidth} interactive
                         tooltip={'Server port number'}>

                <Stack direction="row">
                    <Input
                        type="number"
                        id="config-editor-port"
                        min={1}
                        max={65535}
                        onChange={onPortChange}
                        value={jsonData.port}
                        placeholder="9440"
                        disabled={jsonData.useDefaultPort}
                        width={29}
                    />
                    <InlineSwitch
                        label="Use default"
                        showLabel={true}
                        onChange={onUseDefaultPortToggle}
                        value={jsonData.useDefaultPort}/>
                </Stack>
            </InlineField>

            <InlineField label="Protocol" labelWidth={labelWidth} tooltip={'Connection protocol'}>
                <RadioButtonGroup<Protocol>
                    fullWidth={true}
                    options={protocolOptions}
                    disabledOptions={[]}
                    value={jsonData.protocol || Protocol.Native}
                    onChange={(e) => onProtocolToggle(e!)}

                />
            </InlineField>

            <InlineField label="Secure Connection" labelWidth={labelWidth} tooltip={'Use encrypted connection'}>
                <Stack direction="row">
                    <InlineSwitch
                        width={35}
                        label="Use TLS"
                        showLabel={true}
                        onChange={onSecureConnectionToggle}
                        value={jsonData.secureConnection}/>
                    <InlineSwitch
                        width={35}
                        label="Skip TLS Check"
                        showLabel={true}
                        onChange={onSkipVerifyToggle}
                        value={jsonData.skipTlsVerify}/>
                </Stack>
            </InlineField>
            <Legend style={legendStyle}>Credentials</Legend>
            <InlineField label="Username" labelWidth={labelWidth} interactive tooltip={'username to access the system'}>
                <Input
                    id="config-editor-username"
                    onChange={onUserNameChange}
                    value={jsonData.username}
                    placeholder="username"
                    width={defaultInputWidth}
                />
            </InlineField>

            <InlineField label="Password" labelWidth={labelWidth} interactive tooltip={'password to access the system'}>
                <SecretInput
                    required
                    id="config-editor-password"
                    isConfigured={secureJsonFields.password}
                    value={secureJsonData?.password}
                    placeholder="Enter your API key"
                    width={defaultInputWidth}
                    onReset={onResetPassword}
                    onChange={onPasswordChange}
                />
            </InlineField>
        </>
    );
}
