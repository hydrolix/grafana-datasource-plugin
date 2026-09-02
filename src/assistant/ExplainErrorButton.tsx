import React from "react";
import { DataQueryError, PanelData } from "@grafana/data";
import { OpenAssistantButton } from "@grafana/assistant";
import { matchSolutionTemplate } from "../errors/errorSolution";
import { DataSource } from "../datasource";
import { buildAssistantContextItems } from "./context";

export interface ExplainErrorButtonProps {
  datasource: DataSource;
  rawSql: string;
  refId: string;
  data?: PanelData;
}

/**
 * The panel's latest response arrives through QueryEditorProps.data with each
 * query's error carrying its refId — no datasource-side error state needed.
 */
export const findQueryError = (
  data: PanelData | undefined,
  refId: string
): DataQueryError | undefined =>
  // An error without a refId (e.g. a connection-level failure) applies to
  // every query in the panel, this one included. The deprecated single
  // `data.error` field is just `errors[0]` on Grafana >= 10 — not consulted.
  data?.errors?.find((e) => !e.refId || e.refId === refId);

export const buildExplainErrorPrompt = (
  rawSql: string,
  error: DataQueryError
): string => {
  // query() beautifies error.message before it reaches the panel; the raw
  // text under error.data is what the solution-template regexps target, so
  // classify against it first.
  const rawMessage = error.data?.message || error.message || "";
  const match =
    matchSolutionTemplate(rawMessage) ??
    (error.message ? matchSolutionTemplate(error.message) : undefined);

  const sections = [
    "Explain why this Hydrolix query fails and propose a corrected query.",
    `Failing query:\n\`\`\`sql\n${rawSql}\n\`\`\``,
    `Error:\n${error.message || rawMessage}`,
  ];
  if (match) {
    sections.push(
      `The error is classified as ${match.name}. Hydrolix's documented guidance for it:\n${match.solution}`
    );
  }
  return sections.join("\n\n");
};

export function ExplainErrorButton({
  datasource,
  rawSql,
  refId,
  data,
}: ExplainErrorButtonProps) {
  const error = findQueryError(data, refId);
  if (!error || !rawSql) {
    return null;
  }

  return (
    <OpenAssistantButton
      title="Explain error"
      origin="hydrolix-hydrolix-datasource/query-editor/error"
      prompt={buildExplainErrorPrompt(rawSql, error)}
      // Let the user see what leaves the editor (the SQL and the error text)
      // before it is sent.
      autoSend={false}
      context={buildAssistantContextItems({
        rawSql,
        datasourceUid: datasource.uid,
        datasourceHost: datasource.instanceSettings?.jsonData?.host,
      })}
    />
  );
}
