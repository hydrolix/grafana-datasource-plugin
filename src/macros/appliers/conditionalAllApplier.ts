import { MacrosApplier } from "../macrosApplier";
import { VARIABLE_REGEX } from "../../constants";
import { Context } from "macros/macrosService";

export class ConditionalAllApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    const params = this.parseMacroArgs(rawQuery);
    if (params.length !== 2) {
      throw new Error("Macros $__conditionalAll should contain 2 parameters");
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
    return rawQuery.replace(
      `${this.macroName()}(${params[0]},${params[1]})`,
      phrase
    );
  }

  macroName(): string {
    return "$__conditionalAll";
  }
}
