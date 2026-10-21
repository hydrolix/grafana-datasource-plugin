import { test, expect } from "@grafana/plugin-e2e";
import type {
  APIRequestContext,
  BrowserContext,
  Page,
} from "@playwright/test";
import * as net from "node:net";
// @ts-ignore
import {
  captureRequestsAndResponses,
  closeWhatsNewDialog,
  ConfigPageSteps,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";

/**
 * The "Forward OAuth + Exchange" credentials mode, end to end, with every
 * party real except the two the dev stack cannot have:
 *
 *   - Grafana signs a person in through the stack's Keycloak (generic OAuth,
 *     PKCE, `oauthPassThru`), so the plugin receives a genuine forwarded
 *     sign-in token with an `iss` and a `sub`;
 *   - the plugin reads `GF_PLUGIN_EXCHANGE_URL` / `_CREDENTIALS` the way a
 *     deployment provides them: declared in e2e/grafana.ini, valued from the
 *     container environment, rendered by Grafana into the plugin process;
 *   - the exchange and the cluster are one stub (e2e/exchange-stub), which
 *     mints an opaque token for the sign-in token's subject, forwards a query
 *     carrying that token to ClickHouse, and refuses everything else. It
 *     labels every cluster request by what it carried: a token the stub
 *     minted (`minted`, or `revoked` once withdrawn), a sign-in token it was
 *     shown as a subject (`sign-in`), or anything else (`unknown`, `none`).
 *     Only tokens the stub minted may reach the cluster — the invariant the
 *     mode exists to keep, asserted here under success, revocation and
 *     refusal alike.
 *
 * Runs only inside the compose stack (E2E_EXCHANGE_STUB_URL set on the
 * playwright service); the CI compose file has neither Keycloak nor the stub
 * yet, so there the file is skipped rather than failed.
 */

const STUB_URL = process.env.E2E_EXCHANGE_STUB_URL;
const CLUSTER_HOST = process.env.E2E_EXCHANGE_CLUSTER_HOST ?? "exchange-stub";
const CLUSTER_PORT = "9443";
const GRAFANA_URL = process.env.E2E_GRAFANA_URL ?? "http://localhost:3000";
/** Where Grafana sends the browser (GF_AUTH_GENERIC_OAUTH_AUTH_URL). */
const KEYCLOAK_BROWSER_URL = "http://localhost:8080";
/** Where Keycloak actually answers from this process's point of view. */
const KEYCLOAK_URL = process.env.E2E_KEYCLOAK_URL ?? KEYCLOAK_BROWSER_URL;
const ALICE = { username: "alice", password: "alice" };
const FORWARD_EXCHANGE = "Forward OAuth + Exchange";
const COUNT_SQL = "SELECT count() AS n FROM e2e.macros";
const COUNT = "15"; // rows in testdata/containers/initdb.sql

interface StubEvent {
  kind: "exchange" | "cluster";
  subject?: string;
  auth?: string;
  status: number;
  reason?: string;
  path?: string;
}

test.describe.configure({ mode: "serial" });
test.skip(!STUB_URL, "needs the exchange stub: E2E_EXCHANGE_STUB_URL is unset");

/**
 * Grafana points the browser at Keycloak on localhost:8080. That is right
 * for a developer's browser on the host and wrong inside the playwright
 * container, where localhost is the grafana service's network namespace and
 * nothing listens on 8080. Rewriting URLs in flight would split Keycloak's
 * cookies across two origins, so instead this listens on that port and pipes
 * bytes to the real Keycloak: browser, Keycloak and cookies all see the one
 * origin, exactly as on the host. A no-op when the two addresses agree.
 */
async function forwardLocalhostToKeycloak(): Promise<() => Promise<void>> {
  const target = new URL(KEYCLOAK_URL);
  const listenOn = new URL(KEYCLOAK_BROWSER_URL);
  if (target.host === listenOn.host) {
    return async () => {};
  }
  const sockets = new Set<net.Socket>();
  const server = net.createServer((client) => {
    const upstream = net.connect(Number(target.port || 80), target.hostname);
    sockets.add(client).add(upstream);
    const drop = () => {
      client.destroy();
      upstream.destroy();
      sockets.delete(client);
      sockets.delete(upstream);
    };
    client.on("error", drop).on("close", drop);
    upstream.on("error", drop).on("close", drop);
    client.pipe(upstream).pipe(client);
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(Number(listenOn.port), () => resolve());
  });
  return () =>
    new Promise<void>((resolve) => {
      for (const s of sockets) {
        s.destroy();
      }
      server.close(() => resolve());
    });
}

async function signInThroughKeycloak(
  context: BrowserContext,
  user: { username: string; password: string }
): Promise<Page> {
  const page = await context.newPage();
  // Straight into the generic OAuth flow rather than through the login
  // page's button: the dev image enables anonymous auth, and Grafana sends
  // an anonymous visitor from /login to the home page before any button
  // could be clicked. This URL starts the flow for everyone.
  await page.goto(`${GRAFANA_URL}/login/generic_oauth`);
  await page.locator("#username").fill(user.username);
  await page.locator("#password").fill(user.password);
  await page.locator("#kc-login").click();
  const grafana = new URL(GRAFANA_URL).origin;
  await page.waitForURL(
    (url) => url.origin === grafana && !url.pathname.startsWith("/login"),
    { timeout: 30_000 }
  );
  await closeWhatsNewDialog(page);
  return page;
}

async function stubEvents(request: APIRequestContext): Promise<StubEvent[]> {
  const res = await request.get(`${STUB_URL}/e2e/events`);
  expect(res.ok(), `stub events: ${res.status()}`).toBeTruthy();
  return res.json();
}

async function stubPost(request: APIRequestContext, path: string) {
  const res = await request.post(`${STUB_URL}${path}`);
  expect(res.ok(), `stub ${path}: ${res.status()}`).toBeTruthy();
}

/** The first value of result A's first frame, per captured query response. */
function valuesIn(responses: string[]): string[] {
  const out: string[] = [];
  for (const body of responses) {
    try {
      const v = JSON.parse(body)?.results?.A?.frames?.[0]?.data?.values?.[0]?.[0];
      if (v !== undefined && v !== null) {
        out.push(String(v));
      }
    } catch {
      // not a query response
    }
  }
  return out;
}

function errorsIn(responses: string[]): string[] {
  const out: string[] = [];
  for (const body of responses) {
    try {
      const e = JSON.parse(body)?.results?.A?.error;
      if (typeof e === "string") {
        out.push(e);
      }
    } catch {
      // not a query response
    }
  }
  return out;
}

const exchanges = (events: StubEvent[]) => events.filter((e) => e.kind === "exchange");
const clusterHits = (events: StubEvent[]) => events.filter((e) => e.kind === "cluster");
/**
 * Cluster requests carrying anything but a token the stub minted. A sign-in
 * token the stub recognises is labelled `sign-in`; one it never saw as a
 * subject, or any other bearer, is `unknown`; a missing header is `none`.
 * Filtering on `sign-in` alone would let the second case through.
 */
const foreignClusterHits = (events: StubEvent[]) =>
  events.filter((e) => e.kind === "cluster" && e.auth !== "minted" && e.auth !== "revoked");

// Shared by the serial tests below: one datasource, one dashboard, one
// signed-in person, one capture of her query traffic.
let stopForwarding: () => Promise<void> = async () => {};
let dashboardUid = "";
let alice: { context: BrowserContext; page: Page; responses: string[] };

test.beforeAll(async () => {
  stopForwarding = await forwardLocalhostToKeycloak();
});

test.afterAll(async () => {
  await alice?.context.close();
  await stopForwarding();
});

test("the exchanging datasource saves without a user, and a signed-in user's panel reaches the cluster with a minted token only", async ({
  createDataSourceConfigPage,
  browser,
  page,
  request,
}) => {
  // --- as the admin: a datasource in the exchanging mode, pointed at the stub.
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    `exchange ${Date.now()}`,
    createDataSourceConfigPage
  );
  const field = steps.configPageLocator;
  await field.host().fill(CLUSTER_HOST);
  await field.useDefaultPortSwitch().uncheck({ force: true });
  await field.port().fill(CLUSTER_PORT);
  await field.protocolItem("http").check({ force: true });
  // The mode refuses plain http and skipped verification: the bearer rides
  // only over verified TLS. The stack mounts the stub's self-signed
  // certificate into Grafana's trust store (docker-compose.yaml).
  await field.secureSwitch().check({ force: true });
  await expect(field.skipTlsVerifySwitch()).not.toBeChecked();
  await field.credentialsTypeItem(FORWARD_EXCHANGE).check({ force: true });

  // Save & test has no signed-in user to exchange for, and says so rather
  // than pretending it tested anyone's access.
  await expect(dsConfigPage.saveAndTest()).toBeOK();
  await expect(dsConfigPage).toHaveAlert("success", {
    hasText: /no signed-in user/i,
  });

  const { uid } = await new DashboardBuilder(page, dsConfigPage.datasource)
    .withTitle(`exchange-${Date.now()}`)
    .addPanel({ title: "count", type: "table", rawSql: COUNT_SQL })
    .create();
  dashboardUid = uid;

  await stubPost(request, "/e2e/reset");

  // --- as alice, signed in through Keycloak: the panel renders.
  const context = await browser.newContext();
  const { responses } = await captureRequestsAndResponses(context);
  const alicePage = await signInThroughKeycloak(context, ALICE);
  alice = { context, page: alicePage, responses };

  await alicePage.goto(`${GRAFANA_URL}/d/${dashboardUid}`);
  await expect
    .poll(() => valuesIn(responses), { timeout: 30_000 })
    .toContain(COUNT);

  // --- what the stub saw.
  const events = await stubEvents(request);
  const minted = exchanges(events);
  expect(minted, "one panel, one person: one exchange").toHaveLength(1);
  expect(minted[0].status).toBe(200);
  expect(minted[0].subject, "the exchange named the signed-in subject").toBeTruthy();

  const hits = clusterHits(events);
  expect(hits.length).toBeGreaterThan(0);
  for (const hit of hits) {
    expect(hit.auth, "every cluster request carried the minted token").toBe("minted");
    expect(hit.status).toBe(200);
    expect(hit.subject).toBe(minted[0].subject);
  }
});

test("a cluster 401 drops the token and the query is retried once with a fresh mint", async ({
  request,
}) => {
  const before = await stubEvents(request);
  const seenValues = valuesIn(alice.responses).length;

  await stubPost(request, "/e2e/revoke");
  await alice.page.reload();

  // The panel still renders: the revoked token's 401 is absorbed by one retry.
  await expect
    .poll(() => valuesIn(alice.responses).slice(seenValues), { timeout: 30_000 })
    .toContain(COUNT);

  const since = (await stubEvents(request)).slice(before.length);
  const refusedHit = since.findIndex((e) => e.kind === "cluster" && e.status === 401);
  expect(refusedHit, "the cluster refused the revoked token").toBeGreaterThanOrEqual(0);
  expect(since[refusedHit].auth).toBe("revoked");

  const after = since.slice(refusedHit + 1);
  expect(exchanges(after), "exactly one fresh mint followed the 401").toHaveLength(1);
  expect(exchanges(after)[0].status).toBe(200);
  const okHits = clusterHits(after).filter((e) => e.status === 200);
  expect(okHits.length, "the retried query reached the cluster").toBeGreaterThan(0);
  for (const hit of okHits) {
    expect(hit.auth).toBe("minted");
  }
  expect(
    foreignClusterHits(since),
    "nothing but a token the stub minted reached the cluster"
  ).toHaveLength(0);
});

test("a refusal reaches the panel as a refusal, and the sign-in token is still never forwarded", async ({
  request,
}) => {
  const before = await stubEvents(request);
  const seenErrors = errorsIn(alice.responses).length;

  // A refusal only shows when the plugin has to mint, so revoke first: the
  // 401 drops the cached token, the retry asks the exchange, and the exchange
  // says no.
  await stubPost(request, "/e2e/refuse?on=1");
  await stubPost(request, "/e2e/revoke");
  try {
    await alice.page.reload();

    await expect
      .poll(() => errorsIn(alice.responses).slice(seenErrors), { timeout: 30_000 })
      .toEqual(expect.arrayContaining([expect.stringMatching(/refused/i)]));

    const since = (await stubEvents(request)).slice(before.length);
    const refusals = exchanges(since).filter((e) => e.status === 409);
    expect(refusals.length, "the exchange refused the re-mint").toBeGreaterThan(0);
    expect(refusals[0].reason).toBe("invalid_target");
    expect(
      since.filter((e) => e.kind === "cluster" && e.status === 200),
      "nothing reached the cluster after the refusal"
    ).toHaveLength(0);
    expect(
      foreignClusterHits(since),
      "nothing but a token the stub minted reached the cluster"
    ).toHaveLength(0);
  } finally {
    await stubPost(request, "/e2e/refuse?on=0");
  }
});
