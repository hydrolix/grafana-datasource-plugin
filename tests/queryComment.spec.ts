import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import {
  captureRequestBodies,
  closeWhatsNewDialog,
  ConfigPageSteps,
} from "./helpers";
import { QueryEditorRow } from "./queryEditorRow";
import { openGrafanaSelect, pickOptionByPrefix } from "./grafanaSelect";
import { HDX_QUERY_COMMENT_DEFAULT } from "../src/queryCommentDefault";

/**
 * #10 — hdx_query_comment + canonical attribution defaults
 *
 * Exercises the UX added by `plugin-query-comment-defaults`:
 *   - Picking `hdx_query_comment` or `hdx_query_admin_comment` from the
 *     QuerySettings setting-picker pre-fills the input with the canonical
 *     attribution template (HDX_QUERY_COMMENT_DEFAULT).
 *   - On the wire, the synthetic `${__hydrolix.*}` placeholders expand to
 *     the actual request context (panelId / panelName / app / refId).
 *
 * react-select handling follows the project memory's locator rule —
 * `openGrafanaSelect` + `pickOptionByPrefix`, never raw role-based locators
 * for the option list.
 */

test("hdx_query_comment pre-fills with the canonical default", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor commentDefault",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor commentDefault");

  const row = new QueryEditorRow(panelEditPage, "A");
  await row.setSql("SELECT 1");
  await row.openQuerySettings();

  // Pick `hdx_query_comment` from the setting-picker. Intentionally do NOT
  // call addQuerySetting (which would fill the input and erase the pre-fill).
  await row.row.getByLabel("new setting").click();
  await openGrafanaSelect(row.row);
  await pickOptionByPrefix(page, "hdx_query_comment");

  const input = row.row.getByLabel("hdx_query_comment");
  await expect(input).toHaveValue(HDX_QUERY_COMMENT_DEFAULT);
});

test("hdx_query_admin_comment also pre-fills with the canonical default", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor adminCommentDefault",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor adminCommentDefault");

  const row = new QueryEditorRow(panelEditPage, "A");
  await row.setSql("SELECT 1");
  await row.openQuerySettings();

  await row.row.getByLabel("new setting").click();
  await openGrafanaSelect(row.row);
  await pickOptionByPrefix(page, "hdx_query_admin_comment");

  const input = row.row.getByLabel("hdx_query_admin_comment");
  await expect(input).toHaveValue(HDX_QUERY_COMMENT_DEFAULT);
});

test("synthetic ${__hydrolix.*} placeholders expand on the wire", async ({
  createDataSourceConfigPage,
  dashboardPage,
  page,
  context,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "queryEditor commentExpansion",
    createDataSourceConfigPage
  );
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await dashboardPage.goto();
  await closeWhatsNewDialog(page);
  const panelEditPage = await dashboardPage.addPanel();
  await panelEditPage.datasource.set("queryEditor commentExpansion");

  const row = new QueryEditorRow(panelEditPage, "A");
  await row.setSql("SELECT 1");
  await row.openQuerySettings();

  // Pick the comment setting (input keeps the pre-filled default).
  await row.row.getByLabel("new setting").click();
  await openGrafanaSelect(row.row);
  await pickOptionByPrefix(page, "hdx_query_comment");

  const bodies = await captureRequestBodies(context, "**/api/ds/query**");
  await expect(panelEditPage.refreshPanel()).toBeOK();

  expect(
    bodies.length,
    "expected to capture the /api/ds/query body"
  ).toBeGreaterThan(0);
  const parsed = JSON.parse(bodies[0]);
  const querySettings = parsed?.queries?.[0]?.querySettings;
  const comment = (querySettings as any[]).find(
    (s) => s.setting === "hdx_query_comment"
  );
  expect(comment, "querySettings must contain hdx_query_comment").toBeDefined();
  const value: string = comment.value;

  // Proof of expansion: the synthetic placeholders are gone from the
  // outgoing value.
  expect(value).not.toContain("${__hydrolix.app}");
  expect(value).not.toContain("${__hydrolix.ref_id}");
  expect(value).not.toContain("${__hydrolix.panel.id}");
  expect(value).not.toContain("${__hydrolix.panel.name}");

  // Concrete content: dashboard context + the refId we used.
  expect(value).toMatch(/grafana_app=\w+/);
  expect(value).toContain("grafana_ref_id=A");
});
