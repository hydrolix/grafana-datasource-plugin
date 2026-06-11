import { AnnotationQuery, DataQueryRequest } from "@grafana/data";
import { HdxQuery } from "./types";

export function prepareQuery(
  anno: AnnotationQuery<HdxQuery>
): HdxQuery | undefined {
  const sql = anno.target?.rawSql?.trim();
  if (!sql) {
    return undefined;
  }
  return { ...anno.target, source: "annotation" } as HdxQuery;
}

export function getDefaultQuery(): Partial<HdxQuery> {
  return { source: "annotation" };
}

export function isAnnotationRequest(
  request: DataQueryRequest<HdxQuery>
): boolean {
  return request.targets.some((t) => t.source === "annotation");
}
