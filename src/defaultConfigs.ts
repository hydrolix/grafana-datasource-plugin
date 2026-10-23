import { CredentialsType, Protocol } from "./types";

export default {
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
