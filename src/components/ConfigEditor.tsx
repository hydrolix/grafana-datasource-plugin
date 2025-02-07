import React, { ChangeEvent } from 'react';
import {InlineField, InlineSwitch, Input, Legend, SecretInput, Stack} from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

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
        hostname: event.target.value,
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

  const onUseDefaultPortChange = (event: ChangeEvent<HTMLInputElement>) => {
    console.log("onUseDefaultPortChange", event.target.value)
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        port: 8443,
        useDefaultPort: !jsonData.useDefaultPort
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

  const labelWidth = 12;
  let defaultInputWidth = 47;
  return (
    <>
      <Legend>Server</Legend>
      <InlineField label="Host" labelWidth={labelWidth} interactive tooltip={'Server hostname'}>
        <Input
            id="config-editor-host"
            onChange={onHostChange}
            value={jsonData.hostname}
            placeholder="https://my.hydrolix.domain.com"
            width={defaultInputWidth}
        />
      </InlineField>
      <InlineField label="Port" labelWidth={labelWidth} interactive tooltip={'Server port number (default: 8443)'}>

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
              onChange={onUseDefaultPortChange}
              value={jsonData.useDefaultPort}/>
        </Stack>
      </InlineField>
      <Legend>Credentials</Legend>
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
