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
/**
 * Fills `${name}` slots with a function replacer: a string replacement goes
 * through JavaScript's GetSubstitution, where `$'`, `` $` ``, `$&` and `$$`
 * inside a user-authored condition or a map-key column like `attrs['$ref']`
 * would splice template text into the predicate.
 */
export function fillSlots(
  template: string,
  slots: Record<string, string>
): string {
  return Object.entries(slots).reduce(
    (sql, [name, value]) => sql.replaceAll("${" + name + "}", () => value),
    template
  );
}

/**
 * Ad-hoc value preload SQL. `timeColumn` is the table's primary key; the
 * empty string means the table declares none, so the time conjunct is left
 * out rather than interpolating an empty column name.
 */
export function getColumnValuesStatement(
  column: string,
  table: string,
  timeColumn: string,
  condition: string
): string {
  return fillSlots(AD_HOC_VALUE_QUERY, {
    column,
    table,
    timeFilter: timeColumn ? `$__timeFilter(${timeColumn}) AND ` : "",
    condition: condition ? `AND ${condition}` : "",
  });
}

/**
 * Map-key discovery SQL. The zero-argument `$__timeFilter()` is resolved to
 * the primary key on the backend, so `timeColumn` only decides whether the
 * conjunct is present. Only a positively resolved `""` selects the keyless
 * form; `undefined` (the frontend lookup failed) keeps the conjunct and lets
 * the backend resolve it from its own lookup, which is what this statement
 * did before the keyless form existed.
 */
export function getColumnKeysForMapStatement(
  column: string,
  table: string,
  timeColumn: string | undefined
): string {
  return fillSlots(AD_HOC_MAP_KEY_QUERY, {
    column,
    table,
    timeFilter: timeColumn === "" ? "" : "$__timeFilter() AND ",
  });
}
