import { TimeRange } from "@grafana/data";
import { ColumnDefinition } from "@grafana/plugin-ui";
import {
  ChatContextItem,
  createAssistantContextItem,
} from "@grafana/assistant";
import { isObject } from "../ast";

export interface HdxTableRef {
  database?: string;
  table: string;
}

export interface AssistantContextInput {
  rawSql: string;
  datasourceUid: string;
  datasourceHost?: string;
  timeRange?: TimeRange;
  table?: HdxTableRef;
  columns?: ColumnDefinition[];
  primaryTimeColumn?: string;
}

export interface AssistantContextPayload {
  sql: string;
  // The cluster the datasource is configured against. The MCP server is
  // configured with its own host, so this is the only signal available to the
  // model for noticing that the two disagree.
  datasourceHost?: string;
  timeRange?: { from: string; to: string };
  database?: string;
  table?: string;
  primaryTimeColumn?: string;
  columns?: Array<{ name: string; type?: string }>;
}

/**
 * Collects every table reference from a clickhouse-sql-parser AST (the
 * `/ast` backend resource's response). A TableIdentifier marshals as
 * `{Database?: {Name}, Table: {Name}}`, and `Table.Name` being a string is
 * unique to that node type — TableExpr's `Table` field holds an object, and
 * table functions carry `Name`/`Args` instead.
 */
export const collectTableRefs = (
  node: unknown,
  out: HdxTableRef[] = []
): HdxTableRef[] => {
  if (Array.isArray(node)) {
    node.forEach((child) => collectTableRefs(child, out));
    return out;
  }
  if (!isObject(node)) {
    return out;
  }
  const rec = node as Record<string, unknown>;
  const table = rec.Table;
  if (isObject(table) && typeof (table as { Name?: unknown }).Name === "string") {
    const database = rec.Database;
    const databaseName =
      isObject(database) &&
      typeof (database as { Name?: unknown }).Name === "string"
        ? ((database as { Name: string }).Name as string)
        : undefined;
    out.push({
      table: (table as { Name: string }).Name,
      ...(databaseName ? { database: databaseName } : {}),
    });
    return out;
  }
  Object.values(rec).forEach((child) => collectTableRefs(child, out));
  return out;
};

/**
 * The single table a parsed query reads from, or undefined. Deliberately
 * conservative: more than one distinct source resolves to undefined rather
 * than guessing, because a wrong table would attach the wrong schema to the
 * published context.
 */
export const resolveTableRef = (ast: unknown): HdxTableRef | undefined => {
  const refs = collectTableRefs(ast);
  if (refs.length === 0) {
    return undefined;
  }
  const [first] = refs;
  const single = refs.every(
    (r) => r.table === first.table && r.database === first.database
  );
  return single ? first : undefined;
};

export const buildAssistantContextPayload = (
  input: AssistantContextInput
): AssistantContextPayload => {
  const payload: AssistantContextPayload = { sql: input.rawSql ?? "" };

  if (input.datasourceHost) {
    payload.datasourceHost = input.datasourceHost;
  }
  if (input.timeRange) {
    payload.timeRange = {
      from: input.timeRange.from.toISOString(),
      to: input.timeRange.to.toISOString(),
    };
  }
  if (input.table) {
    payload.table = input.table.table;
    if (input.table.database) {
      payload.database = input.table.database;
    }
  }
  if (input.primaryTimeColumn) {
    payload.primaryTimeColumn = input.primaryTimeColumn;
  }
  if (input.columns?.length) {
    payload.columns = input.columns.map((c) => ({
      name: c.name,
      ...(c.type ? { type: c.type } : {}),
    }));
  }

  return payload;
};

export const buildAssistantContextItems = (
  input: AssistantContextInput
): ChatContextItem[] => {
  const payload = buildAssistantContextPayload(input);
  const title = payload.table
    ? `Hydrolix query (${payload.database ? `${payload.database}.` : ""}${payload.table})`
    : "Hydrolix query";

  return [
    createAssistantContextItem("datasource", {
      datasourceUid: input.datasourceUid,
    }),
    createAssistantContextItem("structured", { title, data: payload }),
  ];
};
