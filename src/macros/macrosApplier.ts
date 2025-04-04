import { Context } from "./macrosService";

export abstract class MacrosApplier {
  abstract applyMacro(sql: string, context: Context): Promise<string>;

  abstract macroName(): string;

  async applyMacros(sql: string, context: Context): Promise<string> {
    if (!sql) {
      return sql;
    }
    while (sql.includes(`${this.macroName()}(`)) {
      sql = await this.applyMacro(sql, context);
    }
    return sql;
  }

  parseMacroArgs(query: string): string[] {
    let argsIndex =
      query.lastIndexOf(this.macroName()) + this.macroName().length;
    const args = [] as string[];
    const re = /[(),]/g;
    let bracketCount = 0;
    let lastArgEndIndex = 1;
    let regExpArray: RegExpExecArray | null;
    const argsSubstr = query.substring(argsIndex, query.length);
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
  }
}
