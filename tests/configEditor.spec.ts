import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import { ConfigPageSteps } from "./helpers";

test("smoke: should render config editor", async ({
  createDataSourceConfigPage,
  page,
}) => {
  const configPage = ConfigPageSteps.getLocator(page);
  await ConfigPageSteps.createDatasourceConfigPage(
    "render",
    createDataSourceConfigPage
  );

  await expect(configPage.host()).toBeVisible();
  await expect(configPage.port()).toBeVisible();

  await expect(configPage.useDefaultPortSwitch()).toBeVisible();
  await expect(configPage.useDefaultPortSwitch()).toBeChecked();
  await configPage.useDefaultPortSwitch().uncheck({ force: true });
  await expect(configPage.useDefaultPortSwitch()).not.toBeChecked();

  await expect(
    configPage.path(),
    "path shouldn't be visible for native (default)"
  ).not.toBeVisible();

  await expect(configPage.protocol()).toBeVisible();
  await expect(configPage.protocolItem("native")).toBeChecked();
  await expect(configPage.protocolItem("http")).not.toBeChecked();
  await configPage.protocolItem("http").check({ force: true });
  await expect(configPage.protocolItem("native")).not.toBeChecked();
  await expect(configPage.protocolItem("http")).toBeChecked();

  await expect(
    configPage.path(),
    "path should be visible for http connection"
  ).toBeVisible();

  await expect(configPage.secureSwitch()).toBeVisible();
  await expect(configPage.secureSwitch()).toBeChecked();
  await configPage.secureSwitch().uncheck({ force: true, timeout: 5000 });
  await expect(configPage.secureSwitch()).not.toBeChecked();

  await expect(configPage.skipTlsVerifySwitch()).not.toBeVisible();
  await configPage.secureSwitch().check({ force: true });
  await expect(configPage.skipTlsVerifySwitch()).toBeVisible();
  await expect(configPage.skipTlsVerifySwitch()).not.toBeChecked();
  await configPage.skipTlsVerifySwitch().check({ force: true });
  await expect(configPage.skipTlsVerifySwitch()).toBeChecked();

  await expect(configPage.username()).toBeVisible();
  await expect(configPage.password()).toBeVisible();

  await expect(configPage.defaultDatabase()).not.toBeVisible();
  await expect(configPage.defaultRound()).not.toBeVisible();
  await expect(configPage.adHocTableVariable()).not.toBeVisible();
  await expect(configPage.adHocTimeColumnVariable()).not.toBeVisible();
  await expect(configPage.adHocDefaultTimeRangeTimeselect()).not.toBeVisible();
  await expect(configPage.dialTimeout()).not.toBeVisible();
  await expect(configPage.queryTimeout()).not.toBeVisible();

  await expect(configPage.additionalSettingsExpandable()).toBeVisible();
  await configPage.additionalSettingsExpandable().click({ force: true });

  await expect(configPage.defaultDatabase()).toBeVisible();
  await expect(configPage.defaultRound()).toBeVisible();
  await expect(configPage.adHocTableVariable()).toBeVisible();
  await expect(configPage.adHocTimeColumnVariable()).toBeVisible();
  await expect(configPage.adHocDefaultTimeRangeTimeselect()).toBeVisible();
  await expect(configPage.dialTimeout()).toBeVisible();
  await expect(configPage.queryTimeout()).toBeVisible();
});

test('"Mandatory fields validation', async ({
  createDataSourceConfigPage,
  page,
}) => {
  const configPage = ConfigPageSteps.getLocator(page);
  await ConfigPageSteps.createDatasourceConfigPage(
    "mandatory fields",
    createDataSourceConfigPage
  );

  await configPage.host().fill("");
  await configPage.host().press("Tab");
  await expect(configPage.hostAlert()).toBeVisible();

  await configPage.useDefaultPortSwitch().uncheck({ force: true });
  await expect(configPage.port()).toBeEnabled();
  await configPage.port().fill("0");
  await configPage.port().press("Tab");
  await expect(configPage.portAlert()).toBeVisible();
});

test('"Save & test" should be successful when configuration is valid', async ({
  createDataSourceConfigPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "valid config",
    createDataSourceConfigPage
  );

  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  await configPageSteps.fillTestHttpDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);
});

test('"Save & test" should fail when configuration is invalid', async ({
  createDataSourceConfigPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "invalid config",
    createDataSourceConfigPage
  );
  const configPage = configPageSteps.configPageLocator;

  await configPage.host().fill("");
  await configPageSteps.saveError("Server address is missing", dsConfigPage);
});
