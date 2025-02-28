import React, {FormEvent, useRef} from 'react';
import {QueryEditorProps} from '@grafana/data';
import {DataSource} from '../datasource';
import {HdxDataSourceOptions, HdxQuery} from '../types';
import {SQLEditor} from "@grafana/plugin-ui";
import {languageDefinition} from "../editor/languageDefinition";
import {Icon, InlineField, InlineLabel, Input, ToolbarButton} from "@grafana/ui";
import {getMetadataProvider} from "../editor/metadataProvider";
import {QUERY_DURATION_REGEX} from "../editor/timeRangeUtils";

export type Props = QueryEditorProps<DataSource, HdxQuery, HdxDataSourceOptions>;

export function QueryEditor(props: Props) {
    const metadataProvider = getMetadataProvider(props);


    const onQueryTextChange = (queryText: string) => {
        props.onChange({...props.query, rawSql: queryText});
    };
    let invalidDuration = useRef(false);
    const onRoundChange = (e: FormEvent<HTMLInputElement>) => {
        let round = e.currentTarget.value;

        invalidDuration.current = !QUERY_DURATION_REGEX.test(round);
        props.onChange({...props.query, round: round});
    };
    return (
        <>
            <SQLEditor query={props.query.rawSql} onChange={onQueryTextChange}
                       language={languageDefinition(metadataProvider)}>
                {({formatQuery}) => {
                    return (
                        <div style={{display: "flex"}}>
                            <InlineField error={'invalid duration'} invalid={invalidDuration.current} test-id={'00'}
                                         label={
                                             <InlineLabel width={10}
                                                          tooltip="Set rounding for $from and $to timestamps...">
                                                 Round
                                             </InlineLabel>
                                         }
                            >
                                <Input width={10}
                                       data-testid="round-input"
                                       placeholder=""
                                       onChange={onRoundChange}
                                       value={props.query.round}
                                />
                            </InlineField>
                            <div style={{marginLeft: "auto", order: 2}}>
                                <ToolbarButton tooltip="Format query" onClick={formatQuery} test-id={'11'}>
                                    <Icon name="brackets-curly" onClick={formatQuery}/>
                                </ToolbarButton>
                            </div>
                        </div>
                    );
                }}
            </SQLEditor>
        </>
    );

}
