import { MacrosApplier } from "../macrosApplier";
import { Context } from "macros/macrosService";
import { DateFilterApplier } from "./dateFilterApplier";
import { TimeFilterApplier } from "./timeFilterApplier";

export const MACRO = "$__dateTimeFilter";

export class DateTimeFilterApplier extends MacrosApplier {
  private dateFilterApplier = new DateFilterApplier();
  private timeFilterApplier = new TimeFilterApplier();
  async applyMacro(rawQuery: string, context: Context): Promise<string> {
    let params = this.parseMacroArgs(rawQuery);
    if (params.length !== 2) {
      throw new Error("Macros $__dateTimeFilter should contain 2 parameters");
    }
    let dateColumn = params[0];
    let timeColumn = params[1];

    let phrase;
    if (context.timeRange) {
      phrase = `${this.dateFilterApplier.generateCondition(
        dateColumn,
        context.timeRange
      )} AND ${this.timeFilterApplier.generateCondition(
        timeColumn,
        context.timeRange
      )}`;
    } else {
      phrase = "1=1";
    }

    return rawQuery.replaceAll(
      `${this.macroName()}(${dateColumn},${timeColumn})`,
      phrase
    );
  }

  macroName(): string {
    return MACRO;
  }
}
