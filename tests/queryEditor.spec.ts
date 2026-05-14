import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import {
  captureRequestBodies,
  captureSqls,
  closeWhatsNewDialog,
  ConfigPageSteps,
  queryTextSet,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";
import { QueryEditorRow } from "./queryEditorRow";
import { VariablePicker } from "./variablePicker";
import { wait } from "fork-ts-checker-webpack-plugin/lib/utils/async/wait";

test("smoke: should render query editor", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  selectors,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor render",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor render");
  await expect(
    panelEditPage.getByGrafanaSelector(
      selectors.components.CodeEditor.container
    )
  ).toBeVisible();

  const queryRow = panelEditPage.getQueryEditorRow("A");
  await expect(queryRow.getByTestId("data-testid round input")).toBeVisible();
  await expect(queryRow.getByTestId("data-testid query type")).toBeVisible();
});

test("should show beautified Hydrolix error in query row on syntax error", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor error message",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestHttpDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor error message");

  // ORDER BY after LIMIT is a ClickHouse syntax error (Code: 62).
  await queryTextSet(
    "A",
    "SELECT * FROM e2e.macros LIMIT 1 ORDER BY datetime DESC",
    panelEditPage
  );
  await panelEditPage.refreshPanel();

  // Grafana renders the beautified per-query error inside the QueryEditorRow,
  // not in the panel's "No data" status element (which fires when the backend
  // returns an empty frame alongside the error).
  const queryRow = panelEditPage.getQueryEditorRow("A");
  const errorText = queryRow.getByText(/Syntax error/i);
  await expect(errorText).toBeVisible({ timeout: 30000 });

  // Beautified content from ErrorMessageBeautifier — the message extracted
  // after "DB::Exception:" should surface in the panel.
  await expect(errorText).toContainText(/Syntax error/i);

  // None of the raw transport wrapping should leak through.
  await expect(queryRow).not.toContainText(/error querying the database/i);
  await expect(queryRow).not.toContainText(/sendQuery/i);
  await expect(queryRow).not.toContainText(/HTTP 400/i);
  await expect(queryRow).not.toContainText(/DB::Exception/i);
});

test("should provide editor hints", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  context,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor hints",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor hints");

  await context.route("**/*/api/ds/query*", async (route, request) => {
    let body = request.postData();
    if (
      body?.includes(
        "SELECT name FROM system.tables WHERE engine = 'TurbineStorage' AND database = "
      )
    ) {
      const json = {
        results: {
          MD: {
            status: 200,
            frames: [
              {
                schema: {
                  name: "MD",
                  refId: "MD",
                  meta: {
                    typeVersion: [0, 0],
                    executedQueryString: "",
                  },
                  fields: [
                    {
                      name: "name",
                      type: "string",
                      typeInfo: {
                        frame: "string",
                      },
                    },
                  ],
                },
                data: {
                  values: [["macros"]],
                },
              },
            ],
          },
        },
      };
      await route.fulfill({ json });
    } else if (
      body?.includes(
        "SELECT DISTINCT database as project FROM system.tables WHERE engine = 'TurbineStorage'"
      )
    ) {
      const json = {
        results: {
          MD: {
            status: 200,
            frames: [
              {
                schema: {
                  name: "MD",
                  refId: "MD",
                  meta: {
                    typeVersion: [0, 0],
                    executedQueryString: "",
                  },
                  fields: [
                    {
                      name: "project",
                      type: "string",
                      typeInfo: {
                        frame: "string",
                      },
                    },
                  ],
                },
                data: {
                  values: [["e2e"]],
                },
              },
            ],
          },
        },
      };
      await route.fulfill({ json });
    } else {
      // Same disposal-race guard as captureSqls / captureRequestBodies:
      // fetch+fulfill on a fast-navigating page can leave the Response
      // disposed before fulfill, throwing `Fetch response has been disposed`.
      try {
        const response = await route.fetch();
        await route.fulfill({ response });
      } catch {
        // route disposed / target closed: ignore
      }
    }
  });

  await queryTextSet("A", "select ", panelEditPage);
  await page.keyboard.press("Control+Space");

  // `await expect(suggestionLocator).toBeVisible()` below absorbs the
  // post-Ctrl+Space backend warm-up via its implicit 5s retry — no
  // explicit `wait` needed for the first invocation.
  const suggestionLocator = page.getByRole("listbox", { name: "Suggest" });

  // New query column suggestion
  await expect(suggestionLocator).toBeVisible();
  let suggestions = await suggestionLocator.locator("a").allTextContents();
  expect(suggestions).toEqual(
    expect.arrayContaining([
      "*",
      "$__interval_s()",
      "$__timeInterval(timeColumn)",
      "$__timeInterval_ms(timeColumn)",
    ])
  );

  // Database suggestion
  await page.keyboard.press("Escape");
  await queryTextSet("A", "select * from ", panelEditPage);
  await page.keyboard.press("Control+Space");
  await wait(200);

  await expect(suggestionLocator).toBeVisible();
  suggestions = await suggestionLocator.locator("a").allTextContents();
  expect(suggestions).toEqual(expect.arrayContaining(["e2e"]));

  // Table suggestion
  await page.keyboard.press("Escape");
  await queryTextSet("A", "select * from e2e.", panelEditPage);
  await page.keyboard.press("Control+Space");
  await wait(200);

  await expect(suggestionLocator).toBeVisible();
  suggestions = await suggestionLocator.locator("a").allTextContents();
  expect(suggestions).toEqual(expect.arrayContaining(["macros"]));

  // Column suggestion
  await page.keyboard.press("Escape");
  await queryTextSet("A", "select  from e2e.macros", panelEditPage);
  for (let i = 0; i < 16; i++) {
    await page.keyboard.press("ArrowLeft");
  }
  await page.keyboard.press("Control+Space");
  await wait(200);

  await expect(suggestionLocator).toBeVisible();
  suggestions = await suggestionLocator.locator("a").allTextContents();
  expect(suggestions).toEqual(
    expect.arrayContaining(["datetime", "date", "v1"])
  );
});

/**
 * #8 – Interpolated Query show/hide and copy-to-clipboard
 *
 * Confirms the full InterpolatedQuery flow against real interpolation:
 *   - Toggle "Show Interpolated Query" → the heading + <pre> appear with the
 *     macro expanded (no "$__" remains in the body).
 *   - Click the copy IconButton → navigator.clipboard receives the SAME
 *     expanded SQL (proves the prop wiring, not just the icon).
 *   - Toggle "Hide Interpolated Query" → the heading disappears.
 *
 * Clipboard access requires explicit permission grants on the browser context.
 */
test("interpolated query is shown, hidden, and copied to clipboard", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  context,
}, testInfo) => {
  // Interpolation hits the plugin's Go backend (/interpolate). On a cold
  // dev container this can take 20-40s on the first call, so we lift the
  // per-test budget above the default 60s.
  testInfo.setTimeout(150000);
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);

  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor interpolated",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor interpolated");

  // Pin the time range so the interpolated SQL is deterministic enough to
  // assert against (the absolute timestamps may differ between runs, but the
  // macro string must be gone after interpolation).
  await panelEditPage.timeRange.set({
    from: "2025-04-10 00:00:00",
    to: "2025-04-10 23:59:59",
    zone: "Coordinated Universal Time",
  });

  const query =
    "SELECT * FROM e2e.macros WHERE $__timeFilter(datetime)";
  await queryTextSet("A", query, panelEditPage);

  // No refreshPanel() needed before clicking Show: QueryEditor.tsx's
  // interpolation useDebounce now lists `props.datasource.options` as a
  // dependency, so when dryRun() populates options the debounce re-fires
  // and the interpolate branch runs on the second pass. (Previously the
  // first click landed in dryRun() forever — see git history for the fix.)
  await page
    .getByRole("button", { name: /Show Interpolated Query/i })
    .click();

  await expect(
    page.getByRole("heading", { name: "Interpolated Query" })
  ).toBeVisible();

  // While interpolation is in flight, the IconButton renders with tooltip
  // "processing" (spinner icon) and the <pre> body is empty. Wait for the
  // steady "copy to clipboard" tooltip before asserting on the body.
  // Interpolation can take several seconds on a cold container; give it
  // more headroom than the default 5s.
  const copyButton = page.getByRole("button", { name: "copy to clipboard" });
  await expect(copyButton).toBeVisible({ timeout: 60000 });

  // The interpolated <pre> sits directly under the editor; it is the only
  // <pre> on the page that contains the original SELECT statement.
  const interpolatedPre = page
    .locator("pre")
    .filter({ hasText: /SELECT \* FROM e2e\.macros/i })
    .first();
  await expect(interpolatedPre).toBeVisible();
  await expect(interpolatedPre).not.toContainText("$__timeFilter");
  await expect(interpolatedPre).toContainText("datetime");

  await copyButton.click();

  const clipboardText = await page.evaluate(() =>
    navigator.clipboard.readText()
  );
  expect(clipboardText).toContain("SELECT * FROM e2e.macros");
  expect(clipboardText).not.toContain("$__timeFilter");

  await page
    .getByRole("button", { name: /Hide Interpolated Query/i })
    .click();
  await expect(
    page.getByRole("heading", { name: "Interpolated Query" })
  ).not.toBeVisible();
});

/**
 * #9 – Query settings round-trip through the request body
 *
 * Adds a setting in the QuerySettings collapsible and verifies that running
 * the query carries it to the backend. The assertion is on the captured
 * POST body to /api/ds/query — this exercises QueryEditor → HdxQuery.querySettings
 * → backend serialization in one shot.
 *
 * react-select is driven by clicking, typing the option text, then pressing
 * Enter — same pattern used elsewhere in plugin-e2e suites; more robust than
 * relying on portal node positions.
 */
test("query setting is added in the UI and sent in the query request", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  context,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor querySettings",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor querySettings");

  const row = new QueryEditorRow(panelEditPage, "A");
  await row.setSql("SELECT 1");
  await row.openQuerySettings();
  await row.addQuerySetting("hdx_query_max_rows", "42");

  // Capture the POST body sent on refresh so we can assert what the plugin
  // serialised into the request (the panel still renders normally because
  // the route is a passthrough).
  const bodies = await captureRequestBodies(context, "**/api/ds/query**");

  await expect(panelEditPage.refreshPanel()).toBeOK();

  expect(bodies.length, "expected to capture the /api/ds/query body").toBeGreaterThan(0);
  const parsed = JSON.parse(bodies[0]);
  const querySettings = parsed?.queries?.[0]?.querySettings;
  expect(Array.isArray(querySettings)).toBe(true);
  expect(querySettings).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        setting: "hdx_query_max_rows",
        value: "42",
      }),
    ])
  );
});

/**
 * #6 – Run + render a panel
 *
 * The existing query-editor specs stop at autocomplete / table refresh.
 * Nothing today asserts that data actually lands in the panel. This test
 * fills that gap by:
 *   1. Setting an absolute time range that intersects the e2e.macros fixture.
 *   2. Typing a simple SELECT (no macros — keeps the surface small).
 *   3. Switching to Table view.
 *   4. Hitting refreshPanel().
 *   5. Asserting the expected field names appear AND specific rows are
 *      rendered in the table.
 *
 * Macros are intentionally avoided here — they're already covered by
 * tests/macroFunctions.spec.ts. The point of this test is the data-flow
 * itself: query string → backend → panel → table cells.
 */
test("runs a SELECT and renders fixture rows in the table panel", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor run-and-render",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor run-and-render");
  await panelEditPage.toggleTableView();

  await queryTextSet(
    "A",
    "SELECT datetime, date, v1 FROM e2e.macros WHERE date = '2025-04-10' ORDER BY datetime",
    panelEditPage
  );

  await panelEditPage.timeRange.set({
    from: "2025-04-10 00:00:00",
    to: "2025-04-10 23:59:59",
    zone: "Coordinated Universal Time",
  });

  await expect(panelEditPage.refreshPanel()).toBeOK();

  await expect(panelEditPage.panel.fieldNames).toContainText([
    "datetime",
    "date",
    "v1",
  ]);
  // Two rows are present for 2025-04-10 in the fixture (see macroFunctions.spec.ts).
  await expect(panelEditPage.panel.data).toContainText([
    "2025-04-10 00:20:00",
    "2025-04-10",
    "2000",
  ]);
  await expect(panelEditPage.panel.data).toContainText([
    "2025-04-10 00:30:00",
    "2025-04-10",
    "3000",
  ]);

  // Negative check: a row outside the WHERE clause must NOT show up.
  await expect(panelEditPage.panel.data).not.toContainText([
    "2025-04-09 00:00:00",
  ]);
});

/**
 * #12 – Template variable substitution
 *
 * Verifies templateSrv.replace() is wired through to the query the plugin
 * actually sends. The plugin's datasource.ts has interpolation logic
 * (applyConditionalAll, replace, etc.) that's invisible to unit tests
 * because it needs a real Grafana templating context.
 *
 * The dashboard (a Custom variable `tbl` + a panel that selects from
 * `e2e.$tbl`) is created through Grafana's HTTP API rather than the
 * Settings → Variables UI: that UI has moved across Grafana 10/11/12/13
 * (button names, tab labels, input labels) and was the source of the
 * previous flake. The JSON model is stable.
 *
 * Flow:
 *   1. Create the datasource (UI), then POST a dashboard JSON that has
 *      the variable predefined and one panel targeting `e2e.$tbl`.
 *   2. Navigate to the dashboard, capture outgoing SQL on /api/ds/query.
 *   3. Refresh → outgoing SQL contains "e2e.macros" (default value).
 *   4. Open the variable picker, switch to "no_such_table", refresh.
 *   5. Outgoing SQL now contains "e2e.no_such_table" — proves substitution
 *      is dynamic, not just the initial render.
 */
test("template variable is substituted into the outgoing SQL", async ({
  createDataSourceConfigPage,
  gotoDashboardPage,
  page,
  context,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor template-var",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  const { uid } = await new DashboardBuilder(page, dsConfigPage.datasource)
    .withTitle(`template-var-test-${Date.now()}`)
    .addCustomVariable({ name: "tbl", query: "macros,no_such_table" })
    .addPanel({
      rawSql: "SELECT datetime, date, v1 FROM e2e.$tbl WHERE date = '2025-04-10'",
    })
    .withTimeRange("2025-04-10T00:00:00.000Z", "2025-04-10T23:59:59.000Z")
    .create();

  const capturedSqls = await captureSqls(context);

  const dashboardPage = await gotoDashboardPage({ uid });
  await closeWhatsNewDialog(page);

  // First load already triggers a panel query with $tbl=macros.
  await expect
    .poll(() => capturedSqls.some((s) => /FROM\s+e2e\.macros\b/i.test(s)), {
      timeout: 30000,
    })
    .toBe(true);

  // Switch the variable via the on-page picker. VariablePicker handles the
  // Grafana 10 (button + checkbox list) vs 11+ (react-select + option list)
  // split internally.
  capturedSqls.length = 0;
  await new VariablePicker("tbl", page).select("no_such_table");

  await dashboardPage.refreshDashboard();

  await expect
    .poll(
      () => capturedSqls.some((s) => /FROM\s+e2e\.no_such_table\b/i.test(s)),
      { timeout: 30000 }
    )
    .toBe(true);
});
