import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import {
  captureAndTagPreloads,
  captureRequestBodies,
  closeWhatsNewDialog,
  ConfigPageSteps,
  findQueryLogEntry,
  stripUuidComment,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";
import { AdHocFilter } from "./adHocFilter";
import {
  AD_HOC_PRELOAD_LOOKBACK_SECONDS,
  AD_HOC_PRELOAD_MAX_TIMERANGE_SECONDS,
  AD_HOC_PRELOAD_ROUND_INTERVAL,
  AD_HOC_PRELOAD_ROUND_INTERVAL_SECONDS,
  METADATA_QUERY_TIMEOUT_SETTING,
  METADATA_QUERY_TIMEOUT_VALUE,
} from "../src/constants";

/**
 * E2E *runtime* verification of the topK guardrails (design D7), as opposed
 * to `tests/adHocFilterValues.spec.ts` which proves the user-facing dropdown
 * behavior on the fast fixture. These three tests each pin one mechanism
 * that unit tests can't reach because it only exists on the wire or inside
 * the backend:
 *
 *   1. Transmission — intercept the preload POST body and assert the exact
 *      guardrail shape (querySettings breaker, round, capped range, SETTINGS
 *      suffix).
 *   2. Rounding stability — tag two preload opens with UUID SQL comments,
 *      look up both *executed* (post-rounding, post-macro-expansion)
 *      queries in ClickHouse's `system.query_log`, and diff them.
 *   3. Slow-source tolerance — a `sleepEachRow`-throttled fixture that takes
 *      ~10s per preload scan, just under the 10s breaker, proving nothing
 *      client-side gives up early.
 *
 * Fixtures: `e2e.adhoc_topk` (shared with adHocFilterValues.spec.ts, see its
 * header comment) and `e2e.adhoc_slow_src` — both seeded by
 * `testdata/containers/initdb.sql` on ClickHouse container init; see the
 * slow-source test below for the block-size trick that makes ClickHouse's
 * 3s-per-call `sleepEachRow` cap survive a ~10s total preload.
 *
 * Serialized (like adHocFilterValues.spec.ts): the slow-source test
 * deliberately burns ~10s of wall time on a live network call, which
 * contends with adHocFilterValues.spec.ts's tight 8s budget assertion when
 * both run in parallel workers. Running this file's tests one at a time
 * keeps that contention off the shared worker pool.
 */

const FIXTURE_TABLE = "e2e.adhoc_topk";
const DASHBOARD_FROM = "2025-04-01T00:00:00.000Z";
const DASHBOARD_TO = "2025-04-20T00:00:00.000Z";

test.describe.configure({ mode: "serial" });

test(
  "ad hoc guardrails: preload payload carries querySettings breaker, round, capped range, and SETTINGS suffix",
  async ({ page, createDataSourceConfigPage, gotoDashboardPage, context }) => {
    const steps = new ConfigPageSteps(page);
    const dsConfigPage = await steps.createDatasourceConfigPage(
      "adhoc guardrails transmission",
      createDataSourceConfigPage
    );
    await steps.fillTestHttpDatasource();
    await steps.configPageLocator
      .additionalSettingsExpandable()
      .click({ force: true });
    await steps.configPageLocator.defaultDatabase().fill("e2e");
    await steps.configPageLocator.adHocTableVariable().fill("table");
    await steps.saveSuccess(dsConfigPage);

    const { uid, type } = dsConfigPage.datasource;
    const { uid: dashUid } = await new DashboardBuilder(page, { uid, type })
      .withTitle(`adhoc-guardrails-transmission-${Date.now()}`)
      .addCustomVariable({ name: "table", query: FIXTURE_TABLE })
      .addAdHocVariable({ name: "Filters" })
      .addPanel({ rawSql: "SELECT 1 AS v" })
      .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
      .create();

    const bodies = await captureRequestBodies(context);
    const dashboardPage = await gotoDashboardPage({ uid: dashUid });
    await closeWhatsNewDialog(page);

    const filters = new AdHocFilter(page);
    await filters.selectKey("status");
    await filters.openValues({ timeout: 8000 });

    // The value-preload request (refId "MD", topK(100)(status)) is one of
    // several metadata POSTs (DESCRIBE, mapKeys, primary key…) fired while
    // building the filter; find it by its distinctive rawSql fragment.
    await expect
      .poll(
        () => bodies.some((b) => b.includes("topK(100)(status)")),
        { timeout: 8000 }
      )
      .toBe(true);

    const preload = bodies
      .map((b) => JSON.parse(b))
      .find((parsed) => parsed?.queries?.[0]?.rawSql?.includes("topK(100)(status)"));
    expect(preload, "expected to capture the status value-preload request").toBeDefined();

    const target = preload.queries[0];

    const breaker = (target.querySettings as Array<{ setting: string; value: string }>).find(
      (s) => s.setting === METADATA_QUERY_TIMEOUT_SETTING
    );
    expect(breaker?.value).toBe(METADATA_QUERY_TIMEOUT_VALUE);
    expect(
      target.querySettings.some(
        (s: { setting: string }) => s.setting === "timeout_overflow_mode"
      )
    ).toBe(false);

    expect(target.round).toBe(AD_HOC_PRELOAD_ROUND_INTERVAL);

    const dashboardToMs = new Date(DASHBOARD_TO).getTime();
    expect(preload.to).toBe(String(dashboardToMs));
    expect(preload.from).toBe(
      String(dashboardToMs - AD_HOC_PRELOAD_LOOKBACK_SECONDS * 1000)
    );

    expect(target.rawSql).toContain(
      `SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = ${AD_HOC_PRELOAD_MAX_TIMERANGE_SECONDS}`
    );
  }
);

test(
  "ad hoc guardrails: executed SQL is stable across opens within a rounding window",
  async ({ page, createDataSourceConfigPage, gotoDashboardPage, context }, testInfo) => {
    testInfo.setTimeout(120_000);

    const steps = new ConfigPageSteps(page);
    const dsConfigPage = await steps.createDatasourceConfigPage(
      "adhoc guardrails rounding",
      createDataSourceConfigPage
    );
    await steps.fillTestHttpDatasource();
    await steps.configPageLocator
      .additionalSettingsExpandable()
      .click({ force: true });
    await steps.configPageLocator.defaultDatabase().fill("e2e");
    await steps.configPageLocator.adHocTableVariable().fill("table");
    await steps.saveSuccess(dsConfigPage);

    const { uid, type } = dsConfigPage.datasource;
    // Relative range: no data needs to fall inside the window, this test
    // asserts SQL text stability, not returned values.
    const { uid: dashUid } = await new DashboardBuilder(page, { uid, type })
      .withTitle(`adhoc-guardrails-rounding-${Date.now()}`)
      .addCustomVariable({ name: "table", query: FIXTURE_TABLE })
      .addAdHocVariable({ name: "Filters" })
      .addPanel({ rawSql: "SELECT 1 AS v" })
      .withTimeRange("now-2d", "now")
      .create();

    const captured = await captureAndTagPreloads(context);
    const dashboardPage = await gotoDashboardPage({ uid: dashUid });
    await closeWhatsNewDialog(page);

    const isPreload = (c: { mutatedSql: string }) =>
      c.mutatedSql.includes("topK(100)(status)");

    // Boundary guard: if the two opens (a few seconds apart) would straddle
    // a 5-minute wall-clock rounding boundary, wait past it first so the
    // pair never legitimately lands in different rounding windows.
    const boundaryMs = AD_HOC_PRELOAD_ROUND_INTERVAL_SECONDS * 1000;
    const msUntilBoundary = boundaryMs - (Date.now() % boundaryMs);
    if (msUntilBoundary < 20_000) {
      await page.waitForTimeout(msUntilBoundary + 2_000);
    }

    // --- Preload open 1 (uuid A) ---
    // The relative range is deliberately pinned away from the fixture data,
    // so the preload legitimately returns no values — this test asserts on the
    // captured request, not on what the dropdown renders.
    const filters = new AdHocFilter(page);
    await filters.selectKey("status");
    await filters.openValues({ timeout: 15_000, waitForOptions: false });
    await expect
      .poll(() => captured.filter(isPreload).length, { timeout: 15_000 })
      .toBeGreaterThanOrEqual(1);
    const uuidA = captured.filter(isPreload)[0].uuid;

    await page.waitForTimeout(5_000);
    await filters.dismiss();

    // --- Preload open 2 (uuid B) — verify a *second* request is actually
    // issued (no client-side caching short-circuits the reopen). ---
    await filters.reopenValues("status", {
      timeout: 15_000,
      waitForOptions: false,
    });
    await expect
      .poll(() => captured.filter(isPreload).length, { timeout: 15_000 })
      .toBeGreaterThanOrEqual(2);
    const uuidB = captured.filter(isPreload)[1].uuid;

    expect(uuidA).not.toBe(uuidB);

    const [sqlA, sqlB] = await Promise.all([
      findQueryLogEntry(uuidA),
      findQueryLogEntry(uuidB),
    ]);

    const strippedA = stripUuidComment(sqlA, uuidA);
    const strippedB = stripUuidComment(sqlB, uuidB);
    expect(strippedA).toBe(strippedB);

    const epochs = [...strippedA.matchAll(/toDateTime\((\d+)\)/g)].map((m) =>
      Number(m[1])
    );
    expect(epochs.length).toBeGreaterThan(0);
    for (const epoch of epochs) {
      expect(epoch % AD_HOC_PRELOAD_ROUND_INTERVAL_SECONDS).toBe(0);
    }
  }
);

test(
  "ad hoc guardrails: slow preload under the breaker cap still populates the dropdown",
  async ({ page, createDataSourceConfigPage, gotoDashboardPage }, testInfo) => {
    testInfo.setTimeout(90_000);

    const steps = new ConfigPageSteps(page);
    const dsConfigPage = await steps.createDatasourceConfigPage(
      "adhoc guardrails slow source",
      createDataSourceConfigPage
    );
    await steps.fillTestHttpDatasource();
    await steps.configPageLocator
      .additionalSettingsExpandable()
      .click({ force: true });
    await steps.configPageLocator.defaultDatabase().fill("e2e");
    await steps.configPageLocator.adHocTableVariable().fill("table");
    // See the fixture note on `addConstantVariable` below — the sleep
    // predicate rides in via a dashboard "constant" variable rather than a
    // view, so the table keeps a real primary key (`getTagValues` silently
    // returns `[]` when `metadataProvider.primaryKey` is empty, which it
    // always is for a plain ClickHouse VIEW).
    await steps.configPageLocator.adHocConditionVariable().fill("slowCondition");
    await steps.saveSuccess(dsConfigPage);

    const { uid, type } = dsConfigPage.datasource;

    // `e2e.adhoc_slow_src`: 100 rows, single INSERT (=> one part),
    // `index_granularity = 10`. ClickHouse caps a single `sleepEachRow` call
    // at 3s of *requested* sleep per block (`Code: 160 TOO_SLOW`); with the
    // default block size the whole 100-row part is read (and slept) in one
    // ~10s call that exceeds the cap and errors outright. Splitting the part
    // into ~10-row blocks via the reduced `index_granularity` plus a
    // `max_block_size` query setting keeps every individual call under 3s
    // while the *total* wall time across ~10 blocks still lands at ~9.99s.
    // Verified directly against ClickHouse and through this exact
    // datasource/backend path (2026-08-25): ~10.0-10.07s end to end.
    //
    // `max_block_size` isn't in the query-settings picker's whitelist
    // (src/labels.ts only lists hdx_* names), so it's set directly through
    // Grafana's datasource API rather than the UI — a test-only fixture
    // knob, not a plugin feature.
    const getResp = await page.request.get(`/api/datasources/uid/${uid}`);
    const dsJson = await getResp.json();
    const putResp = await page.request.put(`/api/datasources/uid/${uid}`, {
      data: {
        ...dsJson,
        jsonData: {
          ...dsJson.jsonData,
          querySettings: [
            ...(dsJson.jsonData.querySettings ?? []),
            { setting: "max_block_size", value: "10" },
          ],
        },
      },
    });
    expect(putResp.ok()).toBe(true);

    const { uid: dashUid } = await new DashboardBuilder(page, { uid, type })
      .withTitle(`adhoc-guardrails-slow-source-${Date.now()}`)
      .addCustomVariable({ name: "table", query: "e2e.adhoc_slow_src" })
      // Appended as `AND <value>` after `$__adHocFilter()` in the
      // value-preload template (`getColumnValuesStatement` in src/ast.ts
      // prepends the "AND" itself — the condition variable's value is just
      // the bare predicate).
      .addConstantVariable({
        name: "slowCondition",
        value: "sleepEachRow(0.1300) = 0",
      })
      .addAdHocVariable({ name: "Filters" })
      .addPanel({ rawSql: "SELECT 1 AS v" })
      .withTimeRange("2025-04-19T00:00:00.000Z", "2025-04-20T00:00:00.000Z")
      .create();

    const dashboardPage = await gotoDashboardPage({ uid: dashUid });
    await closeWhatsNewDialog(page);

    const filters = new AdHocFilter(page);
    await filters.selectKey("status");
    const { options, elapsedMs } = await filters.openValues({
      timeout: 60_000,
    });

    expect(options).toContain("slowval");
    // The lower bound is the load-bearing half: it proves the preload
    // genuinely waited on the slow scan rather than any client-side layer
    // (datasource queryTimeout, Grafana proxy, dropdown UI) giving up before
    // the breaker budget.
    // related to const METADATA_QUERY_TIMEOUT_VALUE
    expect(elapsedMs).toBeGreaterThanOrEqual(9_400);
    // The upper bound only needs to catch "the breaker never fired at all"
    // (i.e. the scan ran unbounded), so it carries slack for slower machines.
    // Note this measures browser wall clock, while the breaker caps
    // ClickHouse *execution* time — the difference (Grafana proxy, backend,
    // dropdown render) is pure environment overhead. It is ~0 on a dev box
    // (~10.0s end to end) but ~3s on a GitHub-hosted runner (13.0-13.5s
    // observed, dropdown populated, no error), so a ceiling near the 10s
    // nominal budget fails on infrastructure speed rather than on plugin
    // behavior.
    expect(elapsedMs).toBeLessThanOrEqual(15_000);
    // No error toast / error UI surfaced.
    await expect(page.getByRole("alert")).toHaveCount(0);
  }
);
