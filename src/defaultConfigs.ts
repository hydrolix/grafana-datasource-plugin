import { Protocol } from "./types";
import { dateTime } from "@grafana/data";

export default {
  username: "default",
  protocol: Protocol.Native,
  port: 9440,
  useDefaultPort: true,
  secure: true,
  skipTlsVerify: false,
  adHocDefaultTimeRange: {
    from: dateTime().subtract("5m"),
    to: dateTime(),
    raw: { from: "now-5m", to: "now" },
  },
  path: "/query",
  adHocKeysQuery: "DESCRIBE ${table}",
  adHocValuesQuery:
    "SELECT ${column}, COUNT(${column}) as count  FROM ${table} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter()  GROUP BY ${column} ORDER BY count DESC LIMIT 100",
};
