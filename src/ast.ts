import { AD_HOC_MAP_KEY_QUERY, AD_HOC_VALUE_QUERY } from "./constants";

/**
 * Yields every node of a parser AST depth-first, parents before children,
 * descending through object values and array elements and stepping over
 * falsy ones. `skipPredicate` prunes a node and its whole subtree.
 *
 * Lazy on purpose: a consumer that wants only the first match stops the walk
 * by breaking out, so short-circuiting costs nothing, while a consumer that
 * wants every match drains it. That is the difference between `traverseTree`
 * and `collectTableRefs` in src/assistant/context.ts — both are strategies
 * over this one traversal rather than two hand-rolled recursions.
 */
export function* walkNodes(
  node: any,
  skipPredicate?: (node: any) => boolean
): Generator<any> {
  if (skipPredicate && skipPredicate(node)) {
    return;
  }
  yield node;
  for (const key in node) {
    if (node.hasOwnProperty(key) && node[key]) {
      if (isObject(node[key])) {
        yield* walkNodes(node[key], skipPredicate);
      } else if (Array.isArray(node[key])) {
        for (const el of node[key]) {
          yield* walkNodes(el, skipPredicate);
        }
      }
    }
  }
}

/**
 * The first node satisfying `predicate`, or undefined. Returns null when
 * `skipPredicate` rejects the root — preserved because callers rely only on
 * falsiness, and collapsing the two would be a silent behavior change.
 */
export const traverseTree = (
  tree: any,
  predicate: (node: any) => boolean,
  skipPredicate?: (node: any) => boolean
): any => {
  if (skipPredicate && skipPredicate(tree)) {
    return null;
  }
  for (const node of walkNodes(tree, skipPredicate)) {
    if (predicate(node)) {
      return node;
    }
  }
};

export const isObject = (value: any): boolean => {
  return typeof value === "object" && value !== null && !Array.isArray(value);
};
export function getColumnValuesStatement(
  column: string,
  table: string,
  timeColumn: string,
  condition: string
): string {
  return AD_HOC_VALUE_QUERY.replaceAll("${column}", column)
    .replaceAll("${table}", table)
    .replaceAll("${timeColumn}", timeColumn)
    .replaceAll("${condition}", condition ? `AND ${condition}` : "");
}
export function getColumnKeysForMapStatement(
  column: string,
  table: string
): string {
  return AD_HOC_MAP_KEY_QUERY.replaceAll("${column}", column).replaceAll(
    "${table}",
    table
  );
}
