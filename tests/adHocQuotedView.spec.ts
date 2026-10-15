import { test, expect, DashboardPage } from "@grafana/plugin-e2e";
import { BrowserContext, Page } from "@playwright/test";
// @ts-ignore
import {
  captureRequestsAndResponses,
  closeWhatsNewDialog,
  ConfigPageSteps,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";

/**
 * `$__adHocFilter(<table>)` with a quoted custom-view name (HDX-12561).
 *
 * Hydrolix custom views are named `<table>#<view>`, which is only a valid
 * identifier when quoted. The explicit macro argument bypasses AST resolution,
 * so the backend gate (`explicitCTEArg` in pkg/plugin/macros_adhoc.go) has to
 * accept the quoted forms and re-quote them for DESCRIBE. (The unquoted form's
 * rejection is unit-tested in pkg/plugin/metadata_injection_test.go.)
 *
 * Fixture: the view `e2e.adhoc_topk#view` over `e2e.adhoc_topk` from
 * `testdata/containers/initdb.sql` — 106 rows, exactly one with
 * status = 'rare1'. A dropped filter falls back to 1=1 and counts all 106,
 * so each accepted case asserts the exact count of 1.
 */

test.describe.configure({ mode: "serial" });

const ADHOC_VAR = "hdx_adhoc";
const TABLE_VAR = "hdx_table";
const VIEW = "e2e.`adhoc_topk#view`";

interface WireQuery {
  refId: string;
  rawSql?: string;
}
interface WireRequest {
  queries?: WireQuery[];
}
interface WireFrame {
  schema?: { fields?: Array<{ name: string }> };
  data?: { values?: unknown[][] };
}
interface WireResult {
  error?: string;
  frames?: WireFrame[];
}
interface WireResponse {
  results?: Record<string, WireResult>;
}

/** The result for the panel query, found by its macro call in the request. */
function findPanelResult(
  requests: string[],
  responses: string[],
  macroCall: string
): WireResult | undefined {
  for (let i = requests.length - 1; i >= 0; i--) {
    try {
      const req = JSON.parse(requests[i]) as WireRequest;
      const query = (req?.queries ?? []).find((q) =>
        q?.rawSql?.includes(macroCall)
      );
      if (!query) {
        continue;
      }
      const resp = JSON.parse(responses[i]) as WireResponse;
      const result = resp?.results?.[query.refId];
      if (result?.error || result?.frames?.length) {
        return result;
      }
    } catch {
      continue;
    }
  }
  return undefined;
}

function columnByName(frame: WireFrame, name: string): unknown[] | undefined {
  const index = (frame?.schema?.fields ?? []).findIndex(
    (f) => f.name === name
  );
  return index < 0 ? undefined : frame?.data?.values?.[index];
}

let datasource: { type: string; uid: string };

test.beforeEach(async ({ createDataSourceConfigPage, page }) => {
  if (datasource !== undefined) {
    return;
  }
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "adhoc quoted view",
    createDataSourceConfigPage
  );
  await steps.fillTestNativeDatasource();
  await steps.configPageLocator
    .additionalSettingsExpandable()
    .click({ force: true });
  await steps.configPageLocator.adHocTableVariable().fill(TABLE_VAR);
  await steps.saveSuccess(dsConfigPage);
  datasource = dsConfigPage.datasource;
});

interface PanelFixtures {
  gotoDashboardPage: (args: { uid: string }) => Promise<DashboardPage>;
  page: Page;
  context: BrowserContext;
}

async function runPanel(
  macroArg: string,
  { gotoDashboardPage, page, context }: PanelFixtures
): Promise<WireResult> {
  const macroCall = `$__adHocFilter(${macroArg})`;
  const { uid } = await new DashboardBuilder(page, datasource)
    .withTitle(`adhoc-quoted-view-${Date.now()}`)
    .addCustomVariable({ name: TABLE_VAR, query: VIEW })
    .addAdHocVariable({
      name: ADHOC_VAR,
      filters: [{ key: "status", operator: "=", value: "rare1" }],
    })
    .addPanel({
      title: "adhoc",
      rawSql: `select count() as cnt from ${VIEW} where ${macroCall}`,
    })
    .withTimeRange("2025-04-19T00:00:00.000Z", "2025-04-19T23:59:59.000Z")
    .create();

  const { requests, responses } = await captureRequestsAndResponses(context);
  const dashboardPage = await gotoDashboardPage({ uid });
  await closeWhatsNewDialog(page);
  await dashboardPage.refreshDashboard();

  await expect
    .poll(() => findPanelResult(requests, responses, macroCall) !== undefined, {
      timeout: 30000,
    })
    .toBe(true);
  return findPanelResult(requests, responses, macroCall)!;
}

for (const macroArg of [VIEW, `"e2e"."adhoc_topk#view"`]) {
  test(`$__adHocFilter(${macroArg}) filters the custom view`, async ({
    gotoDashboardPage,
    page,
    context,
  }) => {
    const result = await runPanel(macroArg, {
      gotoDashboardPage,
      page,
      context,
    });

    expect(result.error, "panel query should not error").toBeUndefined();
    const counts = columnByName(result.frames![0], "cnt");
    expect(counts, "cnt column should be present").toBeTruthy();
    expect(
      Number(counts![0]),
      "cnt=106 means the macro dropped the filter and fell back to 1=1"
    ).toBe(1);
  });
}
