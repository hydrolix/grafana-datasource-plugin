import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import {
  captureRequestBodies,
  captureRequestsAndResponses,
  closeWhatsNewDialog,
  ConfigPageSteps,
} from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";

/**
 * The dashboard JSON model is stable across Grafana 10–13; the
 * annotation field-mapping UI is not. These tests prove the
 * frontend integration end-to-end at three levels:
 *
 *   1. Request side  — prepareQuery (adds source: 'annotation') and
 *      datasource.query() retag (sets app: 'annotation') both fire,
 *      so the POST /api/ds/query body carries source: 'annotation'.
 *
 *   2. Response side — the backend returns data frames the frontend can
 *      consume (schema has `time` and, for region annotations, `timeEnd`).
 *      This is what feeds Grafana's annotation overlay.
 *
 *   3. Chrome side   — the dashboard's annotation submenu wrapper renders
 *      with the annotation name visible. Grafana's on-panel marker is
 *      drawn directly onto uPlot's canvas with no stable test-id, so the
 *      submenu wrapper is the highest-fidelity DOM signal we can assert
 *      reliably across Grafana 10–13 (`annotationsWrapper` has been
 *      versioned since 10.0.0).
 *
 * SQL uses hardcoded SELECTs so no ClickHouse table or fixture is
 * required — only that ClickHouse is reachable for the backend to
 * run the query against.
 */

function findAnnotationRequest(bodies: string[]): {
  body: any;
  query: any;
} | undefined {
  for (const raw of bodies) {
    let body: any;
    try {
      body = JSON.parse(raw);
    } catch {
      continue;
    }
    const queries: any[] = body?.queries ?? [];
    const q = queries.find((t) => t?.source === "annotation");
    if (q) return { body, query: q };
  }
  return undefined;
}

function findAnnotationExchange(
  requests: string[],
  responses: string[],
): { query: any; refId: string; frames: any[] } | undefined {
  for (let i = 0; i < requests.length; i++) {
    let req: any;
    try {
      req = JSON.parse(requests[i]);
    } catch {
      continue;
    }
    const queries: any[] = req?.queries ?? [];
    const q = queries.find((t) => t?.source === "annotation");
    if (!q) continue;
    let resp: any;
    try {
      resp = JSON.parse(responses[i]);
    } catch {
      continue;
    }
    const frames: any[] = resp?.results?.[q.refId]?.frames ?? [];
    return { query: q, refId: q.refId, frames };
  }
  return undefined;
}

function fieldNamesOf(frame: any): string[] {
  return (frame?.schema?.fields ?? []).map((f: any) => f.name);
}

test("instant annotation: source='annotation' on the wire, time frame back, chip in chrome", async ({
  createDataSourceConfigPage,
  gotoDashboardPage,
  page,
  context,
}) => {
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "annotation instant",
    createDataSourceConfigPage,
  );
  await steps.fillTestNativeDatasource();
  await steps.saveSuccess(dsConfigPage);

  const instantSql =
    "SELECT toDateTime(1744286400) AS time, 'deploy' AS text, 'ci,prod' AS tags";

  const annotationName = "deploys";
  const { uid } = await new DashboardBuilder(page, dsConfigPage.datasource)
    .withTitle(`annotations-instant-${Date.now()}`)
    .addAnnotation({ name: annotationName, rawSql: instantSql })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange("2025-04-10T00:00:00.000Z", "2025-04-10T23:59:59.000Z")
    .create();

  const { requests, responses } = await captureRequestsAndResponses(context);

  const dashboardPage = await gotoDashboardPage({ uid });
  await closeWhatsNewDialog(page);
  await dashboardPage.refreshDashboard();

  // 1. Request side: source: 'annotation' reached the backend.
  await expect
    .poll(() => findAnnotationRequest(requests) !== undefined, {
      timeout: 30000,
    })
    .toBe(true);

  const req = findAnnotationRequest(requests)!;
  expect(req.query.rawSql).toContain("AS time");
  expect(req.query.rawSql).not.toContain("timeEnd");
  expect(req.query.source).toBe("annotation");

  // 2. Response side: the backend returned a data frame with a `time` field,
  //    which is what Grafana feeds into its annotation overlay layer.
  await expect
    .poll(() => findAnnotationExchange(requests, responses) !== undefined, {
      timeout: 30000,
    })
    .toBe(true);

  const exchange = findAnnotationExchange(requests, responses)!;
  expect(exchange.frames.length).toBeGreaterThan(0);
  const frameFields = fieldNamesOf(exchange.frames[0]);
  expect(frameFields).toContain("time");
  expect(frameFields).not.toContain("timeEnd");

  // 3. Chrome side: the annotation submenu renders an enable toggle for the
  //    annotation, checked because the annotation is enabled. The toggle role
  //    differs across versions:
  //      - Grafana 10.x:    role="checkbox"
  //      - Grafana 11+:     role="switch" (Scenes layout)
  //    Both expose the annotation name as the accessible name, so we match by
  //    name across either role.
  const annotationToggle = page
    .getByRole("switch", {name: annotationName, exact: true})
    .or(page.getByRole("checkbox", {name: annotationName, exact: true}));
  await expect(annotationToggle.first()).toBeVisible({timeout: 30000});
  await expect(annotationToggle.first()).toBeChecked();
});

test("region annotation: source='annotation' on the wire, timeEnd frame back, chip in chrome", async ({
  createDataSourceConfigPage,
  gotoDashboardPage,
  page,
  context,
}) => {
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "annotation region",
    createDataSourceConfigPage,
  );
  await steps.fillTestNativeDatasource();
  await steps.saveSuccess(dsConfigPage);

  const regionSql =
    "SELECT toDateTime(1744286400) AS time, toDateTime(1744290000) AS timeEnd, 'maintenance' AS text";

  const annotationName = "windows";
  const { uid } = await new DashboardBuilder(page, dsConfigPage.datasource)
    .withTitle(`annotations-region-${Date.now()}`)
    .addAnnotation({ name: annotationName, rawSql: regionSql })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange("2025-04-10T00:00:00.000Z", "2025-04-10T23:59:59.000Z")
    .create();

  const { requests, responses } = await captureRequestsAndResponses(context);

  const dashboardPage = await gotoDashboardPage({ uid });
  await closeWhatsNewDialog(page);
  await dashboardPage.refreshDashboard();

  // 1. Request side: source: 'annotation' + timeEnd in SQL.
  await expect
    .poll(() => findAnnotationRequest(requests) !== undefined, {
      timeout: 30000,
    })
    .toBe(true);

  const req = findAnnotationRequest(requests)!;
  expect(req.query.rawSql).toContain("AS timeEnd");
  expect(req.query.source).toBe("annotation");

  // 2. Response side: data frame schema carries both `time` and `timeEnd` —
  //    the shape Grafana needs to render a region (band) annotation.
  await expect
    .poll(() => findAnnotationExchange(requests, responses) !== undefined, {
      timeout: 30000,
    })
    .toBe(true);

  const exchange = findAnnotationExchange(requests, responses)!;
  expect(exchange.frames.length).toBeGreaterThan(0);
  const frameFields = fieldNamesOf(exchange.frames[0]);
  expect(frameFields).toContain("time");
  expect(frameFields).toContain("timeEnd");

  // 3. Chrome side: the annotation submenu renders an enable toggle for the
  //    annotation, checked because the annotation is enabled. The toggle role
  //    differs across versions:
  //      - Grafana 10.x:    role="checkbox"
  //      - Grafana 11+:     role="switch" (Scenes layout)
  //    Both expose the annotation name as the accessible name, so we match by
  //    name across either role.
  const annotationToggle = page
    .getByRole("switch", {name: annotationName, exact: true})
    .or(page.getByRole("checkbox", {name: annotationName, exact: true}));
  await expect(annotationToggle.first()).toBeVisible({timeout: 30000});
  await expect(annotationToggle.first()).toBeChecked();
});

test("annotation with blank SQL does not fire an annotation request", async ({
  createDataSourceConfigPage,
  gotoDashboardPage,
  page,
  context,
}) => {
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "annotation blank",
    createDataSourceConfigPage,
  );
  await steps.fillTestNativeDatasource();
  await steps.saveSuccess(dsConfigPage);

  const { uid } = await new DashboardBuilder(page, dsConfigPage.datasource)
    .withTitle(`annotations-blank-${Date.now()}`)
    .addAnnotation({ name: "blank", rawSql: "" })
    .addPanel({ rawSql: "SELECT 1 AS v" })
    .withTimeRange("2025-04-10T00:00:00.000Z", "2025-04-10T23:59:59.000Z")
    .create();

  const bodies = await captureRequestBodies(context);

  const dashboardPage = await gotoDashboardPage({ uid });
  await closeWhatsNewDialog(page);
  await dashboardPage.refreshDashboard();

  // The panel query has to land before we can be confident no annotation
  // request is in flight. Wait for any /api/ds/query POST to arrive.
  await expect.poll(() => bodies.length > 0, { timeout: 30000 }).toBe(true);

  // Give the annotation pipeline a beat to potentially fire — if it doesn't,
  // we want to assert that, not race the test to green.
  await page.waitForTimeout(2000);

  expect(findAnnotationRequest(bodies)).toBeUndefined();
});
