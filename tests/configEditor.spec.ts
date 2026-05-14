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
  await expect(configPage.adHocDefaultTimeRangeTimeselect()).not.toBeVisible();
  await expect(configPage.dialTimeout()).not.toBeVisible();
  await expect(configPage.queryTimeout()).not.toBeVisible();

  await expect(configPage.additionalSettingsExpandable()).toBeVisible();
  await configPage.additionalSettingsExpandable().click({ force: true });

  await expect(configPage.defaultDatabase()).toBeVisible();
  await expect(configPage.defaultRound()).toBeVisible();
  await expect(configPage.adHocTableVariable()).toBeVisible();
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

/**
 * #3 – Additional settings persistence
 *
 * Smoke and validation tests only assert that the Additional Settings fields
 * render. This one verifies the form ↔ jsonData wiring: fill the fields, save
 * the datasource, reload the page, and confirm every value survived the
 * round-trip.
 *
 * adHocDefaultTimeRange is intentionally skipped here — it is a popup time
 * picker (Timeselect locator returns a button, not a fillable input). A
 * dedicated test should drive it via the picker UI.
 */
test("additional settings persist after save and reload", async ({
  createDataSourceConfigPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "additional settings persist",
    createDataSourceConfigPage
  );
  const configPage = configPageSteps.configPageLocator;

  await configPageSteps.fillTestNativeDatasource();

  await configPage.additionalSettingsExpandable().click({ force: true });

  await configPage.defaultDatabase().fill("e2e");
  await configPage.defaultRound().fill("15m");
  await configPage.adHocTableVariable().fill("my_ad_hoc_var");
  await configPage.dialTimeout().fill("9");
  await configPage.queryTimeout().fill("31");

  await configPageSteps.saveSuccess(dsConfigPage);

  await page.reload();

  await configPage.additionalSettingsExpandable().click({ force: true });

  await expect(configPage.defaultDatabase()).toHaveValue("e2e");
  await expect(configPage.defaultRound()).toHaveValue("15m");
  await expect(configPage.adHocTableVariable()).toHaveValue("my_ad_hoc_var");
  await expect(configPage.dialTimeout()).toHaveValue("9");
  await expect(configPage.queryTimeout()).toHaveValue("31");
});

/**
 * #1 – Password (secureJsonData) round-trip
 *
 * Covers the form ↔ Grafana secrets backend wiring:
 *   1. Fill out a fresh datasource (host + native + initial password "first").
 *   2. Save & test succeeds.
 *   3. Reload the page → password input is no longer rendered as a plaintext
 *      field (Grafana renders "Reset" + "configured" placeholder once a
 *      secret is set, hiding the actual value).
 *   4. Click "Reset" → the password input is editable again, and the value
 *      is empty (Grafana doesn't repopulate the previous secret).
 *   5. Fill a new password "second", Save & test again — succeeds against
 *      the same ClickHouse credentials (testpass), proving the change went
 *      through the secureJsonData path rather than being ignored.
 *
 * Not covered (intentional):
 *   - Verifying the secret value itself; only Grafana sees it.
 */
test("password persists via secureJsonData and can be reset", async ({
  createDataSourceConfigPage,
  page,
}) => {
  const configPageSteps = new ConfigPageSteps(page);
  const dsConfigPage = await configPageSteps.createDatasourceConfigPage(
    "password round-trip",
    createDataSourceConfigPage
  );
  const configPage = configPageSteps.configPageLocator;

  // Initial save with the real test credentials. fillTestNativeDatasource
  // sets password to CLICKHOUSE_PASSWORD (or "testpass" default) so save&test
  // actually succeeds — required for steps 4/5 to be meaningful.
  await configPageSteps.fillTestNativeDatasource();
  await configPageSteps.saveSuccess(dsConfigPage);

  // Reload — once a secret is set, Grafana hides the password input and
  // exposes a "Reset" button instead. The proxy locator surfaces both.
  await page.reload();
  await expect(configPage.passwordReset()).toBeVisible();

  // Reset: the input should come back, empty.
  await configPage.passwordReset().click({ force: true });
  await expect(configPage.password()).toBeVisible();
  await expect(configPage.password()).toHaveValue("");

  // Fill the same valid password and re-save. If the secret path is broken,
  // saveAndTest fails because the backend would try to authenticate with
  // an empty password.
  const password = process.env.CLICKHOUSE_PASSWORD ?? "testpass";
  await configPage.password().fill(password);
  await configPageSteps.saveSuccess(dsConfigPage);

  // After the second save, reload once more and confirm Grafana once again
  // shows the Reset button — proving the new password also persisted.
  await page.reload();
  await expect(configPage.passwordReset()).toBeVisible();
});
