import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import { closeWhatsNewDialog, ConfigPageSteps } from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";
import { AdHocFilter } from "./adHocFilter";

/**
 * E2E coverage for the ad-hoc filter value preload guardrails
 * (adhoc-filter-values-topk-guardrails). Exercises the real dashboard
 * "Ad hoc filters" variable end-to-end against a ClickHouse fixture table
 * seeded with:
 *   - a dominant value ("common", ~100 rows) + a long tail (rare1/2/3)
 *   - empty-string rows (-> synthetic `__empty__`)
 *   - a Nullable(String) column (`status_null`, -> synthetic `__null__`)
 *   - a non-Nullable String column (`status_nonnull`, no `__null__`)
 *   - a distinctive value ("old_only") that occurs ONLY more than 24h
 *     before the pinned dashboard end time, so the trailing-24h preload
 *     window (design D6) must exclude it from suggestions while manual
 *     entry of that value still works.
 *
 * Fixture DDL/seed: testdata/containers (see
 * .claude/findings/2026-08-24-adhoc-topk-break-spike.md for the dev-stack
 * verification that produced this table). Pinned window:
 *   dashboard range: 2025-04-01T00:00:00Z .. 2025-04-20T00:00:00Z (> 24h)
 *   trailing 24h:    2025-04-19T00:00:00Z .. 2025-04-20T00:00:00Z
 *   "old_only" row:  2025-04-18T23:00:00Z (1h before the trailing window)
 *
 * Locator notes: Grafana renders this variable with two structurally
 * different widgets across the CI matrix — segment buttons on 10.4, a single
 * combobox from 11.5 on (with the placeholder changing again at 13) — so all
 * interaction goes through `tests/adHocFilter.ts`, which branches on
 * whichever is present. Read that file before changing any step here.
 */

const FIXTURE_TABLE = "e2e.adhoc_topk";
const DASHBOARD_FROM = "2025-04-01T00:00:00.000Z";
const DASHBOARD_TO = "2025-04-20T00:00:00.000Z";

/** Guardrail budget from src/constants.ts: METADATA_QUERY_TIMEOUT_VALUE
 *  (10s breaker) + AD_HOC_QUERY_GUARDRAIL_SETTINGS timeout_overflow_mode.
 *  The dropdown must populate comfortably inside that budget. */
const GUARDRAIL_BUDGET_MS = 8000;

test.describe.configure({ mode: "serial" });

test("ad hoc filter value dropdown: topK guardrails end-to-end", async ({
  page,
  createDataSourceConfigPage,
  gotoDashboardPage,
}) => {
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "adhoc topk guardrails",
    createDataSourceConfigPage
  );
  await steps.fillTestHttpDatasource();

  // Configure the ad hoc table-variable binding under "Additional settings"
  // (collapsed by default — see tests/configEditor.spec.ts for the pattern).
  await steps.configPageLocator.additionalSettingsExpandable().click({ force: true });
  await steps.configPageLocator.defaultDatabase().fill("e2e");
  await steps.configPageLocator.adHocTableVariable().fill("table");

  await steps.saveSuccess(dsConfigPage);

  const { uid, type } = dsConfigPage.datasource;
  const { uid: dashUid } = await new DashboardBuilder(page, { uid, type })
    .withTitle(`adhoc-topk-guardrails-${Date.now()}`)
    .addCustomVariable({ name: "table", query: FIXTURE_TABLE })
    .addAdHocVariable({ name: "Filters" })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
    .create();

  const dashboardPage = await gotoDashboardPage({ uid: dashUid });
  await closeWhatsNewDialog(page);

  const filters = new AdHocFilter(page);

  // --- 1. Dominant value + empty string present; outside-window value absent ---
  await filters.selectKey("status");
  const { options: statusOptions, elapsedMs } = await filters.openValues({
    timeout: GUARDRAIL_BUDGET_MS,
  });

  expect(statusOptions).toContain("common");
  expect(statusOptions).toContain("__empty__");
  expect(statusOptions).not.toContain("old_only");
  expect(elapsedMs).toBeLessThan(GUARDRAIL_BUDGET_MS);

  // Finish the filter with the dominant value.
  await filters.pickValue("common");
  await expect(
    page.getByText("status", { exact: false }).first()
  ).toBeVisible();

  // --- 2. Outside-window value still works as a typed/manual filter ---
  await filters.selectKey("status");
  await filters.openValues({ timeout: GUARDRAIL_BUDGET_MS });
  await filters.typeValue("old_only");
  await expect(page.getByText("old_only")).toBeVisible();

  // --- 3. Nullable column: __null__ present ---
  await filters.selectKey("status_null");
  const { options: nullableOptions } = await filters.openValues({
    timeout: GUARDRAIL_BUDGET_MS,
  });
  expect(nullableOptions).toContain("__null__");
  await filters.dismiss();

  // --- 4. Non-nullable column: __null__ absent ---
  await filters.selectKey("status_nonnull");
  const { options: nonNullableOptions } = await filters.openValues({
    timeout: GUARDRAIL_BUDGET_MS,
  });
  expect(nonNullableOptions).not.toContain("__null__");
  expect(nonNullableOptions).toContain("common");
  await filters.dismiss();
});
