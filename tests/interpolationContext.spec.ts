import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import {
  closeWhatsNewDialog,
  ConfigPageSteps,
  queryTextSet,
} from "./helpers";

/**
 * Interpolation context comes from the editor's props, not from request state
 * cached on the datasource. Two properties are worth proving on the wire, and
 * neither is reachable from a unit test:
 *
 *   1. No preparatory query. Interpolation used to need a `dryRun()` panel run
 *      to populate `datasource.options` first. The proving signal is ordering:
 *      the `/interpolate` POST must land before any `/api/ds/query` POST.
 *   2. The preview follows the time picker. The bounds interpolated into the
 *      SQL must change when the selected range changes.
 *
 * Both tests read the `toDateTime(<epoch>)` literals out of the rendered
 * interpolated SQL, which is what `$__timeFilter` expands to.
 */

const QUERY = "SELECT * FROM e2e.macros WHERE $__timeFilter(datetime)";

/** Ordered log of the two request kinds we care about. */
async function captureRequestOrder(context: any): Promise<string[]> {
  const seen: string[] = [];
  await context.route(
    (url: URL) =>
      url.pathname.includes("/api/ds/query") ||
      url.pathname.includes("/interpolate"),
    async (route: any, request: any) => {
      if (request.method() === "POST") {
        seen.push(
          request.url().includes("/interpolate") ? "interpolate" : "dsQuery"
        );
      }
      try {
        const response = await route.fetch();
        await route.fulfill({ response });
      } catch {
        // route disposed / target closed — ignore
      }
    }
  );
  return seen;
}

const interpolatedPre = (page: any) =>
  page
    .locator("pre")
    .filter({ hasText: /SELECT \* FROM e2e\.macros/i })
    .first();

const epochsIn = (sql: string) =>
  [...sql.matchAll(/toDateTime\((\d+)\)/g)].map((m) => Number(m[1]));

test("interpolates without a preparatory panel query", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  context,
}, testInfo) => {
  testInfo.setTimeout(150_000);

  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "interpolation context no dryrun",
    createDataSourceConfigPage
  );
  await steps.fillTestNativeDatasource();
  await steps.saveSuccess(dsConfigPage);

  const seen = await captureRequestOrder(context);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("interpolation context no dryrun");

  // Deliberately no timeRange.set() and no refreshPanel(): the panel's default
  // range is whatever the dashboard opened with, and nothing has run.
  await queryTextSet("A", QUERY, panelEditPage);

  // Selecting the datasource and typing SQL each provoke panel runs of their
  // own, which say nothing about interpolation. Let those settle, then start
  // from a clean log so the window under test is exactly "the Show click".
  await page.waitForTimeout(4_000);
  seen.length = 0;

  await page.getByRole("button", { name: /Show Interpolated Query/i }).click();

  const copyButton = page.getByRole("button", { name: "copy to clipboard" });
  await expect(copyButton).toBeVisible({ timeout: 60_000 });

  const pre = interpolatedPre(page);
  await expect(pre).toBeVisible();
  await expect(pre).not.toContainText("$__timeFilter");

  // The load-bearing assertion: showing the preview issued an /interpolate POST
  // and did NOT issue a data query. The old dryRun() fallback fired
  // onRunQuery in response to this very click to populate datasource state.
  expect(seen).toContain("interpolate");
  expect(seen).not.toContain("dsQuery");
});

test("interpolated bounds follow the selected time range", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
}, testInfo) => {
  testInfo.setTimeout(180_000);

  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "interpolation context range follows",
    createDataSourceConfigPage
  );
  await steps.fillTestNativeDatasource();
  await steps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("interpolation context range follows");

  await panelEditPage.timeRange.set({
    from: "2025-04-10 00:00:00",
    to: "2025-04-10 23:59:59",
    zone: "Coordinated Universal Time",
  });
  await queryTextSet("A", QUERY, panelEditPage);
  await page.getByRole("button", { name: /Show Interpolated Query/i }).click();

  await expect(
    page.getByRole("button", { name: "copy to clipboard" })
  ).toBeVisible({ timeout: 60_000 });

  const pre = interpolatedPre(page);
  const firstEpochs = epochsIn((await pre.textContent()) ?? "");
  expect(firstEpochs.length).toBeGreaterThan(0);

  // Note: timeRange.set also triggers an implicit panel run, so this test does
  // not isolate "no run happened" - that is the first test's job. What it pins
  // is that the bounds the preview shows track the picker's selection.
  await panelEditPage.timeRange.set({
    from: "2025-04-12 00:00:00",
    to: "2025-04-12 23:59:59",
    zone: "Coordinated Universal Time",
  });

  await expect
    .poll(
      async () => {
        const epochs = epochsIn((await pre.textContent()) ?? "");
        return epochs.length > 0 && epochs[0] !== firstEpochs[0];
      },
      { timeout: 60_000 }
    )
    .toBe(true);

  const secondEpochs = epochsIn((await pre.textContent()) ?? "");
  // 2025-04-12T00:00:00Z, i.e. two days after the first selection.
  expect(secondEpochs[0]).toBe(
    Math.floor(Date.UTC(2025, 3, 12, 0, 0, 0) / 1000)
  );
});
