import { Context } from "./macrosService";
import { MacrosApplier } from "./macrosApplier";

const MACRO = "$__toTime";

export class ToTimeApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    if (!context.timeRange) {
      throw new Error("cannot apply macros without time range");
    }
    return rawQuery.replace(
      `${this.macroName()}()`,
      `toDateTime(${context.timeRange.to.toDate().getTime() / 1000})`
    );
  }

  macroName(): string {
    return MACRO;
  }
}
