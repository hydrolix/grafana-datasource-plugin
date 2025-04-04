import { DateTimeFilterApplier } from "./dateTimeFilterApplier";

export const MACRO = "$__dt";

export class DTApplier extends DateTimeFilterApplier {
  macroName(): string {
    return MACRO;
  }
}
