import { DataQueryResponse, toDataFrame } from "@grafana/data";
import { of } from "rxjs";
import { getKeyMap, getMetadataProvider } from "./metadataProvider";
import { setupDataSourceMock } from "../__mocks__/datasource";
import { adHocTableVariable } from "../__mocks__/variable";
import { DESCRIBE1, DESCRIBE2 } from "../__mocks__/tableDescribes";

const FUNCTIONS = ["widthBucket", "tupleConcat"];
const SCHEMAS = ["schema1", "schema2"];
const TABLES = ["table1", "table2"];
const COLUMNS = ["column1", "column2"];
const PK = "timefilter";
const KEY_RESPONSE = {
  fields: [
    { values: ["column1", "column2", "column3", "column4", "column5"] },
    {
      values: ["String", "Nullable(String)", "Array<String>", "String"],
    },
    { values: ["", "", "", "ALIAS", "ALIAS"] },
    { values: ["", "", "", "`column`", "`column1`"] },
  ],
};

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
      { text: "column5", type: "String", value: "column5" },
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
