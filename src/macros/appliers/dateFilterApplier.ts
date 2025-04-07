import { TimeRange } from "@grafana/data";
import { MacrosApplier } from "../macrosApplier";
import { Context } from "macros/macrosService";
import { DATE_FORMAT } from "../../constants";

export const MACRO = "$__dateFilter";

export class DateFilterApplier extends MacrosApplier {
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let params = this.parseMacroArgs(rawQuery);
    if (params.length !== 1 || params[0] === "") {
      throw new Error("Macros $__dateFilter should contain 1 parameter");
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
      `${column} >= toDate(${timeRange.from.format(DATE_FORMAT)}) ` +
      `AND ${column} <= toDate(${timeRange.to.format(DATE_FORMAT)})`
    );
  }
}
