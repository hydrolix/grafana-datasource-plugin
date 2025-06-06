import { AdHocVariableFilter } from "@grafana/data";
import {
  DATE_FORMAT,
  SYNTHETIC_EMPTY,
  SYNTHETIC_NULL,
  VARIABLE_REGEX,
} from "../constants";
import { traverseTree } from "../ast";
import { Context } from "types";

export const adHocFilter = async (
  _: string[],
  context: Context,
  index?: number
): Promise<string> => {
  let condition;
  if (context.adHocFilter?.filters?.length) {
    let tableName;
    if (context.adHocFilter?.ast) {
      let ast = context.adHocFilter?.ast;
      let node = traverseTree(ast, (node) => {
        return (
          node.Where &&
          traverseTree(
            node.Where,
            (n) => n.Name === "$__adHocFilter" && n.NamePos === index,
            (n) => n.Where
          )
        );
      });
      let tableNode = node?.From?.Expr?.Table;
      let start = tableNode?.TablePos;
      let end = tableNode?.TableEnd;
      tableName = context.query.substring(start, end);
    }
    if (!tableName) {
      throw new Error(
        `Cannot apply ad hoc filters: unable to resolve tableName for ad-hoc filter at index ${index}`
      );
    }

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
  const getJoinedValues = () => {
    // @ts-ignore
    const values = filter?.values;
    return [
      [...values, values.find((v: any) => v === SYNTHETIC_EMPTY) ? "" : null]
        .filter((v) => v !== null)
        .map((v) => `'${v}'`)
        ?.join(", "),
      values.find((v: any) => v === SYNTHETIC_NULL),
    ];
  };
  let key = filter.key;
  if (filter.operator === "=|") {
    const [joinedValues, hasNull] = getJoinedValues();
    const condition = [
      joinedValues ? `${key} IN (${joinedValues})` : "",
      hasNull ? `${key} IS NULL` : "",
    ]
      .filter((v) => v)
      .join(" OR ");
    return joinedValues && hasNull ? `(${condition})` : condition;
  } else if (filter.operator === "!=|") {
    const [joinedValues, hasNull] = getJoinedValues();
    return [
      joinedValues ? `${key} NOT IN (${joinedValues})` : "",
      hasNull ? `${key} IS NOT NULL` : "",
    ]
      .filter((v) => v)
      .join(" AND ");
  } else if (
    filter.value?.toLowerCase() === "null" ||
    filter.value === SYNTHETIC_NULL
  ) {
    if (filter.operator === "=") {
      return `(${key} IS NULL OR ${key} = '${SYNTHETIC_NULL}')`;
    } else if (filter.operator === "!=") {
      return `${key} IS NOT NULL AND ${key} != '${SYNTHETIC_NULL}'`;
    } else {
      throw new Error(
        `${key}: operator '${filter.operator}' can not be applied to NULL value`
      );
    }
  } else if (filter.value === "" || filter.value === SYNTHETIC_EMPTY) {
    if (filter.operator === "=") {
      return `(${key} = '' OR ${key} = '${SYNTHETIC_EMPTY}')`;
    } else if (filter.operator === "!=") {
      return `${key} != '' AND ${key} != '${SYNTHETIC_EMPTY}'`;
    } else {
      throw new Error(
        `${key}: operator '${filter.operator}' can not be applied to __empty__ value`
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
