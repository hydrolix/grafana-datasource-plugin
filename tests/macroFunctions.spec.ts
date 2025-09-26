import { test, expect, PanelEditPage } from "@grafana/plugin-e2e";
// @ts-ignore
import { ConfigPageSteps, queryTextSet, timerangeSet } from "./helpers";

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
test.beforeEach(async ({ dashboardPage, createDataSourceConfigPage }) => {
  if (panelEditPage === undefined) {
    const dsConfigPage = await ConfigPageSteps.createDatasourceConfigPage(
      "macroFunctions tests",
      createDataSourceConfigPage
    );
    const configPageSteps = new ConfigPageSteps(dsConfigPage.ctx.page);
    await configPageSteps.fillTestNativeDatasource();
    await configPageSteps.saveSuccess(dsConfigPage);
  }
  await dashboardPage.goto();
  panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("macroFunctions tests");
  await panelEditPage.toggleTableView();
});

[
  {
    name: "fromTime",
    query:
      "select * from e2e.macros where datetime >= $__fromTime and datetime <= $__toTime",
    from: "2025-04-10 00:20:00",
    to: "2025-04-10 23:59:59",
    expected: [
      ["2025-04-10 00:20:00", "2025-04-10", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10", "3000"],
    ],
  },
  {
    name: "fromTime_ms",
    query:
      "select * from e2e.macros where datetime >= $__fromTime_ms and datetime <= $__toTime_ms",
    from: "2025-04-10 00:20:00",
    to: "2025-04-10 23:59:59",
    expected: [
      ["2025-04-10 00:20:00", "2025-04-10", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10", "3000"],
    ],
  },
  {
    name: "toTime",
    query:
      "select * from e2e.macros where datetime >= $__fromTime and datetime <= $__toTime",
    from: "2025-04-09 00:00:00",
    to: "2025-04-09 00:29:00",
    expected: [
      ["2025-04-09 00:00:00", "2025-04-09", "0"],
      ["2025-04-09 00:10:00", "2025-04-09", "1000"],
      ["2025-04-09 00:20:00", "2025-04-09", "2000"],
    ],
  },
  {
    name: "toTime_ms",
    query:
      "select * from e2e.macros where datetime >= $__fromTime and datetime <= $__toTime_ms",
    from: "2025-04-09 00:00:00",
    to: "2025-04-09 00:29:00",
    expected: [
      ["2025-04-09 00:00:00", "2025-04-09", "0"],
      ["2025-04-09 00:10:00", "2025-04-09", "1000"],
      ["2025-04-09 00:20:00", "2025-04-09", "2000"],
    ],
  },
  {
    name: "dateTimeFilter",
    query: "select * from e2e.macros where $__dateTimeFilter(date, datetime)",
    from: "2025-04-10 00:20:00",
    to: "2025-04-10 23:59:59",
    expected: [
      ["2025-04-10 00:20:00", "2025-04-10", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10", "3000"],
    ],
  },
  {
    name: "dateFilter",
    query: "select * from e2e.macros where $__dateFilter(date)",
    from: "2025-04-10 00:00:00",
    to: "2025-04-10 23:59:59",
    expected: [
      ["2025-04-10 00:10:00", "2025-04-10", "1000"],
      ["2025-04-10 00:20:00", "2025-04-10", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10", "3000"],
    ],
  },
  {
    name: "timeFilter",
    query: "select * from e2e.macros where $__timeFilter(datetime)",
    from: "2025-04-09 01:10:59",
    to: "2025-04-09 01:11:00",
    expected: [["2025-04-09 01:11:00", "2025-04-09", "11100"]],
  },
  {
    name: "timeFilter_ms",
    query: "select * from e2e.macros where $__timeFilter_ms(datetime)",
    from: "2025-04-09 01:10:59",
    to: "2025-04-09 01:11:00",
    expected: [["2025-04-09 01:11:00", "2025-04-09", "11100"]],
  },
  {
    name: "timeInterval",
    query:
      "select $__timeInterval(datetime) as datetime, max(date) as date, max(v1) as v1 from e2e.macros where $__timeFilter_ms(datetime) group by datetime",
    from: "2025-04-09 00:00:00",
    to: "2025-04-19 23:59:59",
    maxDataPoints: 400,
    expected: [
      ["2025-04-09 00:00:00", "2025-04-09 00:00:00", "2000"],
      ["2025-04-09 00:30:00", "2025-04-09 00:00:00", "5000"],
      ["2025-04-09 01:00:00", "2025-04-09 00:00:00", "11100"],
      ["2025-04-10 00:00:00", "2025-04-10 00:00:00", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10 00:00:00", "3000"],
    ],
  },
  {
    name: "timeInterval_ms",
    query:
      "select $__timeInterval_ms(datetime) as datetime, max(date) as date, max(v1) as v1 from e2e.macros where $__timeFilter_ms(datetime) group by datetime",
    maxDataPoints: 400,
    from: "2025-04-09 00:00:00",
    to: "2025-04-19 23:59:59",
    expected: [
      ["2025-04-09 00:00:00", "2025-04-09 00:00:00", "2000"],
      ["2025-04-09 00:30:00", "2025-04-09 00:00:00", "5000"],
      ["2025-04-09 01:00:00", "2025-04-09 00:00:00", "11100"],
      ["2025-04-10 00:00:00", "2025-04-10 00:00:00", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10 00:00:00", "3000"],
    ],
  },
  {
    name: "interval_s",
    query:
      "select toStartOfInterval(datetime, INTERVAL $__interval_s second) as datetime, max(date) as date, max(v1) as v1 from e2e.macros where $__timeFilter_ms(datetime) group by datetime",
    maxDataPoints: 400,
    from: "2025-04-09 00:00:00",
    to: "2025-04-19 23:59:59",
    expected: [
      ["2025-04-09 00:00:00", "2025-04-09 00:00:00", "2000"],
      ["2025-04-09 00:30:00", "2025-04-09 00:00:00", "5000"],
      ["2025-04-09 01:00:00", "2025-04-09 00:00:00", "11100"],
      ["2025-04-10 00:00:00", "2025-04-10 00:00:00", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10 00:00:00", "3000"],
    ],
  },
  {
    name: "round",
    query: "select * from e2e.macros where $__timeFilter_ms(datetime)",
    from: "2025-04-10 00:20:30",
    to: "2025-04-11 00:01:30",
    round: "5m",
    expected: [
      ["2025-04-10 00:20:00", "2025-04-10", "2000"],
      ["2025-04-10 00:30:00", "2025-04-10 00:00:00", "3000"],
    ],
  },
].forEach(({ name, query, from, to, expected, maxDataPoints, round }) => {
  test(`testing ${name}`, async () => {
    await queryTextSet("A", query, panelEditPage);
    await panelEditPage.timeRange.set({
      from,
      to,
      zone: "Coordinated Universal Time",
    });

    if (maxDataPoints) {
      // set interval to 30m for TimeInterval
      await panelEditPage
        .getByGrafanaSelector("Query operation row title")
        .click();
      await panelEditPage.ctx.page
        .locator(
          'xpath=//label[contains(text(), "Max data points")]/../div[1]//input'
        )
        .fill(maxDataPoints.toString());
      await panelEditPage
        .getByGrafanaSelector("Query operation row title")
        .click();
    }
    if (round) {
      const queryRow = panelEditPage.getQueryEditorRow("A");
      await queryRow.getByTestId("data-testid round input").fill(round);
    }
    await expect(panelEditPage.refreshPanel()).toBeOK();
    await expect(panelEditPage.panel.fieldNames).toContainText([
      "datetime",
      "date",
      "v1",
    ]);
    expected.forEach(async (v) => {
      await expect(panelEditPage.panel.data).toContainText(v);
    });
    await expect(panelEditPage.panel.data).not.toContainText([
      "2025-04-08 23:59:59",
      "2025-04-08",
      "-1",
    ]);
    await expect(panelEditPage.panel.data).not.toContainText([
      "2025-04-11 00:00:01",
      "2025-04-11",
      "-1",
    ]);
  });
});
