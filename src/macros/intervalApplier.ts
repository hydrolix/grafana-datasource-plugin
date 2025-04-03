import { Context, MacrosApplier } from "./macrosService";

export const MACRO = "$__interval_s()";

export class IntervalSApplier implements MacrosApplier {
  async applyMacros(rawQuery: string, context: Context): Promise<string> {
    if (!rawQuery) {
      return rawQuery;
    }

    let hasMacro = rawQuery.includes(MACRO);
    if (hasMacro) {
      let interval = (context.intervalMs || 0) / 1000;
      rawQuery = rawQuery.replaceAll(MACRO, `${Math.max(interval, 1)}`);
    }
    return rawQuery;
  }
}
