import { MacrosService } from "./macrosService";
import { ConditionalAllApplier } from "./appliers/conditionalAllApplier";
import { AdHocFilterApplier } from "./appliers/adHocFilterApplier";
import { IntervalSApplier } from "./appliers/intervalApplier";
import { TimeIntervalApplier } from "./appliers/timeIntervalApplier";
import { TimeIntervalMsApplier } from "./appliers/timeIntervalMsApplier";
import { TimeFilterApplier } from "./appliers/timeFilterApplier";
import { TimeFilterMsApplier } from "./appliers/timeFilterMsApplier";
import { DateFilterApplier } from "./appliers/dateFilterApplier";
import { DateTimeFilterApplier } from "./appliers/dateTimeFilterApplier";
import { DTApplier } from "./appliers/dtApplier";
import { ToTimeApplier } from "./appliers/toTimeApplier";
import { ToTimeMsApplier } from "./appliers/toTimeMsApplier";
import { FromTimeApplier } from "./appliers/fromTimeApplier";
import { FromTimeMsApplier } from "./appliers/fromTimeMsApplier";
import { MetadataProvider } from "../editor/metadataProvider";

export const registerMacrosService = (
  metadataProvider: MetadataProvider,
  getTableFn: (sql: string) => string
) => {
  let macrosService = new MacrosService();
  macrosService.registerMacros(new ConditionalAllApplier());
  macrosService.registerMacros(
    new AdHocFilterApplier(metadataProvider, getTableFn)
  );

  macrosService.registerMacros(new IntervalSApplier());
  macrosService.registerMacros(new TimeIntervalApplier());
  macrosService.registerMacros(new TimeIntervalMsApplier());

  macrosService.registerMacros(new TimeFilterApplier());
  macrosService.registerMacros(new TimeFilterMsApplier());
  macrosService.registerMacros(new DateFilterApplier());
  macrosService.registerMacros(new DateTimeFilterApplier());
  macrosService.registerMacros(new DTApplier());

  macrosService.registerMacros(new ToTimeApplier());
  macrosService.registerMacros(new ToTimeMsApplier());
  macrosService.registerMacros(new FromTimeApplier());
  macrosService.registerMacros(new FromTimeMsApplier());
  return macrosService;
};
