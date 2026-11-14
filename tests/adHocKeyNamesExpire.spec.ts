import { test, expect } from '@grafana/plugin-e2e';
import { Page } from '@playwright/test';
// @ts-ignore
import { captureRequestsAndResponses, closeWhatsNewDialog, ConfigPageSteps } from './helpers';
import { DashboardBuilder } from './dashboardBuilder';
import { AdHocFilter } from './adHocFilter';
import { visibleOptionTexts } from './grafanaSelect';

/**
 * Ad hoc filter key names expire (CFB-4795). A Hydrolix table gains names while
 * dashboards are open: a naming profile publishes `userAgent` as an alias of `UA`.
 * The browser's key list and the backend's column map behind `$__adHocFilter` are
 * both read again a minute after they were read, so the new name is offered, and
 * a filter on it applies, without saving the datasource or reloading the page.
 *
 * The test owns its table and adds the alias with the dashboard open. The browser
 * clock is installed so the key list's minute can be skipped; the backend's
 * minute passes in real time.
 */

const TABLE = `e2e.adhoc_names_${Date.now()}`;
const DASHBOARD_FROM = '2025-04-19T00:00:00.000Z';
const DASHBOARD_TO = '2025-04-20T00:00:00.000Z';
const KEYS_TIMEOUT_MS = 15_000;
const KEY_LIST_TTL_MS = 60_000;

async function clickhouse(sql: string): Promise<void> {
  const host = process.env.E2E_CLICKHOUSE_HTTP_HOST ?? process.env.CLICKHOUSE_HOSTNAME ?? 'clickhouse-server';
  const username = process.env.CLICKHOUSE_USERNAME ?? 'testuser';
  const password = process.env.CLICKHOUSE_PASSWORD ?? 'testpass';
  const res = await fetch(`http://${host}:8123/`, {
    method: 'POST',
    headers: { Authorization: 'Basic ' + Buffer.from(`${username}:${password}`).toString('base64') },
    body: sql,
  });
  if (!res.ok) {
    throw new Error(`ClickHouse HTTP query failed (${res.status}): ${await res.text()}`);
  }
}

interface WireRequest {
  queries?: Array<{ refId: string; rawSql?: string; filters?: Array<{ key: string }> }>;
}
interface WireResponse {
  results?: Record<string, { frames?: Array<{ data?: { values?: unknown[][] } }> }>;
}

/** The count returned by the latest panel query whose filters include every key in `keys`. */
function filteredCount(requests: string[], responses: string[], keys: string[]): number | undefined {
  for (let i = requests.length - 1; i >= 0; i--) {
    try {
      const req = JSON.parse(requests[i]) as WireRequest;
      const query = (req.queries ?? []).find(
        (q) => q.rawSql?.includes('$__adHocFilter') && keys.every((key) => (q.filters ?? []).some((f) => f.key === key))
      );
      if (!query) {
        continue;
      }
      const value = (JSON.parse(responses[i]) as WireResponse).results?.[query.refId]?.frames?.[0]?.data
        ?.values?.[0]?.[0];
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
  createDataSourceConfigPage: Parameters<ConfigPageSteps['createDatasourceConfigPage']>[1]
) {
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage('adhoc key names expire', createDataSourceConfigPage);
  await steps.fillTestHttpDatasource();
  await steps.configPageLocator.additionalSettingsExpandable().click({ force: true });
  await steps.configPageLocator.defaultDatabase().fill('e2e');
  await steps.configPageLocator.adHocTableVariable().fill('table');
  await steps.saveSuccess(dsConfigPage);
  const { uid, type } = dsConfigPage.datasource;
  return { uid, type };
}

test.beforeAll(async () => {
  await clickhouse(`CREATE TABLE ${TABLE} (ts DateTime64(3, 'UTC'), UA String) ENGINE = MergeTree() ORDER BY ts`);
  await clickhouse(
    `INSERT INTO ${TABLE} VALUES ('2025-04-19 10:00:00', 'WireReaderBot'), ('2025-04-19 10:05:00', 'WireReaderBot'), ('2025-04-19 10:10:00', 'Mozilla')`
  );
});

test.afterAll(async () => {
  await clickhouse(`DROP TABLE IF EXISTS ${TABLE}`);
});

test('a name published while the dashboard is open is offered and filters within a minute', async ({
  page,
  createDataSourceConfigPage,
  gotoDashboardPage,
  context,
}, testInfo) => {
  testInfo.setTimeout(240_000);
  const datasource = await createAdHocDatasource(page, createDataSourceConfigPage);

  // The preset filter matches every row. Its panel query fills the backend's
  // column map before the new name exists.
  const { uid: dashUid } = await new DashboardBuilder(page, datasource)
    .withTitle(`adhoc-key-names-expire-${Date.now()}`)
    .addCustomVariable({ name: 'table', query: TABLE })
    .addAdHocVariable({ name: 'Filters', filters: [{ key: 'UA', operator: '!=', value: 'nobody' }] })
    .addPanel({ rawSql: `SELECT count() AS cnt FROM ${TABLE} WHERE $__adHocFilter()` })
    .withTimeRange(DASHBOARD_FROM, DASHBOARD_TO)
    .create();

  const { requests, responses } = await captureRequestsAndResponses(context);
  await page.clock.install();
  const dashboardPage = await gotoDashboardPage({ uid: dashUid });
  await closeWhatsNewDialog(page);
  await expect.poll(() => filteredCount(requests, responses, ['UA']), { timeout: 30_000 }).toBe(3);

  const filters = new AdHocFilter(page);
  expect(await filters.openKeys({ timeout: KEYS_TIMEOUT_MS })).not.toContain('userAgent');
  await filters.dismiss();

  await clickhouse(`ALTER TABLE ${TABLE} ADD COLUMN userAgent String ALIAS UA`);

  // Within the minute the list read before the publish is reused.
  expect(await filters.openKeys({ timeout: KEYS_TIMEOUT_MS })).not.toContain('userAgent');
  await filters.dismiss();

  // Reopening renders the previous open's options for one paint before the
  // combobox's effect starts the new fetch, so wait for the fetched list.
  await page.clock.fastForward(KEY_LIST_TTL_MS + 1_000);
  await filters.openKeys({ timeout: KEYS_TIMEOUT_MS });
  await expect.poll(() => visibleOptionTexts(page), { timeout: KEYS_TIMEOUT_MS }).toContain('userAgent');
  await filters.dismiss();

  await filters.selectKey('userAgent');
  const { options } = await filters.openValues({ timeout: KEYS_TIMEOUT_MS });
  expect(options).toContain('Mozilla');
  await filters.pickValue('Mozilla');

  // The backend's map was filled without userAgent less than a minute ago, so
  // the filter is dropped at first (3 rows); once the map is read again it
  // applies (1 row). A map that never expired would stay at 3.
  await expect
    .poll(
      async () => {
        await dashboardPage.refreshDashboard();
        return filteredCount(requests, responses, ['UA', 'userAgent']);
      },
      { timeout: KEY_LIST_TTL_MS + 60_000, intervals: [5_000] }
    )
    .toBe(1);
});
