import { AdHocFilterApplier } from "./adHocFilterApplier";
import { MetadataProvider } from "../editor/metadataProvider";
import { emptyContext } from "./macrosService";

describe("AdHocFilterApplier apply", () => {
  const SQL_WITH_FILTER =
    "SELECT column1, columnt2 FROM table WHERE $__adHocFilter()";
  const SQL_WITHOUT_FILTER = "SELECT column1, columnt2 FROM table";

  const TABLE_FN = (_: string) => "table";
  let metadataProvider: MetadataProvider;
  beforeEach(() => {
    metadataProvider = {
      tableKeys: (_: string) =>
        Promise.resolve([{ value: "column1" }, { value: "column2" }]),
    } as MetadataProvider;
  });
  test("apply to query without macros", async () => {
    const applier = new AdHocFilterApplier(metadataProvider, TABLE_FN);
    const actual = await applier.applyMacros(SQL_WITHOUT_FILTER, emptyContext);
    expect(actual).toEqual(SQL_WITHOUT_FILTER);
  });
  test("apply without filters", async () => {
    const applier = new AdHocFilterApplier(metadataProvider, TABLE_FN);
    const actual = await applier.applyMacros(SQL_WITH_FILTER, emptyContext);
    expect(actual).toEqual("SELECT column1, columnt2 FROM table WHERE 1=1");
  });
  test("apply with filter", async () => {
    const applier = new AdHocFilterApplier(metadataProvider, TABLE_FN);
    const actual = await applier.applyMacros(SQL_WITH_FILTER, {
      ...emptyContext,
      filters: [{ key: "column1", operator: "=", value: "value" }],
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = 'value'"
    );
  });
  test("apply with filters", async () => {
    const applier = new AdHocFilterApplier(metadataProvider, TABLE_FN);
    const actual = await applier.applyMacros(SQL_WITH_FILTER, {
      ...emptyContext,
      filters: [
        { key: "column1", operator: "=", value: "value" },
        { key: "column2", operator: "<", value: "value2" },
      ],
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = 'value' AND column2 < 'value2'"
    );
  });
  test("apply and skip invalid column", async () => {
    const applier = new AdHocFilterApplier(metadataProvider, TABLE_FN);
    const actual = await applier.applyMacros(SQL_WITH_FILTER, {
      ...emptyContext,
      filters: [
        { key: "column1", operator: "=", value: "value" },
        { key: "column2", operator: "<", value: "value2" },
        { key: "column3", operator: "=", value: "value3" },
      ],
    });
    expect(actual).toEqual(
      "SELECT column1, columnt2 FROM table WHERE column1 = 'value' AND column2 < 'value2'"
    );
  });
  test("apply with all invalid columns", async () => {
    const applier = new AdHocFilterApplier(metadataProvider, TABLE_FN);
    const actual = await applier.applyMacros(SQL_WITH_FILTER, {
      ...emptyContext,
      filters: [
        { key: "column4", operator: "=", value: "value" },
        { key: "column5", operator: "<", value: "value2" },
        { key: "column3", operator: "=", value: "value3" },
      ],
    });
    expect(actual).toEqual("SELECT column1, columnt2 FROM table WHERE 1=1");
  });
});

describe("AdHocFilterApplier getFilterExpression", () => {
  const MD_PROVIDER = {} as MetadataProvider;
  const TABLE_FN = (_: string) => "table";
  test("eq expression", async () => {
    const applier = new AdHocFilterApplier(MD_PROVIDER, TABLE_FN);
    const actual = applier.getFilterExpression({
      key: "column",
      operator: "=",
      value: "value",
    });
    expect(actual).toEqual("column = 'value'");
  });
  test("lg expression", async () => {
    const applier = new AdHocFilterApplier(MD_PROVIDER, TABLE_FN);
    const actual = applier.getFilterExpression({
      key: "column",
      operator: "<",
      value: "value",
    });
    expect(actual).toEqual("column < 'value'");
  });

  test("eq null expression", async () => {
    const applier = new AdHocFilterApplier(MD_PROVIDER, TABLE_FN);
    const actual = applier.getFilterExpression({
      key: "column",
      operator: "=",
      value: "null",
    });
    expect(actual).toEqual("column IS NULL");
  });

  test("neq null expression", async () => {
    const applier = new AdHocFilterApplier(MD_PROVIDER, TABLE_FN);
    const actual = applier.getFilterExpression({
      key: "column",
      operator: "!=",
      value: "null",
    });
    expect(actual).toEqual("column IS NOT NULL");
  });

  test("lg null expression", async () => {
    const applier = new AdHocFilterApplier(MD_PROVIDER, TABLE_FN);
    const actual = () =>
      applier.getFilterExpression({
        key: "column",
        operator: "<",
        value: "null",
      });
    expect(actual).toThrow(
      "column: operator '<' can not be applied to NULL value"
    );
  });

  test("eq regex expression", async () => {
    const applier = new AdHocFilterApplier(MD_PROVIDER, TABLE_FN);
    const actual = applier.getFilterExpression({
      key: "column",
      operator: "=~",
      value: "REGEX",
    });
    expect(actual).toEqual("toString(column) LIKE 'REGEX'");
  });

  test("neq regex expression", async () => {
    const applier = new AdHocFilterApplier(MD_PROVIDER, TABLE_FN);
    const actual = applier.getFilterExpression({
      key: "column",
      operator: "!~",
      value: "REGEX",
    });
    expect(actual).toEqual("toString(column) NOT LIKE 'REGEX'");
  });
});
