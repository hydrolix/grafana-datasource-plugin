import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import { closeWhatsNewDialog, ConfigPageSteps, queryTextSet } from "./helpers";

/**
 * End-to-end verification of the user-telemetry feature:
 *   1. The Grafana UI sends real attribution values inside
 *      target.meta.grafana on every /api/ds/query POST.
 *   2. The plugin merges the managed admin-comment fragment into the SQL
 *      SETTINGS clause that actually reaches ClickHouse — verified by
 *      asking the server (via getSetting) to echo the value it applied.
 *
 * The test container's clickhouse-server is configured with
 * <custom_settings_prefixes>SQL_,hdx_</custom_settings_prefixes>
 * (see testdata/containers/tcconfig.xml), so hdx_query_admin_comment is
 * accepted as a custom setting and getSetting() can return its value.
 */
test("user telemetry — metadata is sent from UI and merged into SETTINGS", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  context,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "user telemetry",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();

  // Opt into PII attribution: expand the "Additional Settings" section and
  // flip the user-identity toggle. Without this, email / login / name
  // would be redacted to "unknown" in the managed admin comment.
  await page
    .getByLabel("Expand section Additional Settings")
    .click({ force: true });
  await page
    .getByTestId("data-testid hdx_includeUserIdentityInAttribution")
    .locator("input")
    .check({ force: true });

  await configPageSteps.saveSuccess(dsConfigPage);

  // Intercept /api/ds/query — capture both the outgoing POST body (what
  // the browser sent, i.e. target.meta.grafana before any server-side
  // mutation) and the response body (what the plugin returned after the
  // server applied SETTINGS).
  const requestBodies: any[] = [];
  const responseBodies: any[] = [];
  await context.route("**/api/ds/query**", async (route, request) => {
    if (request.method() === "POST") {
      try {
        requestBodies.push(JSON.parse(request.postData() ?? ""));
      } catch {
        // malformed body — ignore
      }
    }
    try {
      const response = await route.fetch();
      const body = await response.json();
      responseBodies.push(body);
      await route.fulfill({ response, body: JSON.stringify(body) });
    } catch {
      // route disposed / target closed — captures already pushed above
    }
  });

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("user telemetry");
  await panelEditPage.toggleTableView();

  // The query asks the server to echo back the setting value it actually
  // applied. The user-supplied marker proves the SETTINGS clause from the
  // SQL survived; the plugin's managed block proves the merge ran.
  const userMarker = "e2e_user_supplied_" + Date.now();
  const query = `SELECT getSetting('hdx_query_admin_comment') AS c SETTINGS hdx_query_admin_comment='custom=${userMarker}'`;
  await queryTextSet("A", query, panelEditPage);
  await expect(panelEditPage.refreshPanel()).toBeOK();

  // Wait until the response carrying our marker arrives. Other refreshes
  // (defaults, autocomplete probes) may fire alongside; pick the one whose
  // request rawSql contains the marker.
  await expect
    .poll(
      () =>
        responseBodies.some((b) => {
          const v = b?.results?.A?.frames?.[0]?.data?.values?.[0]?.[0];
          return typeof v === "string" && v.includes(userMarker);
        }),
      { timeout: 30000 }
    )
    .toBe(true);

  // === Aspect #1: UI provides real values for the metadata fields ===
  const ourRequest = requestBodies.find((b) =>
    b?.queries?.some((q: any) => typeof q?.rawSql === "string" && q.rawSql.includes(userMarker))
  );
  expect(ourRequest, "expected a /api/ds/query POST carrying the test query").toBeTruthy();
  const target = ourRequest!.queries.find((q: any) => q.rawSql?.includes(userMarker));
  const meta = target?.meta?.grafana;
  expect(meta, "target.meta.grafana must be present on every query").toBeTruthy();

  // Every field the frontend wires up from DataQueryRequest must appear on
  // the wire. Drop one in src/datasource.ts and these assertions break.
  for (const field of [
    "panelId",
    "panelName",
    "panelPluginId",
    "dashboardUID",
    "dashboardTitle",
    "app",
    "requestId",
  ]) {
    expect(meta, `meta.grafana.${field} must be present`).toHaveProperty(field);
  }

  // `app` and `requestId` are populated by Grafana on every request — if
  // either is missing or empty, the wiring in datasource.ts is broken.
  expect(meta.app).toBeTruthy();
  expect(typeof meta.requestId).toBe("string");
  expect(meta.requestId.length).toBeGreaterThan(0);
  // `panelPluginId` is set by Grafana once a visualization is chosen
  // (table after toggleTableView).
  expect(meta.panelPluginId).toBeTruthy();

  // === Aspect #2: managed comment is merged into the SQL SETTINGS clause ===
  const ourResponse = responseBodies.find((b) => {
    const v = b?.results?.A?.frames?.[0]?.data?.values?.[0]?.[0];
    return typeof v === "string" && v.includes(userMarker);
  });
  expect(ourResponse, "expected a response carrying the echoed admin_comment").toBeTruthy();
  const echoed: string = ourResponse!.results.A.frames[0].data.values[0][0];

  // The user-supplied prefix survives untouched.
  expect(echoed).toContain(`custom=${userMarker}`);
  // The plugin's managed block is bracketed by the sentinel markers.
  expect(echoed).toContain("grafana_meta_start");
  expect(echoed).toContain("grafana_meta_end");
  // Real attribution values landed inside the managed block.
  expect(echoed).toMatch(/user_email=[^;]+/);
  expect(echoed).toMatch(/user_login=[^;]+/);
  expect(echoed).toContain("app=dashboard");
  expect(echoed).toMatch(/ref_id=A\b/);
  // PII gate is opt-in: with the toggle ON the email is not the literal
  // string "unknown" — it must be a real value (this also catches a
  // regression where the toggle was never wired through to the backend).
  expect(echoed).not.toContain("user_email=unknown");
});

/**
 * Counterpart to the previous test: with the user-identity toggle left at its
 * default (off), real email / login / name must never leak into the admin
 * comment, even though Grafana has them. This is the privacy-by-default
 * guarantee — if it regresses, this test fails.
 */
test("user telemetry — PII is redacted when toggle is off", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  context,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "user telemetry pii off",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  // Deliberately do NOT enable the includeUserIdentityInAttribution toggle.
  await configPageSteps.saveSuccess(dsConfigPage);

  const responseBodies: any[] = [];
  await context.route("**/api/ds/query**", async (route) => {
    try {
      const response = await route.fetch();
      const body = await response.json();
      responseBodies.push(body);
      await route.fulfill({ response, body: JSON.stringify(body) });
    } catch {
      // route disposed
    }
  });

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("user telemetry pii off");
  await panelEditPage.toggleTableView();

  const userMarker = "e2e_pii_off_" + Date.now();
  await queryTextSet(
    "A",
    `SELECT getSetting('hdx_query_admin_comment') AS c SETTINGS hdx_query_admin_comment='custom=${userMarker}'`,
    panelEditPage
  );
  await expect(panelEditPage.refreshPanel()).toBeOK();

  await expect
    .poll(
      () =>
        responseBodies.some((b) => {
          const v = b?.results?.A?.frames?.[0]?.data?.values?.[0]?.[0];
          return typeof v === "string" && v.includes(userMarker);
        }),
      { timeout: 30000 }
    )
    .toBe(true);

  const echoed: string = responseBodies.find((b) => {
    const v = b?.results?.A?.frames?.[0]?.data?.values?.[0]?.[0];
    return typeof v === "string" && v.includes(userMarker);
  })!.results.A.frames[0].data.values[0][0];

  // Managed block is still present — telemetry feature itself isn't disabled.
  expect(echoed).toContain("grafana_meta_start");
  expect(echoed).toContain("grafana_meta_end");
  // …but the three PII fields are redacted to "unknown".
  expect(echoed).toContain("user_email=unknown");
  expect(echoed).toContain("user_login=unknown");
  expect(echoed).toContain("user_name=unknown");
  // Real values never appear anywhere in the comment.
  expect(echoed).not.toMatch(/user_email=[^u].*[^n].*[^k].*[^n].*[^o].*[^w].*[^n]/);
});
