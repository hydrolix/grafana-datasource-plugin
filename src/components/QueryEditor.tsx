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
  HdxDataSourceOptions,
  HdxQuery,
  InterpolationResult,
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
import {
  SHOW_INTERPOLATED_QUERY_ERRORS,
  SHOW_VALIDATION_BAR,
} from "../constants";

import { css } from "@emotion/css";

export type Props = QueryEditorProps<
  DataSource,
  HdxQuery,
  HdxDataSourceOptions
>;

export function QueryEditor(props: Props) {
  const alertStyle = css`
    div:has(> div[role="alert"]) {
      min-width: 10rem;
      position: absolute;
      z-index: 1;
    }
  `;

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
  const [interpolationResult, setInterpolationResult] =
    useState<InterpolationResult>({
      originalSql: props.query.rawSql,
      interpolationId: "",
      hasError: false,
      hasWarning: false,
    });
  useState("");
  let [monaco, setMonaco] = useState<Monaco | null>(null);

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

  const onQueryTextChange = (queryText: string) => {
    props.onChange({ ...props.query, rawSql: queryText });
  };
  let invalidDuration = useRef(false);
  const onRoundChange = (e: FormEvent<HTMLInputElement>) => {
    let round = e.currentTarget.value;

    invalidDuration.current = !QUERY_DURATION_REGEX.test(round);
    props.onChange({ ...props.query, round: round });
  };

  // track values change and refresh interpolated query
  const [interpolationId, setInterpolationId] = useState<string>("");
  const variablesString = props.datasource.templateSrv
    .getVariables()
    .map((v: any) => (v?.current?.value ? v?.current?.value : v?.filters))
    .map((v: any) => (Array.isArray(v) ? v : [v]))
    .flat()
    .map((v) => (typeof v === "object" ? Object.values(v).join(",") : v))
    .join(":");
  const interpolationIdString = `${props.query.rawSql}|${props.query.round}|${variablesString}`;

  if (interpolationId !== interpolationIdString) {
    setInterpolationId(interpolationIdString);
  }
  useDebounce(
    async () => {
      if (showSql || SHOW_VALIDATION_BAR) {
        if (props.datasource.options) {
          let interpolatedQuery = await props.datasource.interpolateQuery(
            props.query.rawSql,
            interpolationId,
            {
              ...props.datasource.options,
              filters: props.datasource.filters,
            },
            getFirstValidRound([
              props.query.round,
              props.datasource.instanceSettings.jsonData.defaultRound || "",
            ])
          );
          setInterpolationResult(interpolatedQuery);
        } else {
          dryRun();
        }
      }
    },
    300,
    [showSql, interpolationId]
  );
  // eslint-disable-next-line eqeqeq
  let dirty = interpolationResult?.interpolationId != interpolationId;
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
              {SHOW_VALIDATION_BAR && (
                <ValidationBar
                  monaco={monaco}
                  interpolationResult={interpolationResult}
                  query={props.query.rawSql}
                />
              )}
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
                  className={alertStyle}
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
        sql={interpolationResult.interpolatedSql ?? ""}
        error={interpolationResult.error ?? ""}
        showSQL={showSql}
        dirty={dirty}
        showErrors={SHOW_INTERPOLATED_QUERY_ERRORS}
      />
    </div>
  );
}
