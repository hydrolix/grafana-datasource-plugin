import { conditionalAll } from "./macroFunctions";
import { Context, MacroFunctionMap } from "../types";

export const applyConditionalAll = (sql: string, context: Context) =>
  applyMacros(sql, context, { conditionalAll });

interface MacroMatch {
  full: string;
  name: string;
  args: string[];
  escaped: boolean;
  pos: number;
  length: number;
}

const applyMacros = (
  sql: string,
  context: Context,
  macroFunctionMap: MacroFunctionMap
): string => {
  // Collect all macro matches from all macro types
  const allMatches: MacroMatch[] = [];

  for (const macroName of Object.keys(macroFunctionMap)) {
    const matches = getMacroMatches(sql, macroName);
    allMatches.push(...matches);
  }

  // Sort matches by position descending (process from right to left)
  // This ensures that string positions remain valid as we replace
  allMatches.sort((a, b) => b.pos - a.pos);

  // Process each match
  for (const match of allMatches) {
    if (!match.escaped) {
      // Apply the macro function
      const macroFunc = macroFunctionMap[match.name];
      const result = macroFunc(match.args, context, match.pos);

      // Replace the full macro text with the result
      sql =
        sql.substring(0, match.pos) +
        sql.substring(match.pos).replace(match.full, result);
    }
  }

  return sql;
};

/**
 * Extracts macro matches from the SQL string for a given macro name.
 * This matches the logic from interpolator.go's getMacroMatches function.
 */
const getMacroMatches = (input: string, name: string): MacroMatch[] => {
  // Regex pattern: \$+__macroName\b (one or more dollar signs followed by __macroName)
  // This matches both $__macroName and $$__macroName
  const pattern = new RegExp(`\\$+__${name}\\b`, "g");
  const matches: MacroMatch[] = [];

  let regexMatch: RegExpExecArray | null;
  while ((regexMatch = pattern.exec(input)) !== null) {
    const start = regexMatch.index;
    const end = start + regexMatch[0].length;

    // Parse arguments starting after the macro name
    const { args, length } = parseArgs(input.substring(end));

    if (length < 0) {
      throw new Error(
        "failed to parse macro arguments (missing close bracket?)"
      );
    }

    // Check if escaped by looking at the second character (index 1)
    // If it's a $, then the macro was escaped ($$__macroName)
    const escaped = input[start + 1] === "$";

    matches.push({
      full: input.substring(start, end + length),
      name: name,
      args: args,
      escaped: escaped,
      pos: start,
      length: end + length - start,
    });
  }

  return matches;
};

/**
 * Parses macro arguments from a string.
 * This matches the logic from interpolator.go's parseArgs function.
 *
 * @param argString - The string starting from after the macro name
 * @returns An object with args array and the length of the bracketed argument list
 */
const parseArgs = (argString: string): { args: string[]; length: number } => {
  if (!argString.startsWith("(")) {
    return { args: [], length: 0 }; // No arguments
  }

  const args: string[] = [];
  let depth = 0;
  let arg = "";

  for (let i = 0; i < argString.length; i++) {
    const char = argString[i];

    switch (char) {
      case "(":
        depth++;
        if (depth === 1) {
          // Don't include the outer bracket in the arg
          continue;
        }
        break;

      case ")":
        depth--;
        if (depth === 0) {
          // Closing bracket - we're done
          args.push(arg.trim());
          return { args, length: i + 1 };
        }
        break;

      case ",":
        if (depth === 1) {
          // A comma at this level is separating args
          args.push(arg.trim());
          arg = "";
          continue;
        }
        break;
    }

    arg += char;
  }

  // If we get here, we have seen an open bracket but not a close bracket
  return { args: [], length: -1 };
};

export const parseMacroArgs = (query: string, argsIndex: number): string[] => {
  const argsSubstr = query.substring(argsIndex);
  const { args, length } = parseArgs(argsSubstr);

  if (length < 0) {
    return [];
  }

  return args;
};

export const emptyContext: Context = {
  templateVars: [],
  query: "",
};
