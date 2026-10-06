import { test, expect, PanelEditPage } from "@grafana/plugin-e2e";
// @ts-ignore
import { closeWhatsNewDialog, ConfigPageSteps, queryTextSet, tableViewSet } from "./helpers";

/**
 * Covers the query editor's validation bar: a debounced EXPLAIN dry-run of the
 * interpolated query plus the unfiltered-primary-key warning. `e2e.macros` is
 * keyed on `datetime`.
 *
 * Runs sequentially in order to avoid multiple datasource creation.
 */
test.describe.configure({ mode: "serial" });

let panelEditPage: PanelEditPage;

test.beforeEach(async ({ dashboardPage, createDataSourceConfigPage, page }) => {
  if (panelEditPage === undefined) {
    const dsConfigPage = await ConfigPageSteps.createDatasourceConfigPage(
      "queryValidation tests",
      createDataSourceConfigPage
    );
    const configPageSteps = new ConfigPageSteps(dsConfigPage.ctx.page);
    await configPageSteps.fillTestNativeDatasource();
    await configPageSteps.saveSuccess(dsConfigPage);
  }
  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryValidation tests");
  await tableViewSet(panelEditPage);
  await panelEditPage.timeRange.set({
    from: "2025-04-10 00:00:00",
    to: "2025-04-10 23:59:59",
    zone: "Coordinated Universal Time",
  });
});

const validationBar = () =>
  panelEditPage.getQueryEditorRow("A").getByTestId("query-validation-bar");

test("reports a query filtered by $__timeFilter() as valid", async () => {
  await queryTextSet(
    "A",
    "SELECT * FROM e2e.macros WHERE $__timeFilter()",
    panelEditPage
  );

  await expect(validationBar()).toContainText("Query is valid", {
    timeout: 30000,
  });
});

test("reports an unknown column as an error", async () => {
  await queryTextSet(
    "A",
    "SELECT no_such_column FROM e2e.macros WHERE $__timeFilter()",
    panelEditPage
  );

  await expect(validationBar()).toContainText("no_such_column", {
    timeout: 30000,
  });
  await expect(validationBar()).not.toContainText("Query is valid");
});

test("warns when the primary key is not filtered, without blocking Run", async () => {
  await queryTextSet(
    "A",
    "SELECT * FROM e2e.macros WHERE v1 > 0",
    panelEditPage
  );

  await expect(validationBar()).toContainText(
    "Primary key `datetime` of `e2e.macros` not filtered in WHERE",
    { timeout: 30000 }
  );

  await expect(panelEditPage.refreshPanel()).toBeOK();
  await expect(panelEditPage.panel.fieldNames).toContainText([
    "datetime",
    "date",
    "v1",
  ]);
});

test("does not dry-run non-SELECT statements", async ({ page }) => {
  const skipped = page.waitForResponse(
    async (response) =>
      response.url().includes("/resources/validate") &&
      (await response.json())?.data?.skipped === true,
    { timeout: 30000 }
  );
  await queryTextSet("A", "DESCRIBE TABLE e2e.macros", panelEditPage);
  await skipped;

  await expect(validationBar()).toBeEmpty();
});
