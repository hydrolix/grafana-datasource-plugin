import React from 'react';
import {QueryEditorProps} from '@grafana/data';
import {DataSource} from '../datasource';
import {HdxDataSourceOptions, HdxQuery} from '../types';
import {SQLEditor} from "@grafana/plugin-ui";

type Props = QueryEditorProps<DataSource, HdxQuery, HdxDataSourceOptions>;

export function QueryEditor(props: Props) {
    console.log("invoke query editor", props.query.refId)
    const onQueryTextChange = (queryText: string) => {
        props.onChange({...props.query, rawSql: queryText});
    };

    return (
        <>
            <SQLEditor query={props.query.rawSql} onChange={onQueryTextChange}/>
        </>
    );
}
