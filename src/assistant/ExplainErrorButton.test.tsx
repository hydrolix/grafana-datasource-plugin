import React from "react";
import { render, screen } from "@testing-library/react";
import { DataQueryError, LoadingState, PanelData } from "@grafana/data";
import { OpenAssistantButton } from "@grafana/assistant";
import { DataSource } from "../datasource";
import {
  buildExplainErrorPrompt,
  ExplainErrorButton,
  findQueryError,
} from "./ExplainErrorButton";

jest.mock("@grafana/assistant", () => ({
  OpenAssistantButton: jest.fn(() => <button>Explain error</button>),
  createAssistantContextItem: jest.fn(
    (type: string, params: { data?: unknown }) => ({
      node: { id: type, name: type, navigable: false, data: params.data },
      occurrences: [],
    })
  ),
}));

const mockButton = OpenAssistantButton as jest.Mock;

const datasource = {
  uid: "hdx-uid",
  instanceSettings: { jsonData: { host: "cluster.example.com" } },
  // ExplainErrorButton only reads the fields above.
} as unknown as DataSource;

const panelData = (errors: DataQueryError[]): PanelData =>
  ({
    state: LoadingState.Error,
    series: [],
    errors,
  }) as unknown as PanelData;

const TIMERANGE_ERROR: DataQueryError = {
  refId: "A",
  message:
    "<HdxStorageError hdx_query_timerange_required is set to true. Your query needs a time range filter in a WHERE clause>",
};

describe("findQueryError", () => {
  it("finds the error matching the query's refId", () => {
    const data = panelData([{ refId: "B", message: "other" }, TIMERANGE_ERROR]);
    expect(findQueryError(data, "A")).toBe(TIMERANGE_ERROR);
  });

  it("ignores errors belonging to other queries", () => {
    const data = panelData([{ refId: "B", message: "other" }]);
    expect(findQueryError(data, "A")).toBeUndefined();
  });

  it("applies a refId-less error to any query", () => {
    const connectionError: DataQueryError = { message: "connection refused" };
    expect(findQueryError(panelData([connectionError]), "A")).toBe(
      connectionError
    );
  });

  it("returns undefined without panel data", () => {
    expect(findQueryError(undefined, "A")).toBeUndefined();
  });
});

describe("buildExplainErrorPrompt", () => {
  it("includes the SQL, the error, and the classified guidance", () => {
    const prompt = buildExplainErrorPrompt(
      "SELECT * FROM cicd.logs",
      TIMERANGE_ERROR
    );

    expect(prompt).toContain("SELECT * FROM cicd.logs");
    expect(prompt).toContain("hdx_query_timerange_required");
    expect(prompt).toContain("classified as TIMERANGE_REQUIRED");
    expect(prompt).toContain("must include a time range filter");
  });

  it("classifies against the raw message under error.data first", () => {
    const beautified: DataQueryError = {
      refId: "A",
      message: "beautified summary",
      data: { message: TIMERANGE_ERROR.message },
    };

    const prompt = buildExplainErrorPrompt("SELECT 1", beautified);

    expect(prompt).toContain("classified as TIMERANGE_REQUIRED");
    // The user-facing message stays the beautified one.
    expect(prompt).toContain("beautified summary");
  });

  it("omits the guidance section for an unclassified error", () => {
    const prompt = buildExplainErrorPrompt("SELECT 1", {
      refId: "A",
      message: "something entirely novel",
    });

    expect(prompt).toContain("something entirely novel");
    expect(prompt).not.toContain("classified as");
  });
});

describe("ExplainErrorButton", () => {
  afterEach(() => jest.clearAllMocks());

  it("renders for this query's error with prompt and context", () => {
    render(
      <ExplainErrorButton
        datasource={datasource}
        rawSql="SELECT * FROM cicd.logs"
        refId="A"
        data={panelData([TIMERANGE_ERROR])}
      />
    );

    expect(screen.getByText("Explain error")).toBeInTheDocument();
    const props = mockButton.mock.calls[0][0];
    expect(props.autoSend).toBe(false);
    expect(props.origin).toBe(
      "hydrolix-hydrolix-datasource/query-editor/error"
    );
    expect(props.prompt).toContain("SELECT * FROM cicd.logs");
    expect(props.context).toHaveLength(2);
  });

  it("renders nothing when the error belongs to another query", () => {
    const { container } = render(
      <ExplainErrorButton
        datasource={datasource}
        rawSql="SELECT 1"
        refId="A"
        data={panelData([{ refId: "B", message: "other" }])}
      />
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing without an error or without SQL", () => {
    const { container } = render(
      <>
        <ExplainErrorButton
          datasource={datasource}
          rawSql="SELECT 1"
          refId="A"
          data={panelData([])}
        />
        <ExplainErrorButton
          datasource={datasource}
          rawSql=""
          refId="A"
          data={panelData([TIMERANGE_ERROR])}
        />
      </>
    );

    expect(container).toBeEmptyDOMElement();
  });
});
