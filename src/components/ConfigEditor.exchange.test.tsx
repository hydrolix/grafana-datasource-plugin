import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { ConfigEditor, Props } from './ConfigEditor';
import '@testing-library/jest-dom';
import fs from 'fs';
import { CredentialsType, HdxDataSourceOptions } from 'types';
import allLabels from 'labels';

const pluginJson = JSON.parse(fs.readFileSync('./src/plugin.json', 'utf-8'));

jest.mock('@grafana/runtime', () => {
  const original = jest.requireActual('@grafana/runtime');
  return {
    ...original,
    config: {
      buildInfo: { version: '10.0.0' },
      secureSocksDSProxyEnabled: true,
    },
  };
});

function propsWith(overrides: HdxDataSourceOptions, onOptionsChange = jest.fn()) {
  return {
    ...pluginJson,
    options: {
      jsonData: {
        host: 'cluster.example.hydrolix.net',
        port: 443,
        protocol: 'http',
        secure: true,
        useDefaultPort: false,
        ...overrides,
      },
      secureJsonData: {},
      secureJsonFields: {},
    },
    onOptionsChange,
  } as Props;
}

describe('ConfigEditor: forward OAuth with a console exchange', () => {
  const labels = allLabels.components.config.editor;

  it('offers the mode', () => {
    render(<ConfigEditor {...propsWith({})} />);
    expect(screen.getByLabelText('Forward OAuth + Exchange')).toBeInTheDocument();
  });

  it('turns on oauthPassThru when the mode is chosen, because Grafana only forwards a token when asked', () => {
    const onOptionsChange = jest.fn();
    render(<ConfigEditor {...propsWith({}, onOptionsChange)} />);

    fireEvent.click(screen.getByLabelText('Forward OAuth + Exchange'));

    expect(onOptionsChange).toHaveBeenCalled();
    const sent = onOptionsChange.mock.calls.at(-1)![0];
    expect(sent.jsonData.credentialsType).toBe(CredentialsType.ForwardOAuthExchange);
    expect(sent.jsonData.oauthPassThru).toBe(true);
  });

  it('keeps plain Forward OAuth Identity working as before', () => {
    const onOptionsChange = jest.fn();
    render(<ConfigEditor {...propsWith({}, onOptionsChange)} />);

    fireEvent.click(screen.getByLabelText('Forward OAuth Identity'));

    const sent = onOptionsChange.mock.calls.at(-1)![0];
    expect(sent.jsonData.credentialsType).toBe(CredentialsType.ForwardOAuth);
    expect(sent.jsonData.oauthPassThru).toBe(true);
  });

  it('turns oauthPassThru off again for a mode that forwards nothing', () => {
    const onOptionsChange = jest.fn();
    render(<ConfigEditor {...propsWith({ credentialsType: CredentialsType.ForwardOAuthExchange }, onOptionsChange)} />);

    fireEvent.click(screen.getByLabelText('Service Account'));

    const sent = onOptionsChange.mock.calls.at(-1)![0];
    expect(sent.jsonData.credentialsType).toBe(CredentialsType.ServiceAccount);
    expect(sent.jsonData.oauthPassThru).toBe(false);
  });

  it('asks for a cluster audience only in the exchanging mode', () => {
    const { unmount } = render(
      <ConfigEditor
        {...propsWith({
          credentialsType: CredentialsType.ForwardOAuthExchange,
        })}
      />
    );
    expect(screen.getByLabelText(labels.exchangeAudience.label)).toBeInTheDocument();
    unmount();

    render(<ConfigEditor {...propsWith({ credentialsType: CredentialsType.ForwardOAuth })} />);
    expect(screen.queryByLabelText(labels.exchangeAudience.label)).not.toBeInTheDocument();
  });

  it('stores the audience in jsonData, where nothing secret belongs', () => {
    const onOptionsChange = jest.fn();
    render(<ConfigEditor {...propsWith({ credentialsType: CredentialsType.ForwardOAuthExchange }, onOptionsChange)} />);

    fireEvent.change(screen.getByLabelText(labels.exchangeAudience.label), {
      target: { value: 'other.example.hydrolix.net' },
    });

    const sent = onOptionsChange.mock.calls.at(-1)![0];
    expect(sent.jsonData.exchangeAudience).toBe('other.example.hydrolix.net');
    expect(sent.secureJsonData ?? {}).toEqual({});
  });

  it("asks for no credential at all: the delegate lives in Grafana's server configuration", () => {
    render(
      <ConfigEditor
        {...propsWith({
          credentialsType: CredentialsType.ForwardOAuthExchange,
        })}
      />
    );
    expect(screen.queryByLabelText(labels.token.label)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(labels.password.label)).not.toBeInTheDocument();
  });
});
