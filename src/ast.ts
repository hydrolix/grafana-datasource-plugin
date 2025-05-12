import { AD_HOC_VALUE_QUERY } from "./constants";

export const traverseTree = (
  tree: any,
  predicate: (node: any) => boolean
): any => {
  if (predicate(tree)) {
    return tree;
  } else {
    for (const key in tree) {
      if (tree.hasOwnProperty(key) && tree[key]) {
        if (isObject(tree[key])) {
          const node = traverseTree(tree[key], predicate);
          if (node) {
            return node;
          }
        } else if (Array.isArray(tree[key])) {
          for (const el of tree[key]) {
            const node = traverseTree(el, predicate);
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
  timeColumn: string
): string {
  // return `SELECT DISTINCT ${column} FROM ${getTable(sql)} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter() LIMIT 100`;
  // return `SELECT DISTINCT ${column}, COUNT(${column}) as count  FROM ${getTable(sql)} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter()  GROUP BY ${column} ORDER BY count DESC LIMIT 100`;
  return AD_HOC_VALUE_QUERY.replaceAll("${column}", column)
    .replaceAll("${table}", table)
    .replaceAll("${timeColumn}", timeColumn);
}
