import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import { closeWhatsNewDialog, ConfigPageSteps, queryTextSet } from "./helpers";
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

test("should show beautified ClickHouse error in query row on syntax error", async ({
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
      const response = await route.fetch();
      await route.fulfill({ response });
    }
  });

  await queryTextSet("A", "select ", panelEditPage);
  await page.keyboard.press("Control+Space");
  wait(2000);

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
