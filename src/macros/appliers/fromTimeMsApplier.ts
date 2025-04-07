import { MacrosApplier } from "../macrosApplier";
import { Context } from "macros/macrosService";

const MACRO = "$__fromTime_ms";

export class FromTimeMsApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    if (!context.timeRange) {
      throw new Error("cannot apply macros without time range");
    }
    return rawQuery.replace(
      this.macrosMatch(rawQuery),
      `fromUnixTimestamp64Milli(${context.timeRange.from.toDate().getTime()})`
    );
  }

  macroName(): string {
    return MACRO;
  }
}
