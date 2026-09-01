import React from "react";
import { render, waitFor } from "@testing-library/react";
import { dateTime, TimeRange } from "@grafana/data";
import {
  ChatContextItem,
  useProvidePageContext,
  createAssistantContextItem,
} from "@grafana/assistant";
import { DataSource } from "../datasource";
import { AssistantQueryContext } from "./AssistantQueryContext";

jest.mock("@grafana/assistant", () => ({
  useProvidePageContext: jest.fn(),
  createAssistantContextItem: jest.fn(
    (type: string, params: { data?: unknown }): ChatContextItem => ({
      node: { id: type, name: type, navigable: false, data: params.data },
      occurrences: [],
    })
  ),
}));

const mockProvide = useProvidePageContext as jest.Mock;
const mockCreateItem = createAssistantContextItem as jest.Mock;

const range: TimeRange = {
  from: dateTime("2026-08-14T00:00:00Z"),
  to: dateTime("2026-08-15T00:00:00Z"),
  raw: { from: "now-1d", to: "now" },
};

// AST for `SELECT * FROM cicd.logs`, trimmed to the keys the collector reads.
const AST_CICD_LOGS = [
  {
    From: {
      Expr: {
        Table: {
          Expr: {
            Database: { Name: "cicd", QuoteType: 1 },
            Table: { Name: "logs", QuoteType: 1 },
          },
        },
      },
    },
  },
];

// AST for an unqualified `SELECT * FROM logs`.
const AST_LOGS = [
  {
    From: {
      Expr: {
        Table: { Expr: { Database: null, Table: { Name: "logs", QuoteType: 1 } } },
      },
    },
  },
];

interface DatasourceStubOptions {
  ast?: unknown;
  astError?: Error;
  defaultDatabase?: string;
}

const makeDatasource = ({
  ast,
  astError,
  defaultDatabase,
}: DatasourceStubOptions) => {
  const stub = {
    uid: "hdx-uid",
    instanceSettings: {
      jsonData: { host: "cluster.example.com", defaultDatabase },
    },
    getAst: astError
      ? jest.fn().mockRejectedValue(astError)
      : jest.fn().mockResolvedValue(ast ?? []),
    metadataProvider: {
      columns: jest
        .fn()
        .mockResolvedValue([{ name: "timestamp", type: "DateTime64" }]),
      primaryKey: jest.fn().mockResolvedValue("timestamp"),
      executeQuery: jest.fn(),
    },
  };
  // The component only touches the stubbed surface above.
  return { stub, datasource: stub as unknown as DataSource };
};

const structuredPayload = (setContext: jest.Mock) => {
  const items: ChatContextItem[] = setContext.mock.calls.at(-1)?.[0];
  const structured = items.find((i) => i.node.id === "structured");
  return structured?.node.data;
};

describe("AssistantQueryContext", () => {
  let setContext: jest.Mock;

  beforeEach(() => {
    setContext = jest.fn();
    mockProvide.mockReturnValue(setContext);
  });

  afterEach(() => jest.clearAllMocks());

  it("publishes datasource and structured items with schema for a resolved table", async () => {
    const { stub, datasource } = makeDatasource({ ast: AST_CICD_LOGS });

    render(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELECT * FROM cicd.logs"
        range={range}
      />
    );

    await waitFor(() => expect(setContext).toHaveBeenCalled());

    expect(stub.metadataProvider.columns).toHaveBeenCalledWith({
      schema: "cicd",
      table: "logs",
    });
    const payload = structuredPayload(setContext);
    expect(payload).toMatchObject({
      sql: "SELECT * FROM cicd.logs",
      datasourceHost: "cluster.example.com",
      database: "cicd",
      table: "logs",
      primaryTimeColumn: "timestamp",
      columns: [{ name: "timestamp", type: "DateTime64" }],
    });
    expect(mockCreateItem).toHaveBeenCalledWith("datasource", {
      datasourceUid: "hdx-uid",
    });
  });

  it("qualifies an unqualified table with the default database", async () => {
    const { stub, datasource } = makeDatasource({
      ast: AST_LOGS,
      defaultDatabase: "mydb",
    });

    render(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELECT * FROM logs"
        range={range}
      />
    );

    await waitFor(() => expect(setContext).toHaveBeenCalled());

    expect(stub.metadataProvider.columns).toHaveBeenCalledWith({
      schema: "mydb",
      table: "logs",
    });
    expect(structuredPayload(setContext)).toMatchObject({
      database: "mydb",
      table: "logs",
    });
  });

  it("skips the schema fetch when no database is resolvable", async () => {
    const { stub, datasource } = makeDatasource({ ast: AST_LOGS });

    render(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELECT * FROM logs"
        range={range}
      />
    );

    await waitFor(() => expect(setContext).toHaveBeenCalled());

    expect(stub.metadataProvider.columns).not.toHaveBeenCalled();
    const payload = structuredPayload(setContext);
    expect(payload.table).toBe("logs");
    expect("columns" in payload).toBe(false);
  });

  it("publishes basic context when the SQL does not parse", async () => {
    const { stub, datasource } = makeDatasource({
      astError: new Error("syntax error"),
    });

    render(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELEC broken"
        range={range}
      />
    );

    await waitFor(() => expect(setContext).toHaveBeenCalled());

    expect(stub.metadataProvider.columns).not.toHaveBeenCalled();
    const payload = structuredPayload(setContext);
    expect(payload).toMatchObject({
      sql: "SELEC broken",
      datasourceHost: "cluster.example.com",
    });
    expect("table" in payload).toBe(false);
  });

  it("does not parse or fetch for an empty query", async () => {
    const { stub, datasource } = makeDatasource({});

    render(
      <AssistantQueryContext datasource={datasource} rawSql="" range={range} />
    );

    await waitFor(() => expect(setContext).toHaveBeenCalled());

    expect(stub.getAst).not.toHaveBeenCalled();
    expect(structuredPayload(setContext).sql).toBe("");
  });

  it("republishes when the SQL changes", async () => {
    const { datasource } = makeDatasource({ ast: AST_CICD_LOGS });

    const { rerender } = render(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELECT * FROM cicd.logs"
        range={range}
      />
    );
    await waitFor(() => expect(setContext).toHaveBeenCalledTimes(1));

    rerender(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELECT count(*) FROM cicd.logs"
        range={range}
      />
    );
    await waitFor(() => expect(setContext).toHaveBeenCalledTimes(2));

    expect(structuredPayload(setContext).sql).toBe(
      "SELECT count(*) FROM cicd.logs"
    );
  });

  it("suppresses a stale publish that resolves after a newer one", async () => {
    const { stub, datasource } = makeDatasource({});
    let resolveOld: (ast: unknown) => void = () => {};
    stub.getAst
      .mockImplementationOnce(
        () => new Promise((resolve) => (resolveOld = resolve))
      )
      .mockResolvedValueOnce(AST_CICD_LOGS);

    const { rerender } = render(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELECT * FROM old.table1"
        range={range}
      />
    );
    // Let the first debounce fire and hang on the old parse.
    await waitFor(() => expect(stub.getAst).toHaveBeenCalledTimes(1));

    rerender(
      <AssistantQueryContext
        datasource={datasource}
        rawSql="SELECT * FROM cicd.logs"
        range={range}
      />
    );
    await waitFor(() => expect(setContext).toHaveBeenCalledTimes(1));
    expect(structuredPayload(setContext).sql).toBe("SELECT * FROM cicd.logs");

    resolveOld([]);
    // Give the stale continuation a chance to (incorrectly) publish.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(setContext).toHaveBeenCalledTimes(1);
  });
});
