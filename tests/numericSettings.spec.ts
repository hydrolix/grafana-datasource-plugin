import { test, expect } from "@grafana/plugin-e2e";

/**
 * HDX-12842: an unquoted number in provisioning YAML, Pulumi or the API
 * reaches the backend as a JSON number. String fields used to be read with an
 * unchecked type assertion, so `dialTimeout: 10` panicked on every instance
 * build and the data source failed every health check and query. These
 * create the data source through the API, the way those tools do, because the
 * config editor always stores strings.
 */
const TYPE = "hydrolix-hydrolix-datasource";

function connection() {
  return {
    host: process.env.CLICKHOUSE_HOSTNAME ?? "clickhouse-server",
    port: 9000,
    protocol: "native",
    secure: false,
    credentialsType: "userAccount",
    username: process.env.CLICKHOUSE_USERNAME ?? "testuser",
  };
}

const secureJsonData = { password: process.env.CLICKHOUSE_PASSWORD ?? "testpass" };

test("a data source with unquoted numeric settings passes the health check and runs a query", async ({
  createDataSource,
  request,
}) => {
  const ds = await createDataSource({
    type: TYPE,
    name: "numeric settings",
    jsonData: {
      ...connection(),
      dialTimeout: 10,
      queryTimeout: 120,
      querySettings: [{ setting: "hdx_query_max_execution_time", value: 30 }],
    },
    secureJsonData,
  });

  const health = await request.get(`/api/datasources/uid/${ds.uid}/health`);
  expect(health.status(), await health.text()).toBe(200);

  const query = await request.post("/api/ds/query", {
    data: {
      from: "now-5m",
      to: "now",
      queries: [
        {
          refId: "A",
          datasource: { type: TYPE, uid: ds.uid },
          rawSql: "SELECT 1 AS one",
          format: 1,
        },
      ],
    },
  });
  expect(query.status(), await query.text()).toBe(200);
  const body = await query.json();
  expect(body.results.A.error).toBeUndefined();
  expect(body.results.A.frames[0].data.values).toEqual([[1]]);
});

test("a data source with an unsupported dialTimeout type reports which field is wrong", async ({
  createDataSource,
  request,
}) => {
  const ds = await createDataSource({
    type: TYPE,
    name: "object dial timeout",
    jsonData: { ...connection(), dialTimeout: { seconds: 10 } },
    secureJsonData,
  });

  const health = await request.get(`/api/datasources/uid/${ds.uid}/health`);
  const text = await health.text();
  expect(health.ok(), text).toBeFalsy();
  expect(text).toContain("dialTimeout");
  expect(text).not.toContain("panic");
});
