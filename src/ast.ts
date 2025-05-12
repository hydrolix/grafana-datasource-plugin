import { AD_HOC_VALUE_QUERY } from "./constants";

export function getTable(sql: string): string {
  const tableRegex = /FROM\s+(\S*)\s/;
  let match = tableRegex.exec(sql);
  if (match) {
    return match[1];
  }
  return "";
}

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
