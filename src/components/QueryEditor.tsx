import React, {
  FormEvent,
  useCallback,
  useMemo,
  useRef,
  useState,
} from "react";
import { QueryEditorProps, SelectableValue } from "@grafana/data";
import { DataSource } from "../datasource";
import { HdxDataSourceOptions, HdxQuery, QueryType } from "../types";
import { SQLEditor } from "@grafana/plugin-ui";
import { languageDefinition } from "../editor/languageDefinition";
import {
  Icon,
  InlineField,
  InlineLabel,
  Input,
  RadioButtonGroup,
  ToolbarButton,
} from "@grafana/ui";
import { QUERY_DURATION_REGEX } from "../editor/timeRangeUtils";

export type Props = QueryEditorProps<
  DataSource,
  HdxQuery,
  HdxDataSourceOptions
>;

export function QueryEditor(props: Props) {
  const queryTypeOptions: Array<SelectableValue<number>> = Object.keys(
    QueryType
  )
    .filter((key) => Number.isNaN(+key))
    .map((key) => ({
      label: key,
      value: QueryType[key as keyof typeof QueryType],
    }));

  const onQueryTextChange = (queryText: string) => {
    props.onChange({ ...props.query, rawSql: queryText });
  };
  let invalidDuration = useRef(false);
  const onRoundChange = (e: FormEvent<HTMLInputElement>) => {
    let round = e.currentTarget.value;

    invalidDuration.current = !QUERY_DURATION_REGEX.test(round);
    props.onChange({ ...props.query, round: round });
  };
  useMemo(() => {
    props.query.format = props.query.format || QueryType.Table;
  }, [props.query]);

  const [queryType, setQueryType] = useState(props.query.format);
  const updateQueryType = useCallback(
    (q: number) => {
      setQueryType(() => q);
      props.query.format = q;
    },
    [props]
  );

  return (
    <div>
      <SQLEditor
        query={props.query.rawSql}
        onChange={onQueryTextChange}
        language={languageDefinition(props)}
      >
        {({ formatQuery }) => {
          return (
            <div style={{ display: "flex" }}>
              <InlineField
                label={
                  <InlineLabel width={15} tooltip="Set query type">
                    Query Type
                  </InlineLabel>
                }
              >
                <RadioButtonGroup
                  options={queryTypeOptions}
                  value={queryType}
                  onChange={(v) => updateQueryType(+v)}
                  size={"md"}
                />
              </InlineField>

              <InlineField
                error={"invalid duration"}
                invalid={invalidDuration.current}
                label={
                  <InlineLabel
                    width={10}
                    tooltip="Set rounding for $from and $to timestamps..."
                  >
                    Round
                  </InlineLabel>
                }
              >
                <Input
                  width={10}
                  data-testid="round-input"
                  onChange={onRoundChange}
                  value={props.query.round}
                />
              </InlineField>
              <div style={{ marginLeft: "auto", order: 2 }}>
                <ToolbarButton tooltip="Format query" onClick={formatQuery}>
                  <Icon name="brackets-curly" onClick={formatQuery} />
                </ToolbarButton>
              </div>
            </div>
          );
        }}
      </SQLEditor>
    </div>
  );
}
