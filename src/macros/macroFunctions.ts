import { VARIABLE_REGEX } from "../constants";
import { Context } from "types";

export const conditionalAll = (params: string[], context: Context): string => {
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
