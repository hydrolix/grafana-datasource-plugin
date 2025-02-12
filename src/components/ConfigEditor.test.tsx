import React from 'react';
import {render, screen} from '@testing-library/react';
import {ConfigEditor, Props} from './ConfigEditor';
import '@testing-library/jest-dom';
import fs from "fs";
import { MyDataSourceOptions } from 'types';

const pluginJson = JSON.parse(fs.readFileSync('./src/plugin.json', 'utf-8'));

jest.mock('@grafana/runtime', () => {
    const original = jest.requireActual('@grafana/runtime');
    return {
        ...original,
        config: {buildInfo: {version: '10.0.0'}, secureSocksDSProxyEnabled: true},
    };
});

function getDefaultProps(overrides: MyDataSourceOptions){
    return {
        ...pluginJson,
        options: {
            jsonData: {
                host: "https://domain.com",
                port: 433,
                useDefaultPort: false,
                username: "use",
                ...overrides
            },
            secureJsonData: {password: 'pass'},
            secureJsonFields: {password: true}
        }
    } as Props
}

describe('ConfigEditor', () => {

    it('new editor', () => {
        render(<ConfigEditor {...getDefaultProps({})} />);
        expect(screen.getByLabelText("Host")).toBeInTheDocument();
        expect(screen.getByLabelText("Use default")).toBeInTheDocument();
        expect(screen.getByLabelText("Username")).toBeInTheDocument();
        expect(screen.getByLabelText("Password")).toBeInTheDocument();
    });

    it('port input is enabled', () => {

        let component = render(<ConfigEditor {...getDefaultProps({})} />);
        expect(component.container.querySelector('#config-editor-port')?.getAttribute("disabled")).toBeNull();
    });

    it('port input is disabled', () => {
        let component = render(<ConfigEditor {...getDefaultProps({useDefaultPort: true})} />);
        expect(component.container.querySelector('#config-editor-port')?.getAttribute("disabled")).toBe("");
    });
});
