import React from 'react';
import {QueryEditorProps} from '@grafana/data';
import {DataSource} from '../datasource';
import {HdxDataSourceOptions, HdxQuery} from '../types';
import {SQLEditor} from "@grafana/plugin-ui";
import {languageDefinition} from "../editor/languageDefinition";
import {Icon, ToolbarButton, ToolbarButtonRow} from "@grafana/ui";
import {getMetadataProvider} from "../editor/metadataProvider";

export type Props = QueryEditorProps<DataSource, HdxQuery, HdxDataSourceOptions>;

export function QueryEditor(props: Props) {
    const metadataProvider = getMetadataProvider(props);

    const onQueryTextChange = (queryText: string) => {
        props.onChange({...props.query, rawSql: queryText});
    };
    return (
        <>
            <SQLEditor query={props.query.rawSql} onChange={onQueryTextChange}
                       language={languageDefinition(metadataProvider)}>
                {({formatQuery}) => {
                    return (
                        <div>
                            <ToolbarButtonRow alignment={'right'}>
                                <ToolbarButton tooltip="Format query" onClick={formatQuery}>
                                    <Icon name="brackets-curly" onClick={formatQuery}/>
                                </ToolbarButton>
                            </ToolbarButtonRow>
                        </div>
                    );
                }}
            </SQLEditor>
        </>
    );

}
