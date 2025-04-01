export default {
  components: {
    config: {
      editor: {
        host: {
          label: "Server address",
          description: "Hydrolix server address",
          placeholder: "Server address",
          error: "Server address required",
        },
        port: {
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
          label: "Use default",
          description: "Use default port",
        },
        protocol: {
          label: "Protocol",
          description: "Native or HTTP for server protocol",
        },
        secure: {
          label: "Secure Connection",
          description: "Toggle on if the connection is secure",
        },
        path: {
          label: "HTTP URL Path",
          description: "Additional URL path for HTTP requests",
          placeholder: "additional-path",
        },
        skipTlsVerify: {
          label: "Skip TLS Verify",
          description: "Skip TLS Verify",
        },
        username: {
          label: "Username",
          description: "Hydrolix username",
          placeholder: "default",
        },
        password: {
          label: "Password",
          description: "Hydrolix password",
          placeholder: "Password",
        },
        dialTimeout: {
          label: "Dial Timeout (seconds)",
          description: "Timeout in seconds for connection",
          placeholder: "10",
        },
        queryTimeout: {
          label: "Query Timeout (seconds)",
          description: "Timeout in seconds for read queries",
          placeholder: "60",
        },
        defaultDatabase: {
          label: "Default database",
          description: "Used when no specific database is provided in queries",
          placeholder: "sample",
        },
        adHocDefaultTimeRange: {
          label: "Ad-hoc filter default time range",
          description:
            "Used to filter possible ad-hoc filter values when the dashboard time range is unavailable",
        },
        adHocKeysQuery: {
          label: "Ad-hoc filter keys query",
          description: "Used to retrieve possible keys for ad-hoc filters",
          placeholder: "",
        },
        adHocValuesQuery: {
          label: "Ad-hoc filter values query",
          description:
            "Used to retrieve possible values for ad-hoc filter keys",
          placeholder: "",
        },
        adHocTableVariable: {
          label: "Ad-hoc filter table variable name",
          description:
            "Dashboard variable name to specify the table for retrieving ad-hoc filter keys",
        },
        adHocTimeColumnVariable: {
          label: "Ad-hoc filter time column variable name",
          description:
            "Dashboard variable name to specify the time column used to filter possible ad-hoc filter values within the dashboard time range",
        },
        defaultRound: {
          label: "Default round",
          description:
            "Automatically rounds $from and $to timestamps to the nearest multiple of a default value (e.g., 1m rounds to the nearest whole minute). Used when no specific round value is provided in the query. Supported time units: ms, s, m, h. No value or a value of 0 means no rounding is applied",
        },
      },
    },
  },
};
