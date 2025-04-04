import { Context } from "./macrosService";
import { MacrosApplier } from "./macrosApplier";

const MACRO = "$__fromTime_ms";

export class FromTimeMsApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    if (!context.timeRange) {
      throw new Error("cannot apply macros without time range");
    }
    return rawQuery.replace(
      `${this.macroName()}()`,
      `fromUnixTimestamp64Milli(${context.timeRange.from.toDate().getTime()})`
    );
  }

  macroName(): string {
    return MACRO;
  }
}
