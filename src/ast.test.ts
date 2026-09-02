import { getColumnValuesStatement, traverseTree, walkNodes } from "./ast";

describe("walkNodes", () => {
  test("yields parents before their children", () => {
    const tree = { id: "root", child: { id: "leaf" } };
    expect([...walkNodes(tree)].map((n) => n.id)).toEqual(["root", "leaf"]);
  });

  // The /ast resource returns a top-level array of statements, so the walk
  // has to descend into one to find anything at all.
  test("descends into a top-level array", () => {
    const target = { id: "stmt" };
    expect([...walkNodes([target])]).toContain(target);
  });

  test("stops walking when the consumer stops asking", () => {
    let visited = 0;
    const counted = (id: string) => ({
      get id() {
        visited++;
        return id;
      },
    });
    const tree = { a: counted("a"), b: counted("b"), c: counted("c") };
    for (const node of walkNodes(tree)) {
      if (node.id === "a") {
        break;
      }
    }
    // The root has no id getter; stopping at "a" must leave b and c untouched.
    expect(visited).toBe(1);
  });
});

// Characterization tests: traverseTree drives the "no WHERE clause" query
// warning and had no coverage of its own. Written to pin current behavior
// before it was refactored onto the shared walker, so a semantic drift in
// traversal order, pruning, or the two falsy return values shows up here.
describe("traverseTree", () => {
  const isMatch = (node: any) => Boolean(node?.match);

  test("returns the root when the root matches", () => {
    const tree = { match: true, child: { match: true, id: "nested" } };
    expect(traverseTree(tree, isMatch)).toBe(tree);
  });

  test("finds a node nested behind object keys", () => {
    const target = { match: true, id: "deep" };
    expect(traverseTree({ a: { b: { c: target } } }, isMatch)).toBe(target);
  });

  test("finds a node inside an array value", () => {
    const target = { match: true, id: "in-array" };
    expect(traverseTree({ items: [{ id: "x" }, target] }, isMatch)).toBe(target);
  });

  test("returns the first match in traversal order", () => {
    const first = { match: true, id: "first" };
    const second = { match: true, id: "second" };
    expect(traverseTree({ a: first, b: second }, isMatch)).toBe(first);
  });

  test("returns undefined when nothing matches", () => {
    expect(traverseTree({ a: { b: 1 } }, isMatch)).toBeUndefined();
  });

  test("returns null when the skip predicate rejects the root", () => {
    const tree = { match: true };
    expect(traverseTree(tree, isMatch, () => true)).toBeNull();
  });

  test("does not descend into a subtree the skip predicate prunes", () => {
    const pruned = { pruned: true, child: { match: true, id: "hidden" } };
    const reachable = { match: true, id: "reachable" };
    const tree = { a: pruned, b: reachable };
    expect(traverseTree(tree, isMatch, (n) => Boolean(n?.pruned))).toBe(
      reachable
    );
  });

  test("walks past null and undefined values without throwing", () => {
    const target = { match: true, id: "after-nulls" };
    const tree = { a: null, b: undefined, c: 0, d: "", e: target };
    expect(traverseTree(tree, isMatch)).toBe(target);
  });
});

describe("ast getColumnValuesStatement", () => {
  test("should return topK statement for a plain column", () => {
    let result = getColumnValuesStatement("statusCode", "sample.log", "ts", "");
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(statusCode)) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should return topK statement for city for table with variables", () => {
    let result = getColumnValuesStatement("city", "sample.log", "ts", "");
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(city)) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should return topK statement for statusCode with condition", () => {
    let result = getColumnValuesStatement(
      "statusCode",
      "sample.log",
      "ts",
      "toString(statusCode) like '2%'"
    );
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(statusCode)) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter() AND toString(statusCode) like '2%' SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should use arrayJoin expansion for Array-typed columns", () => {
    let result = getColumnValuesStatement(
      "arrayJoin(tags)",
      "sample.log",
      "ts",
      ""
    );
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(arrayJoin(tags))) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should support map-access column expressions", () => {
    let result = getColumnValuesStatement(
      "attributes['env']",
      "sample.log",
      "ts",
      ""
    );
    expect(result).toBe(
      "SELECT arrayJoin(topK(100)(attributes['env'])) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000"
    );
  });

  test("should not contain GROUP BY or ORDER BY count", () => {
    let result = getColumnValuesStatement("clientIP", "sample.log", "ts", "");
    expect(result).not.toContain("GROUP BY");
    expect(result).not.toContain("ORDER BY count");
  });

  test("should carry both SETTINGS entries", () => {
    let result = getColumnValuesStatement("clientIP", "sample.log", "ts", "");
    expect(result).toContain("timeout_overflow_mode = 'break'");
    expect(result).toContain("hdx_query_max_timerange_sec = 87000");
  });
});
