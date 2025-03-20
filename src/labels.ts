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
        defaultTable: {
          label: "Default table",
          description: "Used together with the default database as the default source for ad-hoc filter keys and values (if both are unset, keys and values are retrieved for all tables in the dashboard with filtering applied separately)",
          placeholder: "logs",
        },
        adHocKeyQuery: {
          label: "Ad-hoc filter key query",
          description: "Used to retrieve possible keys for ad-hoc filters",
          placeholder: "SELECT key FROM ${table}",
        },
        adHocValuesQuery: {
          label: "Ad-hoc filter value query",
          description: "Used to retrieve possible values for ad-hoc filter keys",
          placeholder: "SELECT ${column}, COUNT(${column}) as count  FROM ${table} WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter() GROUP BY ${column} ORDER BY count DESC LIMIT 100",
        },
        adHocTableVariable: {
          label: "Ad-hoc filter table variable name",
          description: "Dashboard variable name to specify the table for retrieving ad-hoc filter keys (overrides the default table)",
        },
      },
    },
  },
};
