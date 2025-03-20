import { test, expect } from "@grafana/plugin-e2e";
import { HdxDataSourceOptions, HdxSecureJsonData } from "../src/types";
import allLabels from "../src/labels";

test("smoke: should render config editor", async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource({ fileName: "datasources.yml" });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(
    page.getByLabel(allLabels.components.config.editor.host.label)
  ).toBeVisible();
});

/*test('"Save & test" should be successful when configuration is valid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<MyDataSourceOptions, MySecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'Username' }).fill(ds.jsonData.username ?? '');
  await page.getByRole('textbox', { name: 'Password' }).fill(ds.secureJsonData?.password ?? '');
  await expect(configPage.saveAndTest()).toBeOK();
});*/

test('"Save & test" should fail when configuration is invalid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<
    HdxDataSourceOptions,
    HdxSecureJsonData
  >({ fileName: "datasources.yml" });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByLabel(allLabels.components.config.editor.host.label).fill("");
  await expect(configPage.saveAndTest()).not.toBeOK();
  await expect(configPage).toHaveAlert("error", {
    hasText: "Server address is missing",
  });
});
