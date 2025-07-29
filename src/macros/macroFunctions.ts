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
    if (context?.ast) {
      tableName = getFilterTableName("$__adHocFilter", context, index);
    }
    if (!tableName) {
      throw new Error(
        `Cannot apply ad hoc filters: unable to resolve tableName for ad hoc filter at index ${index}`
      );
    }

    let keys = await context.adHocFilter.keys(tableName);
    let columns = keys.map((key) => key.text);
    let typeByColumn = keys.reduce((acc, key) => {
      acc[key.text] = key.type;
      return acc;
    }, {} as { [key: string]: string });
    condition = context.adHocFilter.filters
      .filter((f) => columns.includes(f.key))
      .map((f) =>
        getFilterExpression(
          f,
          (typeByColumn[f.key] ?? "").toLowerCase().includes("string")
        )
      )
      .join(" AND ");
  }
  if (!condition) {
    condition = "1=1";
  }
  return condition;
};

export const getFilterExpression = (
  filter: AdHocVariableFilter,
  isString: boolean
): string => {
  const getJoinedValues = () => {
    const f = filter as any;
    const values = f?.values;
    return [
      [...values, values.find((v: any) => v === SYNTHETIC_EMPTY) ? "" : null]
        .filter((v) => v !== null)
        .filter((v) => v !== SYNTHETIC_NULL || isString)
        .map((v) => `$$$$${v}$$$$`)
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
    `${filter.value}`.toLowerCase() === "null" ||
    filter.value === SYNTHETIC_NULL
  ) {
    if (filter.operator === "=" && isString) {
      return `(${key} IS NULL OR ${key} = '${SYNTHETIC_NULL}')`;
    } else if (filter.operator === "!=" && isString) {
      return `${key} IS NOT NULL AND ${key} != '${SYNTHETIC_NULL}'`;
    } else if (filter.operator === "=") {
      return `${key} IS NULL`;
    } else if (filter.operator === "!=") {
      return `${key} IS NOT NULL`;
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
    return `toString(${key}) LIKE $$$$${prepareWildcardQuery(
      filter.value
    )}$$$$`;
  } else if (filter.operator === "!~") {
    return `toString(${key}) NOT LIKE $$$$${prepareWildcardQuery(
      filter.value
    )}$$$$`;
  } else {
    return `${key} ${filter.operator} $$$$${filter.value}$$$$`;
  }
};

export const prepareWildcardQuery = (v: string) =>
  v.replaceAll(/(?<!\\)\*/g, "%").replaceAll("\\*", "*");

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
  context: Context,
  index?: number
): Promise<string> => {
  if (params.length > 1) {
    throw new Error(
      "Macro $__timeFilter should not contain more than 1 parameter"
    );
  }
  let column;
  if (params.length !== 1 || params[0] === "") {
    column = await getFilterPK("$__timeFilter", context, index);
  } else {
    column = params[0];
  }

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
  context: Context,
  index?: number
): Promise<string> => {
  if (params.length > 1) {
    throw new Error(
      "Macro $__timeFilter_ms should not contain more than 1 parameter"
    );
  }
  let param;
  if (params.length !== 1 || params[0] === "") {
    param = await getFilterPK("$__timeFilter_ms", context, index);
  } else {
    param = params[0];
  }

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
  context: Context,
  index?: number
): Promise<string> => {
  if (params.length > 1) {
    throw new Error(
      "Macro $__timeInterval should not contain more than 1 parameter"
    );
  }
  let param;
  if (params.length !== 1 || params[0] === "") {
    param = await getValuePK("$__timeInterval", context, index);
  } else {
    param = params[0];
  }
  let interval = (context.intervalMs || 0) / 1000;
  return `toStartOfInterval(toDateTime(${param}), INTERVAL ${Math.max(
    interval,
    1
  )} second)`;
};

export const timeInterval_ms = async (
  params: string[],
  context: Context,
  index?: number
): Promise<string> => {
  if (params.length > 1) {
    throw new Error(
      "Macro $__timeInterval_ms should not contain more than 1 parameter"
    );
  }
  let param;
  if (params.length !== 1 || params[0] === "") {
    param = await getValuePK("$__timeInterval_ms", context, index);
  } else {
    param = params[0];
  }
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

const getFilterTableName = (macros: string, context: Context, index?: number) =>
  getTableName(macros, context, (n) => n.Where, index);

const getValueTableName = (macros: string, context: Context, index?: number) =>
  getTableName(macros, context, (n) => n.SelectItems, index);

export const getTableName = (
  macros: string,
  context: Context,
  macrosPlacement: (node: any) => boolean,
  index?: number
) => {
  let ast = context?.ast;
  if (!ast) {
    throw new Error("query ast is not provided");
  }
  let node = traverseTree(ast, (node) => {
    return (
      macrosPlacement(node) &&
      traverseTree(
        macrosPlacement(node),
        (n) => n.Name === macros && n.NamePos === index,
        (n) => macrosPlacement(n)
      )
    );
  });
  let tableNode = node?.From?.Expr?.Table;
  if (!tableNode) {
    throw new Error(`cannot find table for macro ${macros}`);
  }
  let start = tableNode?.TablePos;
  let end = tableNode?.TableEnd;
  let aliasNode = traverseTree(node, (node) => node.Alias);
  if (aliasNode?.Expr?.Table?.NameEnd) {
    end = aliasNode.Expr.Table.NameEnd;
  }
  return context.query.substring(start, end);
};

const getFilterPK = (macros: string, context: Context, index?: number) => {
  let table = getFilterTableName(macros, context, index);
  return getPK(macros, context, table);
};
const getValuePK = (macros: string, context: Context, index?: number) => {
  let table = getValueTableName(macros, context, index);
  return getPK(macros, context, table);
};

const getPK = (macros: string, context: Context, table: string) => {
  if (!table) {
    throw new Error(`Macro ${macros} could not get table name`);
  }
  const pk = context.pk(table);
  if (!pk) {
    throw new Error(`Macro ${macros} cannot find PK for table name ${table}`);
  }
  return pk;
};
