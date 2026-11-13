import { test, expect } from "@grafana/plugin-e2e";

/**
 * A data source that cannot connect used to report the driver's raw error:
 * `dial tcp 127.0.0.1:1: connect: connection refused`, the resolver's address,
 * the request URL, an HTTP answer's body. The host is configurable, so that
 * text told whoever pressed Save & test what listens on addresses they cannot
 * otherwise reach. The backend now answers with one fixed sentence per kind of
 * failure (spec: `connection-error-messages`).
 *
 * These create the data source through the API and read the health and query
 * endpoints directly: the text under test is the backend's answer, and the
 * config editor shows it unchanged.
 */
const TYPE = "hydrolix-hydrolix-datasource";

// Nothing listens on port 1 of the Grafana container's own loopback.
const CLOSED_HOST = "127.0.0.1";
const CLOSED_PORT = 1;

function expectNoNetworkDetail(message: string) {
  for (const detail of [CLOSED_HOST, `:${CLOSED_PORT}`, "dial", "tcp", "lookup"]) {
    expect(message).not.toContain(detail);
  }
}

test("Save & test on a closed port says the connection was refused, and not where", async ({
  createDataSource,
  request,
}) => {
  const ds = await createDataSource({
    type: TYPE,
    name: "closed port",
    jsonData: {
      host: CLOSED_HOST,
      port: CLOSED_PORT,
      protocol: "native",
      secure: false,
      credentialsType: "userAccount",
      username: "nobody",
      dialTimeout: "5",
    },
    secureJsonData: { password: "irrelevant" },
  });

  const health = await request.get(`/api/datasources/uid/${ds.uid}/health`);
  const body = await health.json();

  expect(body.status).toBe("ERROR");
  expect(body.message).toBe("Connection refused");
  expectNoNetworkDetail(body.message);
});

test("a wrong password is reported as an authentication failure without the server's text", async ({
  createDataSource,
  request,
}) => {
  const username = process.env.CLICKHOUSE_USERNAME ?? "testuser";
  const ds = await createDataSource({
    type: TYPE,
    name: "wrong password",
    jsonData: {
      host: process.env.CLICKHOUSE_HOSTNAME ?? "clickhouse-server",
      port: 9000,
      protocol: "native",
      secure: false,
      credentialsType: "userAccount",
      username,
    },
    secureJsonData: { password: "not-the-password" },
  });

  const health = await request.get(`/api/datasources/uid/${ds.uid}/health`);
  const body = await health.json();

  expect(body.status).toBe("ERROR");
  expect(body.message).toBe("Authentication failed");
  // ClickHouse's own message names the user it refused.
  expect(body.message).not.toContain(username);
});

test("a forwarding-mode query to a closed port shows the same fixed message in the panel", async ({
  createDataSource,
  request,
}) => {
  // The forwarding modes skip the handshake when the instance is built and
  // their health check never dials, so the first dial is a query's.
  const ds = await createDataSource({
    type: TYPE,
    name: "forwarding closed port",
    jsonData: {
      host: CLOSED_HOST,
      port: CLOSED_PORT,
      protocol: "native",
      secure: false,
      credentialsType: "forwardOAuth",
      oauthPassThru: true,
      dialTimeout: "5",
    },
  });

  const health = await request.get(`/api/datasources/uid/${ds.uid}/health`);
  expect((await health.json()).status, "the health check does not dial in this mode").toBe("OK");

  const query = await request.post("/api/ds/query", {
    data: {
      // 2025-04-10 00:00:00 – 23:59:59 UTC, epoch ms
      from: "1744243200000",
      to: "1744329599000",
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
  const result = (await query.json()).results.A;

  expect(result.error).toBe("Connection refused");
  expect(result.errorSource).toBe("downstream");
  expectNoNetworkDetail(result.error);
});
