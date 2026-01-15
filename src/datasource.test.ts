import { firstValueFrom, of } from "rxjs";
import { DataQueryRequest, toDataFrame } from "@grafana/data";
import { setupDataSourceMock } from "__mocks__/datasource";
import { adHocTableVariable, fooVariable } from "./__mocks__/variable";
import { AdHocFilterKeys, HdxQuery } from "./types";

describe("HdxDataSource", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("When performing metricFindQuery", () => {
    beforeEach(() => {
      jest.clearAllMocks();
    });

    const cases: Array<{
      name: string;
      response: any;
      expected: any;
    }> = [
      {
        name: "it should return values",
        response: {
          fields: [{ name: "values", type: "number", values: [100, 200] }],
        },
        expected: [
          { text: 100, value: 100 },
          { text: 200, value: 200 },
        ],
      },
      {
        name: "it should return identified values",
        response: {
          fields: [
            { name: "ids", type: "number", values: [1, 2] },
            { name: "values", type: "number", values: [100, 200] },
          ],
        },
        expected: [
          { text: 100, value: 1 },
          { text: 200, value: 2 },
        ],
      },
    ];

    const { datasource, queryMock } = setupDataSourceMock({});

    test.each(cases)("$name", async ({ response, expected }) => {
      queryMock.mockImplementation((_) =>
        of({ data: [toDataFrame(response)] })
      );
      const actual = await datasource.metricFindQuery("mock", {});
      expect(actual).toEqual(expected);
    });
  });

  const filterQueryCases: Array<{ query: string; valid: boolean }> = [
    { query: "", valid: false },
    { query: "select 1;", valid: true },
  ];

  test.each(filterQueryCases)(
    "should filter out invalid query",
    ({ query, valid }) => {
      const { datasource } = setupDataSourceMock({});
      const actual = datasource.filterQuery({
        refId: "",
        rawSql: query,
        round: "",
        querySettings: {},
      });
      expect(actual).toEqual(valid);
    }
  );

  it("should interpolate variables in the query", async () => {
    const { datasource } = setupDataSourceMock({
      variables: [fooVariable],
    });
    const actual = datasource.applyTemplateVariables(
      {
        refId: "",
        rawSql: "foo $foo",
        round: "",
        querySettings: {},
      },
      {}
    );
    expect(actual.rawSql).toEqual("foo templatedFoo");
  });

  describe("ad hoc filtering", () => {
    beforeEach(() => {
      jest.clearAllMocks();
    });
    const { datasource, queryMock } = setupDataSourceMock({
      variables: [adHocTableVariable],
    });
    const getKeysMock = jest.spyOn(datasource.metadataProvider, "tableKeys");

    it("should return keys", async () => {
      let response = ["key1", "key2", "key3"].map(
        (k) => ({ text: k, value: k } as AdHocFilterKeys)
      );
      getKeysMock.mockReturnValue(Promise.resolve(response));
      let keys = await datasource.getTagKeys();

      expect(keys).toBe(response);
    });

    it("should not return values", async () => {
      let response = ["key1", "key2", "key3"].map(
        (k) => ({ text: k, value: k } as AdHocFilterKeys)
      );
      getKeysMock.mockReturnValue(Promise.resolve(response));
      let values = await datasource.getTagValues({ key: "key", filters: [] });

      expect(values).toEqual([]);
    });

    it("should  return values", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve(
          ["key1", "key2", "key3"].map(
            (k) => ({ text: k, value: k } as AdHocFilterKeys)
          )
        )
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: [100, 200] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([100, 200].map((k) => ({ text: k, value: k })));
    });

    it("should return null value", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve(
          ["key1", "key2", "key3"].map(
            (k) => ({ text: k, value: k } as AdHocFilterKeys)
          )
        )
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: [null] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([{ text: "__null__", value: "__null__" }]);
    });
    it("should return empty value", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve(
          ["key1", "key2", "key3"].map(
            (k) => ({ text: k, value: k } as AdHocFilterKeys)
          )
        )
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: [""] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([{ text: "__empty__", value: "__empty__" }]);
    });

    it("should return empty and synthetic value", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve(
          ["key1", "key2", "key3"].map(
            (k) => ({ text: k, value: k } as AdHocFilterKeys)
          )
        )
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: ["", "__empty__"] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([{ text: "__empty__", value: "__empty__" }]);
    });
    it("should return null and synthetic value", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve(
          ["key1", "key2", "key3"].map(
            (k) => ({ text: k, value: k } as AdHocFilterKeys)
          )
        )
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: [null, "__null__"] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([{ text: "__null__", value: "__null__" }]);
    });
    it("should return empty, null and both synthetic values", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve(
          ["key1", "key2", "key3"].map(
            (k) => ({ text: k, value: k } as AdHocFilterKeys)
          )
        )
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: [null, "__null__", "", "__empty__"] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([
        { text: "__empty__", value: "__empty__" },
        { text: "__null__", value: "__null__" },
      ]);
    });

    it("should use arrayJoin for array type columns", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve([
          { text: "key1", value: "key1", type: "Array(String)" },
          { text: "key2", value: "key2", type: "String" },
        ] as AdHocFilterKeys[])
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: ["tag1", "tag2", "tag3"] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([
        { text: "tag1", value: "tag1" },
        { text: "tag2", value: "tag2" },
        { text: "tag3", value: "tag3" },
      ]);
    });

    it("should not use arrayJoin for non-array type columns", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve([
          { text: "key1", value: "key1", type: "String" },
          { text: "key2", value: "key2", type: "Nullable(String)" },
        ] as AdHocFilterKeys[])
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: ["value1", "value2"] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "key1", filters: [] });

      expect(values).toEqual([
        { text: "value1", value: "value1" },
        { text: "value2", value: "value2" },
      ]);
    });

    it("should handle Array(Nullable(String)) type", async () => {
      getKeysMock.mockReturnValue(
        Promise.resolve([
          { text: "tags", value: "tags", type: "Array(Nullable(String))" },
        ] as AdHocFilterKeys[])
      );
      queryMock.mockReturnValue(
        of({
          data: [
            toDataFrame({
              fields: [{ values: ["prod", "staging", "dev"] }],
            }),
          ],
        })
      );
      let values = await datasource.getTagValues({ key: "tags", filters: [] });

      expect(values).toEqual([
        { text: "prod", value: "prod" },
        { text: "staging", value: "staging" },
        { text: "dev", value: "dev" },
      ]);
    });
  });

  it("should process error", async () => {
    const { datasource, queryMock } = setupDataSourceMock({});
    queryMock.mockReturnValue(
      of({ data: [], errors: [{ message: "error message", status: "error" }] })
    );
    const req = {
      targets: [{ rawSql: "select 1", refId: String(Math.random()) }],
    } as DataQueryRequest<HdxQuery>;
    let a = await firstValueFrom(datasource.query(req));
    expect(a.errors![0].message).toBe("error message");
  });
});
