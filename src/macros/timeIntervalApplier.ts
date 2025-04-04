import { Context } from "./macrosService";
import { MacrosApplier } from "./macrosApplier";

export const MACRO = "$__timeInterval";

export class TimeIntervalApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let params = this.parseMacroArgs(rawQuery);
    if (params.length !== 1 || params[0] === "") {
      throw new Error("Macros $__timeInterval should contain 1 parameter");
    }
    let param = params[0];
    let interval = (context.intervalMs || 0) / 1000;
    rawQuery = rawQuery.replaceAll(
      `${MACRO}(${param})`,
      `toStartOfInterval(toDateTime(${param}), INTERVAL ${Math.max(
        interval,
        1
      )} second)`
    );
    return rawQuery;
  }

  macroName(): string {
    return MACRO;
  }
}
