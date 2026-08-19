import { test, expect, PanelEditPage } from "@grafana/plugin-e2e";
// @ts-ignore
import { closeWhatsNewDialog, ConfigPageSteps, queryTextSet } from "./helpers";

/**
 * Runs sequentially in order to avoid multiple datasource creation.
 */
test.describe.configure({ mode: "serial" });

/**
 * Shared panel page
 */
let panelEditPage: PanelEditPage;

/**
 * Creates one datasource and resets panel page before each test
 */
test.beforeEach(async ({ dashboardPage, createDataSourceConfigPage, page }) => {
  if (panelEditPage === undefined) {
    const dsConfigPage = await ConfigPageSteps.createDatasourceConfigPage(
      "dataTypes tests",
      createDataSourceConfigPage,
    );
    const configPageSteps = new ConfigPageSteps(dsConfigPage.ctx.page);
    await configPageSteps.fillTestNativeDatasource();
    await configPageSteps.saveSuccess(dsConfigPage);
  }
  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("dataTypes tests");
  await panelEditPage.toggleTableView();
});

/**
 * Selects a non-nullable column alongside its Nullable(...) counterpart.
 * Kept to two columns per query: the table panel's grid virtualizes columns
 * that don't fit the editor width, and wider selects (see the 6-column
 * variant this was split from) silently drop trailing headers/cells from
 * the DOM, producing a locator-count mismatch that has nothing to do with
 * converter correctness.
 */
[
  {
    name: "UUID",
    col: "uuid_col",
    nullCol: "uuid_null",
    value: "61f0c404-5cb3-11e7-907b-a6006ad3dba0",
  },
  { name: "IPv4", col: "v4_col", nullCol: "v4_null", value: "1.2.3.4" },
  { name: "IPv6", col: "v6_col", nullCol: "v6_null", value: "2001:db8::1" },
].forEach(({ name, col, nullCol, value }) => {
  test(`${name} columns render as text, NULL renders as an empty cell`, async () => {
    await queryTextSet(
      "A",
      `select ${col}, ${nullCol} from e2e.datatypes order by datetime`,
      panelEditPage,
    );
    await expect(panelEditPage.refreshPanel()).toBeOK();

    // Grafana's table header prepends the field-type icon's accessible name
    // (e.g. "string") to the column name with no separator in textContent,
    // so match by substring rather than exact header text.
    await expect(panelEditPage.panel.fieldNames).toContainText([col, nullCol]);

    // Row 1 (2025-04-09 00:00:00): both columns populated with the same value.
    await expect(panelEditPage.panel.data).toContainText([value, value]);

    // Row 2 (2025-04-09 00:10:00): the non-nullable column is populated, the
    // nullable column is SQL NULL and must render as an empty cell rather
    // than a placeholder string.
    await expect(panelEditPage.panel.data).not.toContainText(["<nil>"]);
    await expect(panelEditPage.panel.data).not.toContainText(["null"]);
  });
});

/**
 * The Hydrolix-shaped column: IPv6 holding IPv4-mapped addresses. Rendering
 * follows the column type, matching ClickHouse's own toString(), so the padded
 * ::ffff: prefix is preserved rather than collapsed to dotted-quad. Kept as its
 * own test because v6_mapped has no Nullable counterpart to pair with.
 */
test("IPv4-mapped addresses in an IPv6 column keep the ::ffff: prefix", async () => {
  await queryTextSet(
    "A",
    "select v6_mapped, v6_col from e2e.datatypes order by datetime",
    panelEditPage,
  );
  await expect(panelEditPage.refreshPanel()).toBeOK();

  await expect(panelEditPage.panel.fieldNames).toContainText([
    "v6_mapped",
    "v6_col",
  ]);
  await expect(panelEditPage.panel.data).toContainText([
    "::ffff:1.2.3.4",
    "::ffff:5.6.7.8",
  ]);
});
