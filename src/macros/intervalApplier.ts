import { Context } from "./macrosService";
import { MacrosApplier } from "./macrosApplier";

const MACRO = "$__interval_s";

export class IntervalSApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let interval = (context.intervalMs || 0) / 1000;
    return rawQuery.replaceAll(`${MACRO}()`, `${Math.max(interval, 1)}`);
  }

  macroName(): string {
    return MACRO;
  }
}
