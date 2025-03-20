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
  sql: string,
  sqlTemplate: string
): string {
  const timeColumnRegexp = /\$__timeFilter\((\S*)\)/;
  let match = timeColumnRegexp.exec(sql);
  if (!match) {
    return "";
  }
  let timeColumn = match[1];
  // return `SELECT DISTINCT ${column} FROM ${getTable(sql)} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter() LIMIT 100`;
  // return `SELECT DISTINCT ${column}, COUNT(${column}) as count  FROM ${getTable(sql)} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter()  GROUP BY ${column} ORDER BY count DESC LIMIT 100`;
  return sqlTemplate
    .replaceAll("${column}", column)
    .replaceAll("${table}", getTable(sql))
    .replaceAll("${timeColumn}", timeColumn);
}
