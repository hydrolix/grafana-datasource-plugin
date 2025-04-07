import { MacrosApplier } from "../macrosApplier";
import { Context } from "macros/macrosService";

const MACRO = "$__toTime_ms";

export class ToTimeMsApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    if (!context.timeRange) {
      throw new Error("cannot apply macros without time range");
    }
    return rawQuery.replace(
      this.macrosMatch(rawQuery),
      `fromUnixTimestamp64Milli(${context.timeRange.to.toDate().getTime()})`
    );
  }

  macroName(): string {
    return MACRO;
  }
}
