import { dateTime } from "@grafana/data";
import {
  adHocFilter,
  conditionalAll,
  dateFilter,
  dateTimeFilter,
  fromTime,
  fromTime_ms,
  interval_s,
  timeFilter,
  timeFilter_ms,
  timeInterval,
  timeInterval_ms,
  toTime,
  toTime_ms,
} from "./macroFunctions";
import { Context, MacroFunctionMap } from "../types";

const macroFunctions: MacroFunctionMap = {
  conditionalAll,
  dateFilter,
  timeFilter,
  timeFilter_ms,
  fromTime,
  fromTime_ms,
  interval_s,
  timeInterval,
  timeInterval_ms,
  toTime,
  toTime_ms,
  dateTimeFilter,
  dt: dateTimeFilter,
};
export const applyBaseMacros = async (sql: string, context: Context) =>
  applyMacros(sql, context, macroFunctions);

export const applyAdHocMacro = async (sql: string, context: Context) =>
  applyMacros(sql, context, { adHocFilter });

const applyMacros = async (
  sql: string,
  context: Context,
  macroFunctionMap: MacroFunctionMap
) => {
  return await Object.keys(macroFunctionMap)
    .sort((x, y) => y.length - x.length)
    .reduce(
      async (sqlPromise, macroName) =>
        await applyMacro(
          await sqlPromise,
          macroName,
          context,
          macroFunctionMap
        ),
      Promise.resolve(sql)
    );
};

const applyMacro = async (
  sql: string,
  macroName: string,
  context: Context,
  macroFunctionMap: MacroFunctionMap
) => {
  let macrosRe = new RegExp(
    `\\$__${macroName}(?:\\b\\(\\s*\\)|\\b)(?!(.|\\r\\n|\\r|\\n)*${macroName})`
  );
  while (macrosRe.test(sql)) {
    let match = macrosRe.exec(sql);
    if (!match) {
      break;
    }
    let argsIndex = match.index + `$__${macroName}`.length;
    let params = parseMacroArgs(sql, argsIndex);
    let hasParams = params && params.length > 0 && params[0] !== "";
    let phrase = await macroFunctionMap[macroName](
      params,
      context,
      match.index
    );
    if (hasParams) {
      sql = sql.replace(`${match[0]}(${params.join(",")})`, phrase);
    } else {
      sql = sql.replace(macrosRe, phrase);
    }
  }
  return sql;
};

export const parseMacroArgs = (query: string, argsIndex: number): string[] => {
  const argsSubstr = query.substring(argsIndex, query.length);
  if (!/^\s*\(/.test(argsSubstr)) {
    return []; // if not start with (, means it doesn't have params
  }
  const args = [] as string[];
  const re = /[(),]/g;
  let bracketCount = 0;
  let lastArgEndIndex = 1;
  let regExpArray: RegExpExecArray | null;
  while ((regExpArray = re.exec(argsSubstr)) !== null) {
    const foundNode = regExpArray[0];
    if (foundNode === "(") {
      bracketCount++;
    } else if (foundNode === ")") {
      bracketCount--;
    }
    if (foundNode === "," && bracketCount === 1) {
      args.push(argsSubstr.substring(lastArgEndIndex, re.lastIndex - 1));
      lastArgEndIndex = re.lastIndex;
    }
    if (bracketCount === 0) {
      args.push(argsSubstr.substring(lastArgEndIndex, re.lastIndex - 1));
      return args;
    }
  }
  return [];
};

export const emptyContext: Context = {
  templateVars: [],
  replaceFn: (s) => s,
  query: "",
  timeRange: {
    from: dateTime().subtract("5m"),
    to: dateTime(),
    raw: { from: "now-5m", to: "now" },
  },
};
