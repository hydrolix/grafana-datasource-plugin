import { test, expect } from "@grafana/plugin-e2e";

/**
 * HDX-12840: the data source's Default round had no effect since v0.7.0.
 * The backend now rounds a query without its own round by the data source
 * default; a query round of "0" opts out. The query goes straight to
 * /api/ds/query so the assertion reads the expanded `$__fromTime`, not a
 * panel's rendering of it.
 */
const TYPE = "hydrolix-hydrolix-datasource";
// 2026-11-09 12:00:37 UTC: 37 s past the minute, so 1m rounds it up to 12:01:00.
const FROM_MS = 1794225637000;

async function fromSeconds(
  request: import("@playwright/test").APIRequestContext,
  uid: string,
  round?: string
): Promise<number> {
  const res = await request.post("/api/ds/query", {
    data: {
      from: String(FROM_MS),
      to: String(FROM_MS + 3600_000),
      queries: [
        {
          refId: "A",
          datasource: { type: TYPE, uid },
          rawSql: "SELECT toUnixTimestamp($__fromTime) AS f",
          format: 1,
          ...(round === undefined ? {} : { round }),
        },
      ],
    },
  });
  expect(res.status(), await res.text()).toBe(200);
  const body = await res.json();
  expect(body.results.A.error).toBeUndefined();
  return Number(body.results.A.frames[0].data.values[0][0]);
}

test("a query without its own round is rounded by the data source default", async ({
  createDataSource,
  request,
}) => {
  const ds = await createDataSource({
    type: TYPE,
    name: "default round 1m",
    jsonData: {
      host: process.env.CLICKHOUSE_HOSTNAME ?? "clickhouse-server",
      port: 9000,
      protocol: "native",
      secure: false,
      credentialsType: "userAccount",
      username: process.env.CLICKHOUSE_USERNAME ?? "testuser",
      defaultRound: "1m",
    },
    secureJsonData: { password: process.env.CLICKHOUSE_PASSWORD ?? "testpass" },
  });

  expect(await fromSeconds(request, ds.uid)).toBe(1794225660);
  expect(await fromSeconds(request, ds.uid, "0")).toBe(1794225637);
});
