import { MacroType } from "@grafana/plugin-ui";

export const MACROS = [
  {
    id: "$__fromTime()",
    name: "$__fromTime()",
    text: "$__fromTime",
    args: [],
    type: MacroType.Filter,
    description: "the TimeFilter's From to Unix seconds datetime",
  },
  {
    id: "$__toTime()",
    name: "$__toTime()",
    text: "$__toTime",
    args: [],
    type: MacroType.Filter,
    description: "the TimeFilter's To to Unix seconds datetime",
  },
  {
    id: "$__fromTime_ms()",
    name: "$__fromTime_ms()",
    text: "$__fromTime_ms",
    args: [],
    type: MacroType.Filter,
    description: "the TimeFilter's To to Unix milliseconds datetime",
  },
  {
    id: "$__toTime_ms()",
    name: "$__toTime_ms()",
    text: "$__toTime_ms",
    args: [],
    type: MacroType.Filter,
    description: "the TimeFilter's To to Unix milliseconds datetime",
  },
  {
    id: "$__timeFilter(dateColumn)",
    name: "$__timeFilter(dateColumn)",
    text: "$__timeFilter",
    args: ["dateColumn"],
    type: MacroType.Filter,
    description:
      "the TimeFilter's From <= args[0] AND args[0] <= To as Unix seconds datetime" +
      "args should contain one string element with a column name",
  },
  {
    id: "$__timeFilter_ms(dateColumn)",
    name: "$__timeFilter_ms(dateColumn)",
    text: "$__timeFilter_msf",
    args: ["dateColumn"],
    type: MacroType.Filter,
    description:
      "the TimeFilter's From <= args[0] AND args[0] <= To as Unix milliseconds datetime" +
      "args should contain one string element with a column name",
  },
  {
    id: "$__dateFilter(timeColumn)",
    name: "$__dateFilter(timeColumn)",
    text: "$__dateFilter",
    args: ["timeColumn"],
    type: MacroType.Filter,
    description:
      "the TimeFilter's From <= args[0] AND args[0] <= To as Date" +
      "args should contain one string element with a column name",
  },
  {
    id: "$__dateTimeFilter(dateColumn, timeColumn)",
    name: "$__dateTimeFilter(dateColumn, timeColumn)",
    text: "$__dateTimeFilter",
    args: ["dateColumn", "timeColumn"],
    type: MacroType.Filter,
    description:
      "the TimeFilter's From <= args[0] AND args[0] <= To as Date" +
      "AND From <= args[1] AND args[1] <= To as a Unix seconds datetime" +
      "args should contain two string elements. First one is for Date comparision. Second one for DateTime comparision.",
  },
  {
    id: "$__dt(dateColumn, timeColumn)",
    name: "$__dt(dateColumn, timeColumn)",
    text: "$__dt",
    args: ["dateColumn", "timeColumn"],
    type: MacroType.Filter,
    description:
      "the TimeFilter's From <= args[0] AND args[0] <= To as Date" +
      "AND From <= args[1] AND args[1] <= To as a Unix seconds datetime" +
      "args should contain two string elements. First one is for Date comparision. Second one for DateTime comparision.",
  },
  {
    id: "$__timeInterval(timeColumn)",
    name: "$__timeInterval(timeColumn)",
    text: "$__timeInterval",
    args: ["timeColumn"],
    type: MacroType.Value,
    description:
      "Rounding to the Query's Interval Start in seconds ( => 1s)" +
      "args should contain one string element with a time column name",
  },
  {
    id: "$__timeInterval_ms(timeColumn)",
    name: "$__timeInterval_ms(timeColumn)",
    text: "$__timeInterval_ms",
    args: ["timeColumn"],
    type: MacroType.Value,
    description:
      "Rounding to the Query's Interval Start in milliseconds ( => 1ms)" +
      "args should contain one string element with a time column name",
  },
  {
    id: "$__interval_s()",
    name: "$__interval_s()",
    text: "$__interval_s",
    args: [],
    type: MacroType.Value,
    description: "The Query's Interval rounded to seconds (>= 1s)",
  },
  {
    id: "$__conditionalAll(condition, $templateVar)",
    name: "$__conditionalAll(condition, $templateVar)",
    text: "$__conditionalAll",
    args: ["condition", "$templateVar"],
    type: MacroType.Filter,
    description:
      "Replaced by the first parameter when the template variable in the second parameter does not select every value. Replaced by the 1=1 when the template variable selects every value.",
  },
  {
    id: "$__adHocFilter()",
    name: "$__adHocFilter()",
    text: "$__adHocFilter",
    args: [],
    type: MacroType.Filter,
    description: "Replaced with conditions from ad hoc filter",
  },
];
