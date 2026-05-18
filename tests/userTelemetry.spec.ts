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
  gotoDashboardPage,
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
  // Grafana's <Switch> shape changed in 11.x: pre-11 it exposed only a
  // visually-hidden <label aria-label="Toggle switch">, 11+ exposes
  // role="switch" on the input. Mirror the .or() fallback used by
  // ElementContext.switch() in tests/helpers.ts so this works on every
  // supported Grafana version.
  const userIdentityToggle = page.getByTestId(
    "data-testid hdx_includeUserIdentityInAttribution"
  );
  await userIdentityToggle
    .getByRole("switch")
    .or(userIdentityToggle.getByLabel("Toggle switch"))
    .first()
    .click({ force: true });

  await configPageSteps.saveSuccess(dsConfigPage);

  // Create a saved dashboard via the HTTP API, with the marker query baked
  // into the panel target. Two reasons we don't go through the panel-edit
  // UI here:
  //   1. The default flow (dashboardPage.addPanel on an unsaved dashboard)
  //      leaves panelName and dashboardTitle as undefined on DataQueryRequest;
  //      JSON.stringify drops undefined keys, so the wire never carries them
  //      and the assertions below can't see the wiring.
  //   2. Panel-edit mode reports app="panel-editor" but this test asserts
  //      app=dashboard — we want the dashboard-view refresh path.
  const userMarker = "e2e_user_supplied_" + Date.now();
  const query = `SELECT getSetting('hdx_query_admin_comment') AS c SETTINGS hdx_query_admin_comment='custom=${userMarker}'`;
  const dashboardTitle = `user-telemetry-${Date.now()}`;
  const panelName = "Telemetry Test Panel";
  const dsRef = {
    type: dsConfigPage.datasource.type,
    uid: dsConfigPage.datasource.uid,
  };
  const createResp = await page.request.post("/api/dashboards/db", {
    data: {
      dashboard: {
        title: dashboardTitle,
        schemaVersion: 38,
        panels: [
          {
            id: 1,
            title: panelName,
            type: "table",
            datasource: dsRef,
            targets: [{ refId: "A", datasource: dsRef, rawSql: query }],
            gridPos: { x: 0, y: 0, w: 24, h: 8 },
          },
        ],
        time: { from: "now-6h", to: "now" },
      },
      overwrite: true,
    },
  });
  const { uid } = await createResp.json();

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

  // The marker query (baked into the dashboard JSON above) asks the server
  // to echo back the setting value it actually applied. The user-supplied
  // marker proves the SETTINGS clause from the SQL survived; the plugin's
  // managed block proves the merge ran.
  const dashboardPage = await gotoDashboardPage({ uid });
  await closeWhatsNewDialog(page);
  await dashboardPage.refreshDashboard();

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

  // Fields Grafana populates on every DataQueryRequest in every supported
  // version. Drop one in src/datasource.ts and these break.
  for (const field of [
    "panelId",
    "panelPluginId",
    "dashboardUID",
    "app",
    "requestId",
  ]) {
    expect(meta, `meta.grafana.${field} must be present`).toHaveProperty(field);
  }

  // `app` and `requestId` are populated by Grafana on every request — if
  // either is missing or empty, the wiring in datasource.ts is broken.
  expect(meta.app).toBe("dashboard");
  expect(typeof meta.requestId).toBe("string");
  expect(meta.requestId.length).toBeGreaterThan(0);
  // `panelPluginId` reflects the saved panel's visualization type.
  expect(meta.panelPluginId).toBe("table");
  expect(meta.dashboardUID).toBe(uid);

  // `panelName` and `dashboardTitle` were added to DataQueryRequest in
  // Grafana 11.x and are absent on 10.x — JSON.stringify drops undefined
  // keys, so the wire shape varies by version. The wiring itself is
  // covered exhaustively by unit tests in src/datasource.test.ts; here we
  // only assert the values are correct when Grafana provides them.
  if (meta.panelName !== undefined) {
    expect(meta.panelName).toBe(panelName);
  }
  if (meta.dashboardTitle !== undefined) {
    expect(meta.dashboardTitle).toBe(dashboardTitle);
  }

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
