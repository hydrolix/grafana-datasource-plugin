import React, {
  FormEvent,
  useCallback,
  useMemo,
  useRef,
  useState,
} from "react";
import { QueryEditorProps, SelectableValue } from "@grafana/data";
import { DataSource } from "../datasource";
import {
  AstResponse,
  HdxDataSourceOptions,
  HdxQuery,
  QueryType,
} from "../types";
import { SQLEditor } from "@grafana/plugin-ui";
import { languageDefinition } from "../editor/languageDefinition";
import {
  Button,
  Icon,
  InlineField,
  InlineLabel,
  Input,
  Monaco,
  Select,
  ToolbarButton,
} from "@grafana/ui";
import {
  getFirstValidRound,
  QUERY_DURATION_REGEX,
} from "../editor/timeRangeUtils";
import { InterpolatedQuery } from "./InterpolatedQuery";
import { ValidationBar } from "./ValidationBar";
import { useDebounce } from "react-use";

export type Props = QueryEditorProps<
  DataSource,
  HdxQuery,
  HdxDataSourceOptions
>;

export function QueryEditor(props: Props) {
  const queryTypeOptions = useMemo<Array<SelectableValue<number>>>(
    () =>
      Object.keys(QueryType)
        .filter((key) => Number.isNaN(+key))
        .map((key) => ({
          label: key,
          value: QueryType[key as keyof typeof QueryType],
        })),
    []
  );
  const getQueryTypeValue = useCallback(
    (format: number) => {
      return queryTypeOptions.find((q) => q.value === format);
    },
    [queryTypeOptions]
  );
  useMemo(() => {
    if (props.query.format === undefined) {
      props.query.format = QueryType.Table;
    }
  }, [props.query]);

  const [queryType, setQueryType] = useState(
    getQueryTypeValue(props.query.format!)
  );
  const updateQueryType = useCallback(
    (q: SelectableValue<number>) => {
      setQueryType(() => q);
      props.onChange({ ...props.query, format: q.value });
    },
    [props]
  );

  const [showSql, setShowSql] = useState(false);
  const [dryRunTriggered, setDryRunTriggered] = useState(false);
  const [interpolatedSql, setInterpolatedSql] = useState("");
  const [interpolatingErrorMessage, setInterpolatingErrorMessage] =
    useState("");
  let [monaco, setMonaco] = useState<Monaco | null>(null);

  let [debouncedSql, setDebouncedSql] = useState<string>("");
  let [astResponse, setAstResponse] = useState<AstResponse | null>(null);

  const dryRun = useCallback(() => {
    if (!dryRunTriggered && props.query.rawSql) {
      setDryRunTriggered(true);
      let setDryRun = () => {
        let dry = true;
        return () => {
          let r = dry;
          dry = false;
          return r;
        };
      };
      props.onChange({ ...props.query, skipNextRun: setDryRun() });
      props.onRunQuery();
    }
  }, [props, dryRunTriggered]);

  useMemo(async () => {
    if (showSql) {
      if (props.datasource.options && astResponse) {
        try {
          let interpolatedQuery = await props.datasource.interpolateQuery(
            astResponse.originalSql,
            props.datasource.options,
            getFirstValidRound([
              props.query.round,
              props.datasource.instanceSettings.jsonData.defaultRound || "",
            ]),
            astResponse.data
          );
          setInterpolatingErrorMessage("");
          setInterpolatedSql(interpolatedQuery);
        } catch (e) {
          let message;
          if (e instanceof Error) {
            message = e.message;
          } else {
            message = "Unknown Error";
          }
          setInterpolatingErrorMessage(message);
        }
      } else {
        dryRun();
      }
    }
  }, [props.datasource, props.query, dryRun, showSql, astResponse]);

  const onQueryTextChange = (queryText: string) => {
    props.onChange({ ...props.query, rawSql: queryText });
  };
  let invalidDuration = useRef(false);
  const onRoundChange = (e: FormEvent<HTMLInputElement>) => {
    let round = e.currentTarget.value;

    invalidDuration.current = !QUERY_DURATION_REGEX.test(round);
    props.onChange({ ...props.query, round: round });
  };

  useDebounce(
    () => {
      setDebouncedSql(props.query.rawSql);
    },
    300,
    [props.query.rawSql]
  );
  useMemo(async () => {
    let astResponse: AstResponse = await props.datasource.getAst(debouncedSql);
    setAstResponse(astResponse);
  }, [debouncedSql, props.datasource]);

  return (
    <div>
      <SQLEditor
        query={props.query.rawSql}
        onChange={onQueryTextChange}
        language={languageDefinition(props, setMonaco)}
      >
        {({ formatQuery }) => {
          return (
            <div>
              <ValidationBar
                monaco={monaco}
                astResponse={astResponse}
                query={props.query.rawSql}
              />
              <div
                style={{
                  display: "flex",
                  flexWrap: "wrap",
                  gap: 6,
                  paddingTop: 8,
                }}
              >
                <InlineField
                  data-testid="data-testid query type"
                  label={
                    <InlineLabel width={15} tooltip="Set query type">
                      Query Type
                    </InlineLabel>
                  }
                >
                  <Select
                    options={queryTypeOptions}
                    value={queryType}
                    onChange={(v) => updateQueryType(v)}
                    size={"md"}
                  />
                </InlineField>
                <InlineField
                  error={"invalid duration"}
                  invalid={invalidDuration.current}
                  label={
                    <InlineLabel
                      width={10}
                      tooltip="Round $from and $to timestamps to the nearest multiple of the specified value (1m rounds to the nearest whole minute). Supports time units: ms, s, m, h. No value means that the default round value will be used. A value of 0 means no rounding is applied"
                    >
                      Round
                    </InlineLabel>
                  }
                >
                  <Input
                    width={10}
                    data-testid="data-testid round input"
                    onChange={onRoundChange}
                    value={props.query.round}
                  />
                </InlineField>
                {showSql ? (
                  <Button icon="eye-slash" onClick={() => setShowSql(false)}>
                    Hide Interpolated Query
                  </Button>
                ) : (
                  <Button icon="eye" onClick={() => setShowSql(true)}>
                    Show Interpolated Query
                  </Button>
                )}
                <div style={{ marginLeft: "auto", order: 2 }}>
                  <ToolbarButton tooltip="Format query" onClick={formatQuery}>
                    <Icon name="brackets-curly" onClick={formatQuery} />
                  </ToolbarButton>
                </div>
              </div>
            </div>
          );
        }}
      </SQLEditor>
      <InterpolatedQuery
        sql={interpolatedSql}
        error={interpolatingErrorMessage}
        showSQL={showSql}
      />
    </div>
  );
}
