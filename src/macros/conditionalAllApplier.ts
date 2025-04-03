import { VARIABLE_REGEX } from "../constants";
import { Context, MacrosApplier } from "./macrosService";

export class ConditionalAllApplier implements MacrosApplier {
  async applyMacros(rawQuery: string, context: Context): Promise<string> {
    if (!rawQuery) {
      return rawQuery;
    }
    const macro = "$__conditionalAll(";
    let macroIndex = rawQuery.lastIndexOf(macro);

    while (macroIndex !== -1) {
      const params = this.parseMacroArgs(
        rawQuery,
        macroIndex + macro.length - 1
      );
      if (params.length !== 2) {
        return rawQuery;
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
      rawQuery = rawQuery.replace(`${macro}${params[0]},${params[1]})`, phrase);
      macroIndex = rawQuery.lastIndexOf(macro);
    }
    return rawQuery;
  }

  private parseMacroArgs(query: string, argsIndex: number): string[] {
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
