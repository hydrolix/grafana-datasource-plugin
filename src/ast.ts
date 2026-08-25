import { AD_HOC_MAP_KEY_QUERY, AD_HOC_VALUE_QUERY } from "./constants";

export const traverseTree = (
  tree: any,
  predicate: (node: any) => boolean,
  skipPredicate?: (node: any) => boolean
): any => {
  if (skipPredicate && skipPredicate(tree)) {
    return null;
  } else if (predicate(tree)) {
    return tree;
  } else {
    for (const key in tree) {
      if (tree.hasOwnProperty(key) && tree[key]) {
        if (isObject(tree[key])) {
          const node = traverseTree(tree[key], predicate, skipPredicate);
          if (node) {
            return node;
          }
        } else if (Array.isArray(tree[key])) {
          for (const el of tree[key]) {
            const node = traverseTree(el, predicate, skipPredicate);
            if (node) {
              return node;
            }
          }
        }
      }
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
