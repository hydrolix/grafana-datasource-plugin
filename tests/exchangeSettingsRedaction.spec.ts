import { test, expect } from "@grafana/plugin-e2e";

/**
 * The delegate credentials for the "Forward OAuth + Exchange" mode are a
 * `[plugin.hydrolix-hydrolix-datasource]` key in Grafana's own configuration,
 * and Grafana shows that whole file to every server admin at
 * /api/admin/settings (Administration → General → Settings). A value is
 * masked there only when its key matches one of setting.RedactedValue's
 * patterns (PASSWORD, SECRET, PRIVATE_KEY, …_TOKEN$, …), so the key is named
 * `exchange_client_secrets` to match SECRET. This pins the name: rename it to
 * something the patterns miss and every cluster's delegate secret is shown in
 * plain text.
 *
 * Both e2e stacks mount e2e/grafana.ini, which declares the key with a
 * non-empty placeholder; the real value only ever reaches the plugin process.
 * Grafana returns "" for an empty value without consulting the patterns, so a
 * non-empty placeholder is what makes this assertion mean anything.
 */
const SECTION = "plugin.hydrolix-hydrolix-datasource";
/** setting.RedactedPassword, unchanged from Grafana 11 through main. */
const REDACTED = "*********";

test("the exchange client secrets are masked in Grafana's admin settings", async ({ request }) => {
  const response = await request.get("/api/admin/settings");
  expect(response.ok(), `GET /api/admin/settings: ${response.status()}`).toBeTruthy();
  const settings: Record<string, Record<string, string>> = await response.json();

  const section = settings[SECTION];
  expect(section, `e2e/grafana.ini declares [${SECTION}]`).toBeDefined();
  // The endpoint, a non-secret in the same section, is shown as written. Not
  // every key is masked, so a masked value is the pattern's doing.
  expect(section.exchange_url).toBeTruthy();
  expect(section.exchange_url).not.toBe(REDACTED);
  expect(section.exchange_client_secrets).toBe(REDACTED);
});
