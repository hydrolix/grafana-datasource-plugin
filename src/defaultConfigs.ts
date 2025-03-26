import { Protocol } from "./types";
import { dateTime } from "@grafana/data";

export default {
  username: "default",
  protocol: Protocol.Http,
  port: 443,
  useDefaultPort: true,
  secure: true,
  skipTlsVerify: false,
  defaultTimeRange: {
    from: dateTime().subtract("5m"),
    to: dateTime(),
    raw: { from: "now-5m", to: "now" },
  },
  path: "/query",
  adHocKeyQuery: "DESCRIBE ${table}",
  adHocValuesQuery:
    "SELECT ${column}, COUNT(${column}) as count  FROM ${table} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter()  GROUP BY ${column} ORDER BY count DESC LIMIT 100",
};
