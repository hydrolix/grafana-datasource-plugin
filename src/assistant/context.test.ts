import { dateTime, TimeRange } from "@grafana/data";
import {
  buildAssistantContextPayload,
  collectTableRefs,
  resolveTableRef,
} from "./context";

// Trimmed nodes from real clickhouse-sql-parser output (the /ast resource's
// response shape). Positions and unrelated nulls omitted; the structural keys
// the collector navigates are verbatim.
const tableIdent = (table: string, database?: string) => ({
  ...(database ? { Database: { Name: database, QuoteType: 1 } } : { Database: null }),
  Table: { Name: table, QuoteType: 1 },
});

const selectFrom = (expr: object) => ({
  SelectItems: [{ Expr: { Name: "*", QuoteType: 0 } }],
  From: { Expr: { Table: { Alias: null, Expr: expr, HasFinal: false } } },
  Where: null,
});

describe("collectTableRefs / resolveTableRef", () => {
  it("resolves a qualified table", () => {
    const ast = [selectFrom(tableIdent("logs", "cicd"))];
    expect(resolveTableRef(ast)).toEqual({ database: "cicd", table: "logs" });
  });

  it("resolves an unqualified table", () => {
    const ast = [selectFrom(tableIdent("logs"))];
    expect(resolveTableRef(ast)).toEqual({ table: "logs" });
  });

  it("keeps identifiers containing non-word characters intact", () => {
    // SELECT * FROM "my-project"."logs" — the parser strips quoting and
    // preserves the full name; nothing left to truncate.
    const ast = [selectFrom(tableIdent("logs", "my-project"))];
    expect(resolveTableRef(ast)).toEqual({
      database: "my-project",
      table: "logs",
    });
  });

  it("resolves an aliased table", () => {
    // SELECT * FROM logs AS l — the identifier is wrapped in an alias node.
    const ast = [
      selectFrom({
        Expr: tableIdent("logs"),
        Alias: { Name: "l", QuoteType: 1 },
      }),
    ];
    expect(resolveTableRef(ast)).toEqual({ table: "logs" });
  });

  it("tolerates repeated references to the same table", () => {
    // SELECT * FROM cicd.logs UNION ALL SELECT * FROM cicd.logs
    const ast = [
      {
        ...selectFrom(tableIdent("logs", "cicd")),
        Union: selectFrom(tableIdent("logs", "cicd")),
        UnionMode: "ALL",
      },
    ];
    expect(resolveTableRef(ast)).toEqual({ database: "cicd", table: "logs" });
  });

  it("resolves verbatim parser output", () => {
    // Unedited `/ast` response for `SELECT * FROM cicd.logs WHERE x = 1`,
    // captured from clickhouse-sql-parser v0.5.2 — the version go.mod pins.
    // Guards the trimmed fixtures above against parser shape drift.
    const ast = JSON.parse(
      '[{"SelectPos":0,"StatementEnd":35,"With":null,"Top":null,"HasDistinct":false,"DistinctOn":null,"SelectItems":[{"Expr":{"Name":"*","QuoteType":0,"NamePos":7,"NameEnd":7},"Modifiers":[],"Alias":null}],"From":{"FromPos":9,"Expr":{"Table":{"TablePos":14,"TableEnd":23,"Alias":null,"Expr":{"Database":{"Name":"cicd","QuoteType":1,"NamePos":14,"NameEnd":18},"Table":{"Name":"logs","QuoteType":1,"NamePos":19,"NameEnd":23}},"HasFinal":false},"StatementEnd":23,"SampleRatio":null,"HasFinal":false}},"Window":null,"Prewhere":null,"Where":{"WherePos":24,"Expr":{"LeftExpr":{"Name":"x","QuoteType":1,"NamePos":30,"NameEnd":31},"Operation":"=","RightExpr":{"NumPos":34,"NumEnd":35,"Literal":"1","Base":10},"HasGlobal":false,"HasNot":false}},"GroupBy":null,"WithTotal":false,"Having":null,"OrderBy":null,"LimitBy":null,"Limit":null,"Settings":null,"HasParen":false,"OuterSettings":null,"Format":null,"Union":null,"UnionMode":"","Except":null,"ExceptMode":"","Intersect":null,"IntersectMode":""}]'
    );
    expect(resolveTableRef(ast)).toEqual({ database: "cicd", table: "logs" });
  });

  it("resolves a subquery over a single table", () => {
    // SELECT * FROM (SELECT * FROM cicd.logs) x
    const ast = [
      selectFrom({
        Expr: { HasParen: true, Select: selectFrom(tableIdent("logs", "cicd")) },
        Alias: { Name: "x", QuoteType: 1 },
      }),
    ];
    expect(resolveTableRef(ast)).toEqual({ database: "cicd", table: "logs" });
  });

  it("declines to guess when sources differ", () => {
    // SELECT * FROM a, b — a comma join parses to Left/Right table exprs.
    const ast = [
      {
        SelectItems: [{ Expr: { Name: "*", QuoteType: 0 } }],
        From: {
          Expr: {
            Left: { Table: { Expr: tableIdent("a"), HasFinal: false } },
            Right: { Table: { Expr: tableIdent("b"), HasFinal: false } },
          },
        },
      },
    ];
    expect(resolveTableRef(ast)).toBeUndefined();
    expect(collectTableRefs(ast)).toEqual([{ table: "a" }, { table: "b" }]);
  });

  it("finds no table in a table function", () => {
    // SELECT * FROM merge('db','^logs') — a function node has Name/Args, not
    // a Table ident.
    const ast = [
      selectFrom({
        Name: { Name: "merge", QuoteType: 1 },
        Args: { Args: [{ Literal: "db" }, { Literal: "^logs" }] },
      }),
    ];
    expect(resolveTableRef(ast)).toBeUndefined();
  });

  it("finds no table in literals or FROM-less selects", () => {
    // SELECT 'from null' / SELECT 1 -- from staging: literals and comments
    // never become table nodes.
    const ast = [
      {
        SelectItems: [{ Expr: { Literal: "from null" } }],
        From: null,
      },
    ];
    expect(resolveTableRef(ast)).toBeUndefined();
  });

  it("returns undefined for empty input", () => {
    expect(resolveTableRef([])).toBeUndefined();
    expect(resolveTableRef(null)).toBeUndefined();
  });
});

const range = (from: string, to: string): TimeRange => ({
  from: dateTime(from),
  to: dateTime(to),
  raw: { from, to },
});

describe("buildAssistantContextPayload", () => {
  const base = { rawSql: "SELECT 1", datasourceUid: "abc" };

  it("includes the datasource host whenever it is configured", () => {
    const payload = buildAssistantContextPayload({
      ...base,
      datasourceHost: "cluster.example.com",
    });
    expect(payload.datasourceHost).toBe("cluster.example.com");
  });

  it("carries a resolved table with its schema and primary time column", () => {
    const payload = buildAssistantContextPayload({
      ...base,
      rawSql: "SELECT * FROM cicd.logs",
      table: { database: "cicd", table: "logs" },
      primaryTimeColumn: "timestamp",
      columns: [
        { name: "timestamp", type: "DateTime64" },
        { name: "message", type: "String" },
      ],
    });

    expect(payload.database).toBe("cicd");
    expect(payload.table).toBe("logs");
    expect(payload.primaryTimeColumn).toBe("timestamp");
    expect(payload.columns).toEqual([
      { name: "timestamp", type: "DateTime64" },
      { name: "message", type: "String" },
    ]);
  });

  it("omits unresolved fields rather than emitting empty placeholders", () => {
    const payload = buildAssistantContextPayload(base);

    expect(payload.sql).toBe("SELECT 1");
    expect("table" in payload).toBe(false);
    expect("database" in payload).toBe(false);
    expect("columns" in payload).toBe(false);
    expect("primaryTimeColumn" in payload).toBe(false);
  });

  it("omits columns when the fetch returned an empty list", () => {
    const payload = buildAssistantContextPayload({ ...base, columns: [] });
    expect("columns" in payload).toBe(false);
  });

  it("serialises the time range", () => {
    const payload = buildAssistantContextPayload({
      ...base,
      timeRange: range("2026-08-14T00:00:00Z", "2026-08-15T00:00:00Z"),
    });

    expect(payload.timeRange).toEqual({
      from: "2026-08-14T00:00:00.000Z",
      to: "2026-08-15T00:00:00.000Z",
    });
  });
});
