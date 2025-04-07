import { MacrosApplier } from "../macrosApplier";
import { Context } from "macros/macrosService";

const MACRO = "$__interval_s";

export class IntervalSApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let interval = (context.intervalMs || 0) / 1000;
    return rawQuery.replaceAll(
      this.macrosMatch(rawQuery),
      `${Math.max(interval, 1)}`
    );
  }

  macroName(): string {
    return MACRO;
  }
}
