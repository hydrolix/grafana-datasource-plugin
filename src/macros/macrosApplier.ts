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

const baseMacroFunctions: MacroFunctionMap = {
  conditionalAll,
  dateFilter,
  fromTime,
  fromTime_ms,
  interval_s,
  toTime,
  toTime_ms,
  dateTimeFilter,
  dt: dateTimeFilter,
};
const astAwareMacroFunctions: MacroFunctionMap = {
  adHocFilter,
  timeFilter,
  timeFilter_ms,
  timeInterval,
  timeInterval_ms,
};
export const applyBaseMacros = async (sql: string, context: Context) =>
  applyMacros(sql, context, baseMacroFunctions);

export const applyAstAwareMacro = async (sql: string, context: Context) =>
  applyMacros(sql, context, astAwareMacroFunctions);

const applyMacros = async (
  sql: string,
  context: Context,
  macroFunctionMap: MacroFunctionMap
) => {
  const macroNames = Object.keys(macroFunctionMap).join("|");
  const macrosRe = new RegExp(
    `\\$__(${macroNames})(?:\\b\\(\\s*\\)|\\b)(?!(.|\\r\\n|\\r|\\n)*(${macroNames}))`
  );
  //
  while (macrosRe.test(sql)) {
    let match = macrosRe.exec(sql);
    if (!match || match.length <= 1) {
      break;
    }
    const macroName = match[1];
    let argsIndex = match.index + `$__${macroName}`.length;
    let params = parseMacroArgs(sql, argsIndex);
    let hasParams = params && params.length > 0 && params[0] !== "";
    let phrase = await macroFunctionMap[macroName](
      params,
      context,
      match.index
    );
    if (hasParams) {
      sql =
        sql.substring(0, match.index) +
        sql
          .substring(match.index)
          .replace(`${match[0]}(${params.join(",")})`, phrase);
    } else {
      sql =
        sql.substring(0, match.index) +
        sql.substring(match.index).replace(macrosRe, phrase);
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
  pk: (s) => Promise.resolve(s),
  query: "",
  timeRange: {
    from: dateTime().subtract("5m"),
    to: dateTime(),
    raw: { from: "now-5m", to: "now" },
  },
};
