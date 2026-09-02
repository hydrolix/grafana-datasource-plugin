import { DataQueryResponse, toDataFrame } from "@grafana/data";
import { firstValueFrom, of } from "rxjs";
import { getKeyMap, getMetadataProvider, getQueryRunner } from "./metadataProvider";
import {
  MockDataSourceInstanceSettings,
  setupDataSourceMock,
} from "../__mocks__/datasource";
import { adHocTableVariable } from "../__mocks__/variable";
import {
  DESCRIBE1,
  DESCRIBE2,
  DESCRIBE_UUID_IP,
} from "../__mocks__/tableDescribes";
import {
  ARRAY_TYPES,
  SUPPORTED_TYPES,
  NULLABLE_TYPES,
  AD_HOC_PRELOAD_ROUND_INTERVAL,
  METADATA_QUERY_TIMEOUT_SETTING,
  METADATA_QUERY_TIMEOUT_VALUE,
} from "../constants";
import { getColumnKeysForMapStatement, getColumnValuesStatement } from "../ast";

const FUNCTIONS = ["widthBucket", "tupleConcat"];
const SCHEMAS = ["schema1", "schema2"];
const TABLES = ["table1", "table2"];
const COLUMNS = ["column1", "column2"];
const PK = "timefilter";
const KEY_RESPONSE = {
  fields: [
    {
      values: [
        "column1",
        "column2",
        "column3",
        "column4",
        "column5",
        "column6",
      ],
    },
    {
      values: [
        "String",
        "Nullable(String)",
        "Array(String)",
        "String",
        "String",
        "Array(Nullable(String))",
      ],
    },
    { values: ["", "", "", "ALIAS", "ALIAS", ""] },
    { values: ["", "", "", "`column`", "`column1`", ""] },
  ],
};

describe("ARRAY_TYPES constant", () => {
  test("should include arrays of supported types", () => {
    expect(ARRAY_TYPES).toContain("Array(String)");
    expect(ARRAY_TYPES).toContain("Array(UInt8)");
    expect(ARRAY_TYPES).toContain("Array(UInt16)");
    expect(ARRAY_TYPES).toContain("Array(UInt32)");
    expect(ARRAY_TYPES).toContain("Array(UInt64)");
    expect(ARRAY_TYPES).toContain("Array(Int8)");
    expect(ARRAY_TYPES).toContain("Array(Int16)");
    expect(ARRAY_TYPES).toContain("Array(Int32)");
    expect(ARRAY_TYPES).toContain("Array(Int64)");
    expect(ARRAY_TYPES).toContain("Array(Float32)");
    expect(ARRAY_TYPES).toContain("Array(Float64)");
    expect(ARRAY_TYPES).toContain("Array(UUID)");
    expect(ARRAY_TYPES).toContain("Array(IPv4)");
    expect(ARRAY_TYPES).toContain("Array(IPv6)");
  });

  test("should include arrays of nullable types", () => {
    expect(ARRAY_TYPES).toContain("Array(Nullable(String))");
    expect(ARRAY_TYPES).toContain("Array(Nullable(UInt32))");
    expect(ARRAY_TYPES).toContain("Array(Nullable(Int64))");
  });

  test("should have correct length", () => {
    const expectedLength = SUPPORTED_TYPES.length + NULLABLE_TYPES.length;
    expect(ARRAY_TYPES.length).toBe(expectedLength);
  });
});

describe("MetadataProvider", () => {
  const { datasource, queryMock } = setupDataSourceMock({
    variables: [adHocTableVariable],
  });
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test("get functions", async () => {
    let mdp = getMetadataProvider(datasource);
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: FUNCTIONS }] })],
      })
    );
    let actual = await mdp.functions();
    expect(actual.map((n) => n.id)).toEqual(FUNCTIONS);
  });

  test("get cached functions", async () => {
    let mdp = getMetadataProvider(datasource);
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: FUNCTIONS }] })],
      })
    );
    await mdp.functions();
    let actual = await mdp.functions();
    expect(actual.map((n) => n.id)).toEqual(FUNCTIONS);
    expect(queryMock).toHaveBeenCalledTimes(1);
  });

  test("get schemas", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: SCHEMAS }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    let actual = await mdp.schemas();
    expect(actual.map((n) => n.name)).toEqual(SCHEMAS);
  });

  test("get cached schemas", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: SCHEMAS }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    await mdp.schemas();
    let actual = await mdp.schemas();
    expect(actual.map((n) => n.name)).toEqual(SCHEMAS);
    expect(queryMock).toHaveBeenCalledTimes(1);
  });

  test("get tables", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: TABLES }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    let actual = await mdp.tables({ schema: "schema" });
    expect(actual.map((n) => n.name)).toEqual(TABLES);
  });

  test("get cached tables", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: TABLES }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    await mdp.tables({ schema: "schema" });
    let actual = await mdp.tables({ schema: "schema" });
    expect(actual.map((n) => n.name)).toEqual(TABLES);
    expect(queryMock).toHaveBeenCalledTimes(1);
  });

  test("get columns", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: COLUMNS }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    let actual = await mdp.columns({ schema: "schema", table: "table" });
    expect(actual.map((n) => n.name)).toEqual(COLUMNS);
  });

  test("get cached columns", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: COLUMNS }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    await mdp.columns({ schema: "schema", table: "table" });
    let actual = await mdp.columns({ schema: "schema", table: "table" });
    expect(actual.map((n) => n.name)).toEqual(COLUMNS);
    expect(queryMock).toHaveBeenCalledTimes(1);
  });

  test("get non cached", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: COLUMNS }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    await mdp.columns({ schema: "schema", table: "table" });
    await mdp.columns({ schema: "schema", table: "table2" });
    expect(queryMock).toHaveBeenCalledTimes(2);
  });

  test("get keys", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame(KEY_RESPONSE)],
      })
    );
    let mdp = getMetadataProvider(datasource);
    let actual = await mdp.tableKeys("table");
    expect(actual.map((n) => n)).toEqual([
      { text: "column1", type: "String", value: "column1" },
      { text: "column2", type: "Nullable(String)", value: "column2" },
      { text: "column3", type: "Array(String)", value: "column3" },
      { text: "column5", type: "String", value: "column5" },
      { text: "column6", type: "Array(Nullable(String))", value: "column6" },
    ]);
  });

  test("get cached keys", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame(KEY_RESPONSE)],
      })
    );
    let mdp = getMetadataProvider(datasource);
    await mdp.tableKeys("table");
    await mdp.tableKeys("table");
    expect(queryMock).toHaveBeenCalledTimes(1);
  });

  test("get pk", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: [PK] }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    let actual = await mdp.primaryKey({ schema: "schema", table: "table" });
    expect(actual).toEqual("timefilter");
  });

  test("get cached pk", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: [PK] }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    await mdp.primaryKey({ schema: "schema", table: "table" });
    await mdp.primaryKey({ schema: "schema", table: "table" });
    expect(queryMock).toHaveBeenCalledTimes(1);
  });

  // A falsy primary key is a real result, not a miss. Assistant republishes
  // context on a 300ms debounce, so treating "" or undefined as "not fetched
  // yet" issues a cluster query per keystroke.
  test("treats an empty pk as already-fetched", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: [""] }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    let first = await mdp.primaryKey({ schema: "schema", table: "table" });
    let second = await mdp.primaryKey({ schema: "schema", table: "table" });
    expect(first).toEqual("");
    expect(second).toEqual("");
    expect(queryMock).toHaveBeenCalledTimes(1);
  });

  test("treats a pk query returning no rows as already-fetched", async () => {
    queryMock.mockReturnValue(
      of({
        data: [toDataFrame({ fields: [{ values: [] }] })],
      })
    );
    let mdp = getMetadataProvider(datasource);
    let first = await mdp.primaryKey({ schema: "schema", table: "table" });
    let second = await mdp.primaryKey({ schema: "schema", table: "table" });
    expect(first).toBeUndefined();
    expect(second).toBeUndefined();
    expect(queryMock).toHaveBeenCalledTimes(1);
  });
});

describe("getKeyMap", () => {
  const cases = [
    {
      name: "summary describe",
      describe: DESCRIBE2,
      keys: [
        {
          text: "statusCode",
          type: "Nullable(UInt32)",
          value: "statusCode",
        },
        {
          text: "reqHost",
          type: "Nullable(String)",
          value: "reqHost",
        },
        {
          text: "city",
          type: "Nullable(String)",
          value: "city",
        },
        {
          text: "state",
          type: "Nullable(String)",
          value: "state",
        },
        {
          text: "country",
          type: "Nullable(String)",
          value: "country",
        },
        {
          text: "cacheable",
          type: "Nullable(UInt8)",
          value: "cacheable",
        },
        {
          text: "errorCode",
          type: "Nullable(String)",
          value: "errorCode",
        },
        {
          text: "reqMethod",
          type: "Nullable(String)",
          value: "reqMethod",
        },
        {
          text: "rspContentType",
          type: "Nullable(String)",
          value: "rspContentType",
        },
        {
          text: "proto",
          type: "Nullable(String)",
          value: "proto",
        },
        {
          text: "cacheStatus",
          type: "Nullable(UInt8)",
          value: "cacheStatus",
        },
        {
          text: "cp",
          type: "Nullable(UInt32)",
          value: "cp",
        },
        {
          text: "timestamp_min",
          type: "DateTime",
          value: "timestamp_min",
        },
      ],
    },
    {
      name: "summary with parsable alias",
      describe: DESCRIBE1,
      keys: [
        {
          text: "hour_ts",
          type: "DateTime",
          value: "hour_ts",
        },
        {
          text: "sampled_request_path",
          type: "Nullable(String)",
          value: "sampled_request_path",
        },
      ],
    },
    {
      name: "array types are included",
      describe: [
        {
          name: "tags",
          type: "Array(String)",
          default_type: "",
          default_expression: "",
        },
        {
          name: "categories",
          type: "Array(Nullable(String))",
          default_type: "",
          default_expression: "",
        },
        {
          name: "status",
          type: "String",
          default_type: "",
          default_expression: "",
        },
        {
          name: "func(column)",
          type: "String",
          default_type: "",
          default_expression: "",
        },
      ],
      keys: [
        {
          text: "tags",
          type: "Array(String)",
          value: "tags",
        },
        {
          text: "categories",
          type: "Array(Nullable(String))",
          value: "categories",
        },
        {
          text: "status",
          type: "String",
          value: "status",
        },
      ],
    },
    {
      name: "complex array types",
      describe: [
        {
          name: "ids",
          type: "Array(UInt32)",
          default_type: "",
          default_expression: "",
        },
        {
          name: "scores",
          type: "Array(Float64)",
          default_type: "",
          default_expression: "",
        },
        {
          name: "nested",
          type: "Array(Array(String))",
          default_type: "",
          default_expression: "",
        },
        {
          name: "simple",
          type: "UInt64",
          default_type: "",
          default_expression: "",
        },
      ],
      keys: [
        {
          text: "ids",
          type: "Array(UInt32)",
          value: "ids",
        },
        {
          text: "scores",
          type: "Array(Float64)",
          value: "scores",
        },
        {
          text: "simple",
          type: "UInt64",
          value: "simple",
        },
      ],
    },
    {
      name: "map types are included",
      describe: [
        {
          name: "labels",
          type: "Map(String, String)",
          default_type: "",
          default_expression: "",
        },
        {
          name: "metadata",
          type: "Map(String, Nullable(String))",
          default_type: "",
          default_expression: "",
        },
        {
          name: "counts",
          type: "Map(String, UInt32)",
          default_type: "",
          default_expression: "",
        },
        {
          name: "status",
          type: "String",
          default_type: "",
          default_expression: "",
        },
      ],
      keys: [
        {
          text: "labels",
          type: "Map(String, String)",
          value: "labels",
        },
        {
          text: "metadata",
          type: "Map(String, Nullable(String))",
          value: "metadata",
        },
        {
          text: "counts",
          type: "Map(String, UInt32)",
          value: "counts",
        },
        {
          text: "status",
          type: "String",
          value: "status",
        },
      ],
    },
    {
      name: "mixed array and map types",
      describe: [
        {
          name: "tags",
          type: "Array(String)",
          default_type: "",
          default_expression: "",
        },
        {
          name: "labels",
          type: "Map(String, String)",
          default_type: "",
          default_expression: "",
        },
        {
          name: "name",
          type: "String",
          default_type: "",
          default_expression: "",
        },
        {
          name: "scores",
          type: "Array(Float64)",
          default_type: "",
          default_expression: "",
        },
      ],
      keys: [
        {
          text: "tags",
          type: "Array(String)",
          value: "tags",
        },
        {
          text: "labels",
          type: "Map(String, String)",
          value: "labels",
        },
        {
          text: "name",
          type: "String",
          value: "name",
        },
        {
          text: "scores",
          type: "Array(Float64)",
          value: "scores",
        },
      ],
    },
    {
      name: "uuid and ip types are included",
      describe: DESCRIBE_UUID_IP,
      keys: [
        {
          text: "request_id",
          type: "UUID",
          value: "request_id",
        },
        {
          text: "session_id",
          type: "Nullable(UUID)",
          value: "session_id",
        },
        {
          text: "client_v4",
          type: "IPv4",
          value: "client_v4",
        },
        {
          text: "client_v6",
          type: "IPv6",
          value: "client_v6",
        },
      ],
    },
  ];
  it.each(cases)("$name", ({ describe, keys }) => {
    let response: DataQueryResponse = {
      data: [
        {
          fields: [
            { values: describe.map((d) => d.name) },
            { values: describe.map((d) => d.type) },
            { values: describe.map((d) => d.default_type) },
            { values: describe.map((d) => d.default_expression) },
          ],
        },
      ],
    };
    let result = getKeyMap(response);
    expect(result).toEqual(keys);
  });
});

describe("getQueryRunner guardrails", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("injects the execution-time breaker into every metadata query target", async () => {
    const { datasource, queryMock } = setupDataSourceMock({});
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(target.querySettings).toEqual([
      {
        setting: METADATA_QUERY_TIMEOUT_SETTING,
        value: METADATA_QUERY_TIMEOUT_VALUE,
      },
    ]);
  });

  it("never puts timeout_overflow_mode on the driver querySettings channel", async () => {
    const { datasource, queryMock } = setupDataSourceMock({});
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(
      target.querySettings.find(
        (s: any) => s.setting === "timeout_overflow_mode"
      )
    ).toBeUndefined();
  });

  it("sets round to 5m on metadata query targets", async () => {
    const { datasource, queryMock } = setupDataSourceMock({});
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(target.round).toBe(AD_HOC_PRELOAD_ROUND_INTERVAL);
  });

  it("a larger DS-level hdx_query_max_execution_time cannot loosen the breaker", async () => {
    const { datasource, queryMock } = setupDataSourceMock({
      customInstanceSettings: {
        ...MockDataSourceInstanceSettings,
        jsonData: {
          ...MockDataSourceInstanceSettings.jsonData,
          querySettings: [
            { setting: "hdx_query_max_execution_time", value: "60" },
          ],
        },
      },
    });
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(target.querySettings).toEqual([
      {
        setting: METADATA_QUERY_TIMEOUT_SETTING,
        value: METADATA_QUERY_TIMEOUT_VALUE,
      },
    ]);
  });

  it("a smaller DS-level max_execution_time alias tightens the breaker", async () => {
    const { datasource, queryMock } = setupDataSourceMock({
      customInstanceSettings: {
        ...MockDataSourceInstanceSettings,
        jsonData: {
          ...MockDataSourceInstanceSettings.jsonData,
          querySettings: [{ setting: "max_execution_time", value: "5" }],
        },
      },
    });
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(
      target.querySettings.find(
        (s: any) => s.setting === METADATA_QUERY_TIMEOUT_SETTING
      )
    ).toEqual({ setting: METADATA_QUERY_TIMEOUT_SETTING, value: "5" });
  });

  it("never adopts a DS-level 0 (unlimited)", async () => {
    const { datasource, queryMock } = setupDataSourceMock({
      customInstanceSettings: {
        ...MockDataSourceInstanceSettings,
        jsonData: {
          ...MockDataSourceInstanceSettings.jsonData,
          querySettings: [
            { setting: "hdx_query_max_execution_time", value: "0" },
          ],
        },
      },
    });
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(
      target.querySettings.find(
        (s: any) => s.setting === METADATA_QUERY_TIMEOUT_SETTING
      )
    ).toEqual({
      setting: METADATA_QUERY_TIMEOUT_SETTING,
      value: METADATA_QUERY_TIMEOUT_VALUE,
    });
  });

  it("ignores a non-numeric DS-level value", async () => {
    const { datasource, queryMock } = setupDataSourceMock({
      customInstanceSettings: {
        ...MockDataSourceInstanceSettings,
        jsonData: {
          ...MockDataSourceInstanceSettings.jsonData,
          querySettings: [
            { setting: "hdx_query_max_execution_time", value: "$timeout" },
          ],
        },
      },
    });
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(target.querySettings).toEqual([
      {
        setting: METADATA_QUERY_TIMEOUT_SETTING,
        value: METADATA_QUERY_TIMEOUT_VALUE,
      },
    ]);
  });

  it("does not let unrelated DS-level settings affect the breaker", async () => {
    const { datasource, queryMock } = setupDataSourceMock({
      customInstanceSettings: {
        ...MockDataSourceInstanceSettings,
        jsonData: {
          ...MockDataSourceInstanceSettings.jsonData,
          querySettings: [{ setting: "hdx_query_admin_comment", value: "x" }],
        },
      },
    });
    queryMock.mockReturnValue(of({ data: [] }));
    const runner = getQueryRunner(datasource);

    await firstValueFrom(runner("SELECT 1"));

    const target = queryMock.mock.calls[0][0].targets[0];
    expect(
      target.querySettings.find(
        (s: any) => s.setting === METADATA_QUERY_TIMEOUT_SETTING
      )
    ).toEqual({
      setting: METADATA_QUERY_TIMEOUT_SETTING,
      value: METADATA_QUERY_TIMEOUT_VALUE,
    });
  });

  it("renders the value-preload and map-key SQL with the break/timerange SETTINGS suffix", () => {
    const valueSql = getColumnValuesStatement(
      "clientIP",
      "sample.log",
      "ts",
      ""
    );
    const mapSql = getColumnKeysForMapStatement("attributes", "sample.log");

    [valueSql, mapSql].forEach((sql) => {
      expect(sql).toContain("SETTINGS timeout_overflow_mode = 'break'");
      expect(sql).toContain("hdx_query_max_timerange_sec = 87000");
    });
  });
});
