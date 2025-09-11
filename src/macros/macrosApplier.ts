import { conditionalAll } from "./macroFunctions";
import { Context, MacroFunctionMap } from "../types";

export const applyConditionalAll = (sql: string, context: Context) =>
  applyMacros(sql, context, { conditionalAll });

const applyMacros = (
  sql: string,
  context: Context,
  macroFunctionMap: MacroFunctionMap
) => {
  const macroNames = Object.keys(macroFunctionMap)
    .map((v) => `\\$__(${v})`)
    .join("|");
  const macrosRe = new RegExp(
    `(${macroNames})(?:\\b\\(\\s*\\)|\\b)(?!(.|\\r\\n|\\r|\\n)*(${macroNames}))`
  );
  //
  while (macrosRe.test(sql)) {
    let match = macrosRe.exec(sql);
    if (!match || match.length <= 1) {
      break;
    }
    const macroName = match[1];
    let argsIndex = match.index + `${macroName}`.length;
    let params = parseMacroArgs(sql, argsIndex);
    let hasParams = params && params.length > 0 && params[0] !== "";
    let phrase = macroFunctionMap[macroName.replace("$__", "")](
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
  query: "",
};
