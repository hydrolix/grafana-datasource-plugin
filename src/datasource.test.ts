import { firstValueFrom, of } from "rxjs";
import { DataQueryRequest, toDataFrame } from "@grafana/data";
import { setupDataSourceMock } from "__mocks__/datasource";
import {
  adHocTableVariable,
  adHocTimeColumnVariable,
  fooVariable,
} from "./__mocks__/variable";
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
      },
      {}
    );
    expect(actual.rawSql).toEqual("foo templatedFoo");
  });

  describe("ad-hoc filtering", () => {
    beforeEach(() => {
      jest.clearAllMocks();
    });
    const { datasource, queryMock } = setupDataSourceMock({
      variables: [adHocTableVariable, adHocTimeColumnVariable],
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

      expect(values).toEqual([{ text: "null", value: null }]);
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
