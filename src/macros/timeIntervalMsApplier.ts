import { Context } from "./macrosService";
import { MacrosApplier } from "./macrosApplier";

export const MACRO = "$__timeInterval_ms";

export class TimeIntervalMsApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let params = this.parseMacroArgs(rawQuery);
    if (params.length !== 1 || params[0] === "") {
      throw new Error("Macros $__timeInterval_ms should contain 1 parameter");
    }
    let param = params[0];
    rawQuery = rawQuery.replaceAll(
      `${MACRO}(${param})`,
      `toStartOfInterval(toDateTime64(${param}, 3), INTERVAL ${Math.max(
        context.intervalMs || 0,
        1
      )} millisecond)`
    );
    return rawQuery;
  }

  macroName(): string {
    return MACRO;
  }
}
