import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import { captureSqls, closeWhatsNewDialog, ConfigPageSteps } from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";
import { AdHocFilter } from "./adHocFilter";
import { AD_HOC_PRELOAD_MAX_TIMERANGE_SECONDS } from "../src/constants";

/**
 * Ad-hoc filtering against a table with no primary key (the
 * `adhoc-value-preload` spec). Both preload paths resolve their time
 * column from `system.tables.primary_key`; when it is empty there is no
 * column to filter on, so the statement builders drop the time conjunct
 * instead of emitting a predicate against an empty column name.
 *
 * Fixture: `e2e.adhoc_keyless` from `testdata/containers/initdb.sql` —
 * `ENGINE = MergeTree() ORDER BY tuple()`, so the table has no sorting key
 * and `primary_key` comes back empty. Its rows are pinned to 2020, far
 * outside the 2025 dashboard range below: a time-filtered preload would
 * return nothing, so a populated dropdown is the proof that the time
 * conjunct was dropped rather than merely widened.
 *
 * Both keyless statements keep the full guardrail suffix: the dev stack is
 * stock ClickHouse and cannot enforce `hdx_query_max_timerange_sec`, so this
 * spec proves the setting travels, not that Hydrolix accepts it on a
 * predicate-free statement (the docs say it is inert without a primary-column
 * filter; the first run against a real query head is the proving signal).
 *
 * The widget is driven through the shared `AdHocFilter` page object, whose
 * react-select helpers own the `[data-value=""]` entry point.
 *
 * `initdb.sql` only runs on ClickHouse container *init*. If this spec fails
 * with an empty key or value list, confirm the table exists before
 * suspecting the plugin.
 */

const FIXTURE_TABLE = "e2e.adhoc_keyless";
const DASHBOARD_FROM = "2025-04-01T00:00:00.000Z";
const DASHBOARD_TO = "2025-04-20T00:00:00.000Z";

/** Same budget the keyed guardrail spec uses: the 10s execution-time breaker
 *  bounds the unfiltered scan, and this fixture is five rows. */
const GUARDRAIL_BUDGET_MS = 8000;

const SETTINGS_SUFFIX = `SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = ${AD_HOC_PRELOAD_MAX_TIMERANGE_SECONDS}`;

test("ad hoc filters work on a table with no primary key", async ({
  page,
  createDataSourceConfigPage,
  gotoDashboardPage,
  context,
}, testInfo) => {
  testInfo.setTimeout(120_000);

  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "adhoc keyless",
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
    .withTitle(`adhoc-keyless-${Date.now()}`)
    .addCustomVariable({ name: "table", query: FIXTURE_TABLE })
    .addAdHocVariable({ name: "Filters" })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
    .create();

  const sqls = await captureSqls(context);
  await gotoDashboardPage({ uid: dashUid });
  await closeWhatsNewDialog(page);

  const filters = new AdHocFilter(page);

  // --- 1. Value preload populates despite the dashboard range ---
  await filters.selectKey("status");
  const { options, elapsedMs } = await filters.openValues({
    timeout: GUARDRAIL_BUDGET_MS,
  });

  expect(options).toContain("ok");
  expect(options).toContain("error");
  expect(options).toContain("warn");
  expect(elapsedMs).toBeLessThan(GUARDRAIL_BUDGET_MS);
  await filters.dismiss();

  const valueSql = sqls.find((s) => s.includes("topK(100)(status)"));
  expect(valueSql, "expected to capture the value-preload request").toBeDefined();
  expect(valueSql).not.toContain("$__timeFilter");
  expect(valueSql).toContain("$__adHocFilter()");
  expect(valueSql!.endsWith(SETTINGS_SUFFIX)).toBe(true);

  // --- 2. Map-key discovery expands the Map column instead of erroring ---
  // Offering the accessor at all proves the mapKeys query succeeded: a
  // failed discovery leaves the Map column unexpanded in the key list.
  await filters.selectKey("attrs['env']");
  await filters.dismiss();

  await expect
    .poll(() => sqls.some((s) => s.includes("mapKeys")), { timeout: 15_000 })
    .toBe(true);

  const discoverySql = sqls.find((s) => s.includes("mapKeys"))!;
  expect(discoverySql).toContain("mapKeys(attrs)");
  expect(discoverySql).not.toContain("$__timeFilter");
  expect(discoverySql).toContain("$__adHocFilter()");
  expect(discoverySql.endsWith(SETTINGS_SUFFIX)).toBe(true);
});
