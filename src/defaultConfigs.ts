import { CredentialsType, ExposeErrorsOptions, Protocol } from "./types";

const defaultConfigs = {
  username: "default",
  protocol: Protocol.Native,
  port: 9440,
  useDefaultPort: true,
  secure: true,
  skipTlsVerify: false,
  path: "/query",
  settings: {},
  exposeErrors: {
    enabled: true,
    variableName: "hdx_query_errors",
    maxCount: 5,
    ttl: 300,
  },
  credentialsType: CredentialsType.UserAccount,
  oauthPassThru: false,
};

export default defaultConfigs;

/**
 * The error-exposure configuration the plugin acts on: each key missing from
 * the stored block takes its default. The runtime and the config editor both
 * use it, so the editor shows what actually runs. A stored `null` (a blank
 * provisioning key, an API payload) counts as missing.
 */
export function resolveExposeErrors(
  exposeErrors?: ExposeErrorsOptions | null
): Required<ExposeErrorsOptions> {
  const stored = Object.fromEntries(
    Object.entries(exposeErrors ?? {}).filter(
      ([, v]) => v !== undefined && v !== null
    )
  );
  return { ...defaultConfigs.exposeErrors, ...stored };
}
