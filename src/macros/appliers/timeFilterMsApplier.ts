import { MacrosApplier } from "../macrosApplier";
import { Context } from "macros/macrosService";

export const MACRO = "$__timeFilter_ms";

export class TimeFilterMsApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let params = this.parseMacroArgs(rawQuery);
    if (params.length !== 1 || params[0] === "") {
      throw new Error("Macros $__timeFilter_ms should contain 1 parameter");
    }
    let param = params[0];

    let phrase;
    if (context.timeRange) {
      phrase =
        `${param} >= fromUnixTimestamp64Milli(${context.timeRange.from
          .toDate()
          .getTime()}) ` +
        `AND ${param} <= fromUnixTimestamp64Milli(${context.timeRange.to
          .toDate()
          .getTime()})`;
    } else {
      phrase = "1=1";
    }

    return rawQuery.replaceAll(`${MACRO}(${param})`, phrase);
  }

  macroName(): string {
    return MACRO;
  }
}
