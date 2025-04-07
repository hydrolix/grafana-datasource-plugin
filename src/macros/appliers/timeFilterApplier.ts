import { MacrosApplier } from "../macrosApplier";
import { Context } from "macros/macrosService";
import { TimeRange } from "@grafana/data";

export const MACRO = "$__timeFilter";

export class TimeFilterApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let params = this.parseMacroArgs(rawQuery);
    if (params.length !== 1 || params[0] === "") {
      throw new Error("Macros $__timeFilter should contain 1 parameter");
    }
    let param = params[0];

    let phrase;
    if (context.timeRange) {
      phrase = this.generateCondition(param, context.timeRange);
    } else {
      phrase = "1=1";
    }

    return rawQuery.replaceAll(`${MACRO}(${param})`, phrase);
  }

  macroName(): string {
    return MACRO;
  }

  generateCondition(column: string, timeRange: TimeRange): string {
    return (
      `${column} >= toDateTime(${timeRange.from.toDate().getTime() / 1000}) ` +
      `AND ${column} <= toDateTime(${timeRange.to.toDate().getTime() / 1000})`
    );
  }
}
