import { test, expect } from "@grafana/plugin-e2e";
import { Page } from "@playwright/test";
// @ts-ignore
import {
  captureRequestsAndResponses,
  closeWhatsNewDialog,
  ConfigPageSteps,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";
import { AdHocFilter } from "./adHocFilter";

/**
 * Ad-hoc keys prefer aliases (CFB-4004): with `jsonData.adHocKeysPreferAliases` on,
 * ad-hoc key discovery offers a same-type identity alias instead of its
 * physical base. The option is the "Ad hoc filter keys prefer aliases"
 * switch in the datasource's additional settings.
 *
 * Fixture: `e2e.adhoc_topk` from `testdata/containers/initdb.sql`, whose
 * `status_alias String ALIAS status` is reported by plain ClickHouse in the
 * bare DESCRIBE spelling (`default_expression = status`). The stock case
 * therefore also proves the alias parser accepts that spelling: before the
 * fix the alias was dropped as untyped. Exactly one fixture row has
 * `status = 'rare1'`, so a filter on the alias that reaches SQL by its own
 * name yields a count of 1 (a dropped filter counts all 106).
 *
 * `initdb.sql` only runs on ClickHouse container init: if `status_alias` is
 * missing from the key list, recreate the clickhouse volume before
 * suspecting the code.
 */

const FIXTURE_TABLE = "e2e.adhoc_topk";
const DASHBOARD_FROM = "2025-04-19T00:00:00.000Z";
const DASHBOARD_TO = "2025-04-20T00:00:00.000Z";
const KEYS_TIMEOUT_MS = 15_000;

interface WireRequest {
  queries?: Array<{ refId: string; rawSql?: string; filters?: Array<{ key: string }> }>;
}
interface WireResponse {
  results?: Record<string, { error?: string; frames?: Array<{ data?: { values?: unknown[][] } }> }>;
}

/** The count returned for the panel query that carried an ad-hoc filter on `key`. */
function filteredCount(requests: string[], responses: string[], key: string): number | undefined {
  for (let i = requests.length - 1; i >= 0; i--) {
    try {
      const req = JSON.parse(requests[i]) as WireRequest;
      const query = (req.queries ?? []).find(
        (q) => q.rawSql?.includes("$__adHocFilter") && (q.filters ?? []).some((f) => f.key === key)
      );
      if (!query) {
        continue;
      }
      const result = (JSON.parse(responses[i]) as WireResponse).results?.[query.refId];
      const value = result?.frames?.[0]?.data?.values?.[0]?.[0];
      if (value !== undefined) {
        return Number(value);
      }
    } catch {
      continue;
    }
  }
  return undefined;
}

async function createAdHocDatasource(
  page: Page,
  name: string,
  createDataSourceConfigPage: Parameters<ConfigPageSteps["createDatasourceConfigPage"]>[1],
  adHocKeysPreferAliases: boolean
) {
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(name, createDataSourceConfigPage);
  await steps.fillTestHttpDatasource();
  await steps.configPageLocator.additionalSettingsExpandable().click({ force: true });
  await steps.configPageLocator.defaultDatabase().fill("e2e");
  await steps.configPageLocator.adHocTableVariable().fill("table");
  if (adHocKeysPreferAliases) {
    await steps.configPageLocator.adHocKeysPreferAliasesSwitch().check({ force: true });
  }
  await steps.saveSuccess(dsConfigPage);

  const { uid, type } = dsConfigPage.datasource;
  return { uid, type };
}

test("opt-in ad-hoc keys offer the alias, keep Map keys and filter by the alias", async ({
  page,
  createDataSourceConfigPage,
  gotoDashboardPage,
  context,
}, testInfo) => {
  testInfo.setTimeout(120_000);
  const datasource = await createAdHocDatasource(
    page,
    "adhoc preferred field names",
    createDataSourceConfigPage,
    true
  );

  const { uid: dashUid } = await new DashboardBuilder(page, datasource)
    .withTitle(`adhoc-adhoc-keys-prefer-aliases-${Date.now()}`)
    .addCustomVariable({ name: "table", query: FIXTURE_TABLE })
    .addAdHocVariable({ name: "Filters" })
    .addPanel({
      rawSql: `SELECT count() AS cnt FROM ${FIXTURE_TABLE} WHERE $__adHocFilter()`,
    })
    .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
    .create();

  const { requests, responses } = await captureRequestsAndResponses(context);
  const dashboardPage = await gotoDashboardPage({ uid: dashUid });
  await closeWhatsNewDialog(page);

  const filters = new AdHocFilter(page);
  const keys = await filters.openKeys({ timeout: KEYS_TIMEOUT_MS });
  expect(keys).toContain("status_alias");
  expect(keys).not.toContain("status");
  // Unrelated columns and the Map-key enumeration are untouched by the policy.
  expect(keys).toContain("status_null");
  expect(keys).toContain("attrs['env']");
  await filters.dismiss();

  // The hidden base's values are what the alias reads, so the alias gets the
  // same preload.
  await filters.selectKey("status_alias");
  const { options } = await filters.openValues({ timeout: KEYS_TIMEOUT_MS });
  expect(options).toContain("rare1");
  await filters.pickValue("rare1");
  await dashboardPage.refreshDashboard();

  // The filter reaches SQL by its own name: `status_alias = 'rare1'` resolves
  // through the alias on the cluster and matches the single fixture row.
  await expect
    .poll(() => filteredCount(requests, responses, "status_alias"), { timeout: 30_000 })
    .toBe(1);
});

test("stock ad-hoc keys list both the alias and its base", async ({
  page,
  createDataSourceConfigPage,
  gotoDashboardPage,
}, testInfo) => {
  testInfo.setTimeout(120_000);
  const datasource = await createAdHocDatasource(
    page,
    "adhoc stock alias listing",
    createDataSourceConfigPage,
    false
  );

  const { uid: dashUid } = await new DashboardBuilder(page, datasource)
    .withTitle(`adhoc-stock-alias-${Date.now()}`)
    .addCustomVariable({ name: "table", query: FIXTURE_TABLE })
    .addAdHocVariable({ name: "Filters" })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
    .create();

  await gotoDashboardPage({ uid: dashUid });
  await closeWhatsNewDialog(page);

  const keys = await new AdHocFilter(page).openKeys({ timeout: KEYS_TIMEOUT_MS });
  expect(keys).toContain("status");
  expect(keys).toContain("status_alias");
});
