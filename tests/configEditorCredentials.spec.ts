import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import { ConfigPageSteps } from "./helpers";

const USER_ACCOUNT = "User Account";
const SERVICE_ACCOUNT = "Service Account";
const FORWARD_OAUTH = "Forward OAuth Identity";
const FORWARD_EXCHANGE = "Forward OAuth + Exchange";

/**
 * The credentials picker decides which secret the datasource holds, so each
 * mode's fields are a user-visible contract rather than cosmetics. The
 * exchanging mode's whole point is that it asks for nothing: its delegate
 * credential is per cluster and lives in Grafana's server configuration, so a
 * credential field appearing here would mean a secret had landed on the
 * datasource.
 */
test("config editor: each credentials mode asks for its own fields", async ({
  createDataSourceConfigPage,
  page,
}) => {
  const configPage = ConfigPageSteps.getLocator(page);
  await ConfigPageSteps.createDatasourceConfigPage(
    "credentials",
    createDataSourceConfigPage
  );

  await expect(configPage.credentialsType()).toBeVisible();

  // User Account is the default and asks for a username and password.
  await expect(configPage.username()).toBeVisible();
  await expect(configPage.password()).toBeVisible();
  await expect(configPage.exchangeAudience()).not.toBeVisible();

  await configPage.credentialsTypeItem(SERVICE_ACCOUNT).check({ force: true });
  await expect(configPage.username()).not.toBeVisible();
  await expect(configPage.password()).not.toBeVisible();
  await expect(configPage.exchangeAudience()).not.toBeVisible();

  await configPage.credentialsTypeItem(FORWARD_OAUTH).check({ force: true });
  await expect(
    configPage.exchangeAudience(),
    "plain forwarding has no audience to exchange for"
  ).not.toBeVisible();

  await configPage.credentialsTypeItem(FORWARD_EXCHANGE).check({ force: true });
  await expect(
    configPage.username(),
    "the exchanging mode must not ask for a credential"
  ).not.toBeVisible();
  await expect(configPage.password()).not.toBeVisible();
  await expect(
    configPage.exchangeAudience(),
    "its one field is the cluster audience, which is not a secret"
  ).toBeVisible();

  // Optional: empty means the host, which is the audience on every cluster the
  // console registers today.
  await expect(configPage.exchangeAudience()).toHaveValue("");
  await configPage.exchangeAudience().fill("cluster.example.hydrolix.net");
  await expect(configPage.exchangeAudience()).toHaveValue(
    "cluster.example.hydrolix.net"
  );

  // Going back leaves nothing of the exchanging mode behind.
  await configPage.credentialsTypeItem(USER_ACCOUNT).check({ force: true });
  await expect(configPage.exchangeAudience()).not.toBeVisible();
  await expect(configPage.username()).toBeVisible();
});
