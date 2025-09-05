import { AdHocVariableFilter } from "@grafana/data";
import { SYNTHETIC_EMPTY, SYNTHETIC_NULL, VARIABLE_REGEX } from "../constants";
import { Context } from "types";

export const adHocFilter = async (
  _: string[],
  context: Context,
  index?: number
): Promise<string> => {
  let condition;
  if (context.adHocFilter?.filters?.length) {
    let tableName;
    if (context?.macroCTE) {
      tableName = context.macroCTE.find((t) => t.macroPos === index)?.cte;
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
