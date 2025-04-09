import {
  AdHocVariableFilter,
  dateTime,
  TimeRange,
  TypedVariableModel,
} from "@grafana/data";
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

const macroFunctions: {
  [macro: string]: (
    params: string[],
    context: Context
  ) => Promise<string> | string;
} = {
  adHocFilter,
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

export const applyMacros = async (sql: string, context: Context) => {
  return await Object.keys(macroFunctions)
    .sort((x, y) => y.length - x.length)
    .reduce(
      async (sqlPromise, macroName) =>
        await applyMacro(await sqlPromise, macroName, context),
      Promise.resolve(sql)
    );
};

const applyMacro = async (sql: string, macroName: string, context: Context) => {
  let macrosRe = new RegExp(`\\$__${macroName}(?:\\b\\(\\s*\\)|\\b)`);
  while (macrosRe.test(sql)) {
    let match = macrosRe.exec(sql);
    if (!match) {
      break;
    }
    let argsIndex = match.index + `$__${macroName}`.length;
    let params = parseMacroArgs(sql, argsIndex);
    let hasParams = params && params.length > 0 && params[0] !== "";
    let phrase = await macroFunctions[macroName](params, context);
    if (hasParams) {
      sql = sql.replace(`${match[0]}(${params.join(",")})`, phrase);
    } else {
      sql = sql.replace(match[0], phrase);
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

export interface Context {
  adHocFilter?: AdHocFilterContext;
  templateVars: TypedVariableModel[];
  replaceFn: (s: string) => string;

  intervalMs?: number;
  timeRange?: TimeRange;
}

interface AdHocFilterContext {
  filters?: AdHocVariableFilter[];
  keys: () => Promise<string[]>;
}

export const emptyContext: Context = {
  templateVars: [],
  replaceFn: (s) => s,

  timeRange: {
    from: dateTime().subtract("5m"),
    to: dateTime(),
    raw: { from: "now-5m", to: "now" },
  },
};
