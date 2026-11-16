import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import {
  captureRequestBodies,
  captureRequestsAndResponses,
  closeWhatsNewDialog,
  ConfigPageSteps,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";
import { AdHocFilter } from "./adHocFilter";
import {
  AD_HOC_MAP_KEY_TOP_K,
  AD_HOC_PRELOAD_LOOKBACK_SECONDS,
} from "../src/constants";

/**
 * Map-key discovery gets its time window from the tag-keys options argument
 * Grafana passes to `getTagKeys`, not from request state cached on the
 * datasource. Two things are only observable end-to-end:
 *
 *   - the `Map(String, Nullable(String))` column is expanded into concrete
 *     `attrs['<key>']` ad-hoc keys at all (if the discovery query fails,
 *     `getTagKeys` rejects and the key list comes back empty, which is exactly
 *     what a missing fixture looks like — see the fixture note below);
 *   - the discovery query is issued on the capped trailing-24h window.
 *
 * Fixture: `e2e.adhoc_topk` from `testdata/containers/initdb.sql`, whose
 * `attrs` column carries `map('env', 'prod')` on its bulk rows. Note that
 * `initdb.sql` only runs on ClickHouse container *init* — if this spec fails
 * with an empty key list, check the table exists before suspecting the code.
 */

const FIXTURE_TABLE = "e2e.adhoc_topk";
const DASHBOARD_FROM = "2025-04-01T00:00:00.000Z";
const DASHBOARD_TO = "2025-04-20T00:00:00.000Z";

test("map-typed ad-hoc key is discovered on the capped window", async ({
  page,
  createDataSourceConfigPage,
  gotoDashboardPage,
  context,
}, testInfo) => {
  testInfo.setTimeout(120_000);

  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "adhoc map keys",
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
    .withTitle(`adhoc-map-keys-${Date.now()}`)
    .addCustomVariable({ name: "table", query: FIXTURE_TABLE })
    .addAdHocVariable({ name: "Filters" })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
    .create();

  const bodies = await captureRequestBodies(context);
  await gotoDashboardPage({ uid: dashUid });
  await closeWhatsNewDialog(page);

  // Selecting the expanded key at all proves discovery ran and resolved:
  // getTagKeys() only offers `attrs['env']` if the mapKeys query succeeded.
  const filters = new AdHocFilter(page);
  await filters.selectKey("attrs['env']");

  // The discovery query is the one enumerating map keys for `attrs`.
  await expect
    .poll(() => bodies.some((b) => b.includes("mapKeys")), { timeout: 15_000 })
    .toBe(true);

  const discovery = bodies
    .map((b) => JSON.parse(b))
    .find((parsed) => parsed?.queries?.[0]?.rawSql?.includes("mapKeys"));
  expect(discovery, "expected to capture the map-key discovery request").toBeDefined();

  const dashboardToMs = new Date(DASHBOARD_TO).getTime();
  expect(discovery.to).toBe(String(dashboardToMs));
  expect(discovery.from).toBe(
    String(dashboardToMs - AD_HOC_PRELOAD_LOOKBACK_SECONDS * 1000)
  );
});

interface WireFrame {
  data?: { values?: unknown[][] };
}
interface WireResponse {
  results?: Record<string, { error?: string; frames?: WireFrame[] }>;
}

/** Keys returned by the discovery query for `column`, in response order. */
function discoveredKeys(
  requests: string[],
  responses: string[],
  column: string
): { keys?: unknown[]; error?: string } | undefined {
  for (let i = requests.length - 1; i >= 0; i--) {
    try {
      const query = (
        JSON.parse(requests[i])?.queries as Array<{ refId: string; rawSql?: string }>
      )?.find((q) => q?.rawSql?.includes(`mapKeys(${column})`));
      if (!query) {
        continue;
      }
      const result = (JSON.parse(responses[i]) as WireResponse)?.results?.[query.refId];
      return { keys: result?.frames?.[0]?.data?.values?.[0], error: result?.error };
    } catch {
      continue;
    }
  }
  return undefined;
}

/**
 * Fixture: `e2e.adhoc_map_topk` from `testdata/containers/initdb.sql`. Its
 * `attributes` map has 152 distinct keys in the trailing window (`hot` on
 * every row, `warm` on every other row, one `cold<n>` per row), and its
 * primary key column is named `key`, which the discovery query must not
 * shadow with an alias.
 */
test("map-key discovery returns the most common keys first, capped", async ({
  page,
  createDataSourceConfigPage,
  gotoDashboardPage,
  context,
}, testInfo) => {
  testInfo.setTimeout(120_000);

  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "adhoc map key cap",
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
    .withTitle(`adhoc-map-key-cap-${Date.now()}`)
    .addCustomVariable({ name: "table", query: "e2e.adhoc_map_topk" })
    .addAdHocVariable({ name: "Filters" })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
    .create();

  const { requests, responses } = await captureRequestsAndResponses(context);
  await gotoDashboardPage({ uid: dashUid });
  await closeWhatsNewDialog(page);

  const options = await new AdHocFilter(page).openKeys({ timeout: 15_000 });
  const hot = options.indexOf("attributes['hot']");
  const warm = options.indexOf("attributes['warm']");
  expect(hot, `key options: ${options.join(", ")}`).toBeGreaterThanOrEqual(0);
  expect(warm).toBeGreaterThan(hot);

  // The dropdown virtualizes long lists, so the cap is checked on the rows
  // the dropdown was built from rather than on the rendered options.
  await expect
    .poll(() => discoveredKeys(requests, responses, "attributes")?.keys?.length, {
      timeout: 15_000,
    })
    .toBe(AD_HOC_MAP_KEY_TOP_K);
  const discovery = discoveredKeys(requests, responses, "attributes");
  expect(discovery?.error).toBeUndefined();
  expect(discovery?.keys?.slice(0, 2)).toEqual(["hot", "warm"]);
});
