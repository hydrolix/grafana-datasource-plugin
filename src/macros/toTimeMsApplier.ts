import { Context } from "./macrosService";
import { MacrosApplier } from "./macrosApplier";

const MACRO = "$__toTime_ms";

export class ToTimeMsApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    if (!context.timeRange) {
      throw new Error("cannot apply macros without time range");
    }
    return rawQuery.replace(
      `${this.macroName()}()`,
      `fromUnixTimestamp64Milli(${context.timeRange.to.toDate().getTime()})`
    );
  }

  macroName(): string {
    return MACRO;
  }
}
