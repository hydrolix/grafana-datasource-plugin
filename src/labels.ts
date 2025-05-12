export default {
  components: {
    config: {
      editor: {
        host: {
          testId: "data-testid hdx_serverAddress",
          label: "Server address",
          description: "Hydrolix server address",
          placeholder: "Server address",
          error: "Server address required",
        },
        port: {
          testId: "data-testid hdx_serverPort",
          label: "Server port",
          description: "Hydrolix server port",
          placeholder: "Server port",
          error: "Server port required",
          insecureNativePort: "9000",
          insecureHttpPort: "8123",
          secureNativePort: "9440",
          secureHttpPort: "443",
        },
        useDefaultPort: {
          testId: "data-testid hdx_useDefaultPort",
          label: "Use default",
          description: "Use default port",
        },
        protocol: {
          testId: "data-testid data-testid hdx_protocol",
          label: "Protocol",
          description: "Native or HTTP for server protocol",
        },
        secure: {
          testId: "data-testid hdx_secureConnection",
          label: "Secure Connection",
          description: "Toggle on if the connection is secure",
        },
        path: {
          testId: "data-testid hdx_requestPath",
          label: "HTTP URL Path",
          description: "Additional URL path for HTTP requests",
          placeholder: "additional-path",
        },
        skipTlsVerify: {
          testId: "data-testid hdx_skipTlsVerify",
          label: "Skip TLS Verify",
          description: "Skip TLS Verify",
        },
        username: {
          testId: "data-testid hdx_requestUsername",
          label: "Username",
          description: "Hydrolix username",
          placeholder: "default",
        },
        password: {
          testId: "data-testid hdx_requestPassword",
          label: "Password",
          description: "Hydrolix password",
          placeholder: "Password",
        },
        dialTimeout: {
          testId: "data-testid hdx_dialTimeout",
          label: "Dial Timeout (seconds)",
          description: "Timeout in seconds for connection",
          placeholder: "10",
        },
        queryTimeout: {
          testId: "data-testid hdx_queryTimeout",
          label: "Query Timeout (seconds)",
          description: "Timeout in seconds for read queries",
          placeholder: "60",
        },
        defaultDatabase: {
          testId: "data-testid hdx_defaultDatabase",
          label: "Default database",
          description: "Used when no specific database is provided in queries",
          placeholder: "sample",
        },
        adHocDefaultTimeRange: {
          testId: "data-testid hdx_adHocFilterTimeRange",
          label: "Ad-hoc filter default time range",
          description:
            "Used to filter possible ad-hoc filter values when the dashboard time range is unavailable",
        },
        adHocTableVariable: {
          testId: "data-testid hdx_adHocTableVariable",
          label: "Ad-hoc filter table variable name",
          description:
            "Dashboard variable name to specify the table for retrieving ad-hoc filter keys and values",
        },
        adHocTimeColumnVariable: {
          testId: "data-testid hdx_adHocTimeColumnVariable",
          label: "Ad-hoc filter time column variable name",
          description:
            "Dashboard variable name to specify the time column used to filter possible ad-hoc filter values within the dashboard time range",
        },
        defaultRound: {
          testId: "data-testid hdx_defaultRound",
          label: "Default round",
          description:
            "Automatically rounds $from and $to timestamps to the nearest multiple of a default value (e.g., 1m rounds to the nearest whole minute). Used when no specific round value is provided in the query. Supported time units: ms, s, m, h. No value or a value of 0 means no rounding is applied",
        },
        additionalSettings: {
          testId: "data-testid hdx_additionalSection",
          label: "Additional Settings",
          description: "",
        },
      },
    },
  },
};
