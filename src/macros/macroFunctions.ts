import { AdHocVariableFilter } from "@grafana/data";
import { Context } from "macros/macrosApplier";
import { DATE_FORMAT, VARIABLE_REGEX } from "../constants";
import { traverseTree } from "../ast";

export const adHocFilter = async (
  _: string[],
  context: Context,
  index?: number
): Promise<string> => {
  let condition;
  let tableName;
  if (context.adHocFilter?.ast) {
    let ast = context.adHocFilter?.ast;
    let node = traverseTree(ast, (node) => {
      return (
        node.Where &&
        traverseTree(
          node.Where,
          (n) => n.Name === "$__adHocFilter" && n.NamePos === index
        )
      );
    });
    let tableNode = node?.From?.Expr?.Table;
    let start = tableNode?.TablePos;
    let end = tableNode?.TableEnd;
    tableName = context.query.substring(start, end);
    console.log(`found ${tableName} at index ${index}`);
  }

  if (context.adHocFilter?.filters?.length && tableName) {
    let keys = await context.adHocFilter.keys(tableName);
    condition = context.adHocFilter.filters
      .filter((f) => keys.includes(f.key))
      .map(getFilterExpression)
      .join(" AND ");
  }
  if (!condition) {
    condition = "1=1";
  }
  return condition;
};
export const getFilterExpression = (filter: AdHocVariableFilter): string => {
  let key = filter.key;
  if (filter.value === "null") {
    if (filter.operator === "=") {
      return `${key} IS NULL`;
    } else if (filter.operator === "!=") {
      return `${key} IS NOT NULL`;
    } else {
      throw new Error(
        `${key}: operator '${filter.operator}' can not be applied to NULL value`
      );
    }
  } else if (filter.operator === "=~") {
    return `toString(${key}) LIKE '${filter.value}'`;
  } else if (filter.operator === "!~") {
    return `toString(${key}) NOT LIKE '${filter.value}'`;
  } else {
    return `${key} ${filter.operator} '${filter.value}'`;
  }
};

export const conditionalAll = async (
  params: string[],
  context: Context
): Promise<string> => {
  if (params.length !== 2) {
    throw new Error("Macro $__conditionalAll should contain 2 parameters");
  }
  const templateVarParam = params[1].trim();

  const templateVar = VARIABLE_REGEX.exec(templateVarParam);
  let phrase = params[0];
  if (templateVar) {
    const key = context.templateVars.find(
      (x) => x.name === templateVar[0]
    ) as any;
    let value = key?.current.value.toString();
    if (value === "" || value === "$__all") {
      phrase = "1=1";
    }
  }
  return phrase;
};

export const dateFilter = async (
  params: string[],
  context: Context
): Promise<string> => {
  if (params.length !== 1 || params[0] === "") {
    throw new Error("Macro $__dateFilter should contain 1 parameter");
  }
  let column = params[0];

  let phrase;
  if (context.timeRange) {
    phrase =
      `${column} >= toDate('${context.timeRange.from.format(DATE_FORMAT)}') ` +
      `AND ${column} <= toDate('${context.timeRange.to.format(DATE_FORMAT)}')`;
  } else {
    phrase = "1=1";
  }

  return phrase;
};

export const dateTimeFilter = async (
  params: string[],
  context: Context
): Promise<string> => {
  if (params.length !== 2) {
    throw new Error("Macro $__dateTimeFilter should contain 2 parameters");
  }
  let dateColumn = params[0];
  let timeColumn = params[1];

  let phrase;
  if (context.timeRange) {
    phrase = `${await dateFilter([dateColumn], context)} AND ${await timeFilter(
      [timeColumn],
      context
    )}`;
  } else {
    phrase = "1=1";
  }

  return phrase;
};

export const fromTime = async (
  _: string[],
  context: Context
): Promise<string> => {
  if (!context.timeRange) {
    throw new Error("cannot apply macro without time range");
  }
  return `toDateTime(${context.timeRange.from.toDate().getTime() / 1000})`;
};

export const fromTime_ms = async (
  _: string[],
  context: Context
): Promise<string> => {
  if (!context.timeRange) {
    throw new Error("cannot apply macro without time range");
  }
  return `fromUnixTimestamp64Milli(${context.timeRange.from
    .toDate()
    .getTime()})`;
};

export const interval_s = async (
  _: string[],
  context: Context
): Promise<string> => {
  let interval = (context.intervalMs || 0) / 1000;
  return `${Math.max(interval, 1)}`;
};

export const timeFilter = async (
  params: string[],
  context: Context
): Promise<string> => {
  if (params.length !== 1 || params[0] === "") {
    throw new Error("Macro $__timeFilter should contain 1 parameter");
  }
  let column = params[0];

  let phrase;
  if (context.timeRange) {
    phrase =
      `${column} >= toDateTime(${
        context.timeRange.from.toDate().getTime() / 1000
      }) ` +
      `AND ${column} <= toDateTime(${
        context.timeRange.to.toDate().getTime() / 1000
      })`;
  } else {
    phrase = "1=1";
  }

  return phrase;
};

export const timeFilter_ms = async (
  params: string[],
  context: Context
): Promise<string> => {
  if (params.length !== 1 || params[0] === "") {
    throw new Error("Macro $__timeFilter_ms should contain 1 parameter");
  }
  let param = params[0];

  let phrase;
  if (context.timeRange) {
    phrase =
      `${param} >= fromUnixTimestamp64Milli(${context.timeRange.from
        .toDate()
        .getTime()}) ` +
      `AND ${param} <= fromUnixTimestamp64Milli(${context.timeRange.to
        .toDate()
        .getTime()})`;
  } else {
    phrase = "1=1";
  }

  return phrase;
};

export const timeInterval = async (
  params: string[],
  context: Context
): Promise<string> => {
  if (params.length !== 1 || params[0] === "") {
    throw new Error("Macro $__timeInterval should contain 1 parameter");
  }
  let param = params[0];
  let interval = (context.intervalMs || 0) / 1000;
  return `toStartOfInterval(toDateTime(${param}), INTERVAL ${Math.max(
    interval,
    1
  )} second)`;
};

export const timeInterval_ms = async (
  params: string[],
  context: Context
): Promise<string> => {
  if (params.length !== 1 || params[0] === "") {
    throw new Error("Macro $__timeInterval_ms should contain 1 parameter");
  }
  let param = params[0];
  return `toStartOfInterval(toDateTime64(${param}, 3), INTERVAL ${Math.max(
    context.intervalMs || 0,
    1
  )} millisecond)`;
};

export const toTime = async (
  _: string[],
  context: Context
): Promise<string> => {
  if (!context.timeRange) {
    throw new Error("cannot apply macro without time range");
  }
  return `toDateTime(${context.timeRange.to.toDate().getTime() / 1000})`;
};

export const toTime_ms = async (
  _: string[],
  context: Context
): Promise<string> => {
  if (!context.timeRange) {
    throw new Error("cannot apply macro without time range");
  }
  return `fromUnixTimestamp64Milli(${context.timeRange.to.toDate().getTime()})`;
};
