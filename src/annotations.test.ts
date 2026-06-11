import { AnnotationQuery, DataQueryRequest } from "@grafana/data";
import {
  getDefaultQuery,
  isAnnotationRequest,
  prepareQuery,
} from "./annotations";
import { HdxQuery } from "./types";

function annoWithSql(rawSql: string | undefined): AnnotationQuery<HdxQuery> {
  return {
    name: "anno",
    enable: true,
    iconColor: "red",
    target:
      rawSql === undefined
        ? (undefined as unknown as HdxQuery)
        : ({ refId: "Anno", rawSql, round: "", querySettings: [] } as HdxQuery),
  };
}

describe("prepareQuery", () => {
  it("returns undefined when target is missing", () => {
    const anno = { name: "x", enable: true, iconColor: "red" } as AnnotationQuery<HdxQuery>;
    expect(prepareQuery(anno)).toBeUndefined();
  });

  it("returns undefined when rawSql is empty string", () => {
    expect(prepareQuery(annoWithSql(""))).toBeUndefined();
  });

  it("returns undefined when rawSql is whitespace only", () => {
    expect(prepareQuery(annoWithSql("   \n\t  "))).toBeUndefined();
  });

  it("returns a shallow copy with source: 'annotation' on the happy path", () => {
    const anno = annoWithSql("SELECT 1");
    const out = prepareQuery(anno);

    expect(out).toBeDefined();
    expect(out!.source).toBe("annotation");
    expect(out!.rawSql).toBe("SELECT 1");
    expect(out).not.toBe(anno.target);
  });

  it("does not mutate the input target", () => {
    const anno = annoWithSql("SELECT 1");
    const before = { ...anno.target };
    prepareQuery(anno);
    expect(anno.target).toEqual(before);
    expect((anno.target as HdxQuery).source).toBeUndefined();
  });
});

describe("getDefaultQuery", () => {
  it("returns { source: 'annotation' } with no rawSql key", () => {
    const out = getDefaultQuery();
    expect(out.source).toBe("annotation");
    expect(Object.prototype.hasOwnProperty.call(out, "rawSql")).toBe(false);
  });
});

describe("isAnnotationRequest", () => {
  const baseTarget = (
    overrides: Partial<HdxQuery> = {}
  ): HdxQuery =>
    ({
      refId: "A",
      rawSql: "SELECT 1",
      round: "",
      querySettings: [],
      ...overrides,
    } as HdxQuery);

  function req(targets: HdxQuery[]): DataQueryRequest<HdxQuery> {
    return { targets } as unknown as DataQueryRequest<HdxQuery>;
  }

  it("returns true when any target has source === 'annotation'", () => {
    expect(
      isAnnotationRequest(req([baseTarget(), baseTarget({ source: "annotation" })]))
    ).toBe(true);
  });

  it("returns false when all targets lack the source field", () => {
    expect(isAnnotationRequest(req([baseTarget(), baseTarget()]))).toBe(false);
  });

  it("returns false for an empty targets array", () => {
    expect(isAnnotationRequest(req([]))).toBe(false);
  });
});
