import React, {FormEvent, useCallback, useRef, useState} from 'react';
import {QueryEditorProps, SelectableValue} from '@grafana/data';
import {DataSource} from '../datasource';
import {HdxDataSourceOptions, HdxQuery, QueryType} from '../types';
import {SQLEditor} from "@grafana/plugin-ui";
import {languageDefinition} from "../editor/languageDefinition";
import {Icon, InlineField, InlineLabel, Input, RadioButtonGroup, ToolbarButton} from "@grafana/ui";
import {getMetadataProvider} from "../editor/metadataProvider";
import {QUERY_DURATION_REGEX} from "../editor/timeRangeUtils";

export type Props = QueryEditorProps<DataSource, HdxQuery, HdxDataSourceOptions>;

export function QueryEditor(props: Props) {
    const metadataProvider = getMetadataProvider(props);

    const queryTypeOptions: Array<SelectableValue<string>> = Object.keys(QueryType).map(key => ({
        label: key,
        value: QueryType[key as keyof typeof QueryType]
    }));

    const onQueryTextChange = (queryText: string) => {
        props.onChange({...props.query, rawSql: queryText});
    };
    let invalidDuration = useRef(false);
    const onRoundChange = (e: FormEvent<HTMLInputElement>) => {
        let round = e.currentTarget.value;

        invalidDuration.current = !QUERY_DURATION_REGEX.test(round);
        props.onChange({...props.query, round: round});
    };
    const [queryType, setQueryType] = useState(props.query.queryType || QueryType.Table);
    const updateQueryType = useCallback((q: string) => {
        setQueryType(() => q);
        props.query.queryType = q
    }, [props]);

    return (
        <div>
            <SQLEditor query={props.query.rawSql} onChange={onQueryTextChange}
                       language={languageDefinition(metadataProvider, props)}>
                {({formatQuery}) => {
                    return (
                        <div style={{display: "flex"}}>
                                <InlineField
                                             label={
                                                 <InlineLabel width={15}
                                                              tooltip="Set query type">
                                                     Query Type
                                                 </InlineLabel>
                                             }
                                >
                                    <RadioButtonGroup options={queryTypeOptions}
                                                      value={queryType}
                                                      onChange={v => updateQueryType(v!)} size={'md'}/>
                                </InlineField>

                                <InlineField error={'invalid duration'} invalid={invalidDuration.current}
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
                                <ToolbarButton tooltip="Format query" onClick={formatQuery}>
                                    <Icon name="brackets-curly" onClick={formatQuery}/>
                                </ToolbarButton>
                            </div>
                        </div>
                    );
                }}
            </SQLEditor>
        </div>
    );

}
