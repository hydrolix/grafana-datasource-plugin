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
        credentialsType: {
          testId: "data-testid data-testid hdx_credentialsType",
          label: "Credentials Type",
          description: "User or service account",
        },
        token: {
          testId: "data-testid hdx_token",
          label: "Token",
          description: "Service account token",
          placeholder: "default",
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
          label: "Ad hoc filter default time range",
          description:
            "Default time range for time filtering when the dashboard time range is not available",
        },
        adHocTableVariable: {
          testId: "data-testid hdx_adHocTableVariable",
          label: "Ad hoc filter table variable name",
          description:
            "Name of a dashboard variable that defines which table to use for retrieving ad hoc filter columns and values",
        },
        adHocConditionVariable: {
          testId: "data-testid hdx_adHocConditionVariable",
          label: "Ad hoc filter values query condition variable name",
          description:
            "Name of a dashboard variable that defines query condition to filter ad hoc filter values",
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
        aiEnabled: {
          testId: "data-testid hdx_aiEnabled",
          label: "Enable Assistant",
          description: "Toggle on to enable Assistant",
        },
        aiBaseUrl: {
          testId: "data-testid hdx_aiEnabled",
          label: "Assistant API base URL",
          description:
            "Base URL for Assistant API",
        },
        useDefaultAiBaseUrl: {
          testId: "data-testid hdx_useDefaultPortAiBaseUrl",
          label: "Use default",
          description: "Use default Assistant API base URL",
        },
        querySettings: {
          testId: "data-testid hdx_querySettings",
          label: "Query Settings",
          values: [
            {
              setting: "hdx_query_max_rows",
              type: "number",
              default: 0,
              min: 0,
              description:
                "Set the maximum number of rows that can be evaluated to answer a query. The query is canceled if it exceeds this number of rows.\n" +
                "\n" +
                "Default value is 0 which sets no limit.",
            },
            {
              setting: "hdx_query_max_attempts",
              type: "number",
              default: 3,
              min: 0,
              description:
                "Limit the number of retries for recoverable failures of a query peer. The query head cancels the query if an additional attempt exceeds this number.\n" +
                "\n" +
                "Default value is 3.",
            },
            {
              setting: "hdx_query_max_result_bytes",
              type: "number",
              default: 0,
              min: 10000,
              description:
                "Limit the number of bytes that can be stored on the query head before returning data. The query is canceled if the response byte count exceeds this value.\n" +
                "\n" +
                "The Config API enforces a minimum of 10000 for this query option on organization, project, and table.\n" +
                "\n" +
                "Users may set this circuit breaker lower using either the Query API or SQL SETTINGS for responses that are expected to be very small.\n" +
                "\n" +
                "Default value is 0 which sets no limit.",
            },
            {
              setting: "hdx_query_max_result_rows",
              type: "number",
              default: 0,
              min: 0,
              description:
                "Limit the number of rows that can be stored on the query head before returning the response. The query is canceled if the resulting row count exceeds this number.\n" +
                "\n" +
                "Default value is 0 which sets no limit.",
            },
            {
              setting: "hdx_query_max_timerange_sec",
              type: "number",
              default: 0,
              min: 0,
              description:
                "Limit the total timerange allowed for a query. Calculates a query's covered time range from the WHERE clause filter on the primary column and cancels if the time range in seconds exceeds this number.\n" +
                "\n" +
                "For example, setting this value to 86400 prevents operating on a time window larger than one day.\n" +
                "\n" +
                "To prevent queries lacking WHERE clauses (or any filters) on the primary column, you must use the hdx_query_timerange_required query option.\n" +
                "\n" +
                "Default value is 0 which sets no limit.",
            },
            {
              setting: "hdx_query_max_execution_time",
              type: "number",
              default: 0,
              min: 0,
              description:
                "Limit the total runtime of a query. The query is canceled after the specified number of seconds have passed since query execution began.\n" +
                "Default value is 0 which sets no limit.",
            },
            {
              setting: "hdx_query_timerange_required",
              type: "boolean",
              default: false,
              description:
                "Set this parameter to true to ensure that queries include a WHERE clause on the primary column. Any query lacking a WHERE clause or filter on the primary column is canceled.\n" +
                "\n" +
                "Requiring a time range for queries prevents accidental scanning of all data for all time.\n" +
                "\n" +
                "Default is false.",
            },
            {
              setting: "hdx_query_max_partitions",
              default: 0,
              type: "number",
              description:
                "Limit the number of partitions the query can read. The query is canceled if the total number of partitions required to execute the query exceeds this number.\n" +
                "\n" +
                "Default value is 0 which sets no limit.",
            },
            {
              setting: "hdx_query_max_peers",
              type: "number",
              default: 0,
              min: 0,
              description:
                "Instruct the query head to use only a subset of available peers for the query. If the number is greater than the number of available peers, all available peers are used.\n" +
                "\n" +
                "The effect of this setting may influence total query runtime by restricting work to a subset of query peers. For a systematic approach to separating query resources, see also Query pools.\n" +
                "\n" +
                "Default is 0. The query head distributes work across all available peers to maximize parallel processing.",
            },
            {
              setting: "hdx_query_pool_name",
              type: "text",
              description:
                "Send the query to a specific query pool. This option is only useful when multiple query pools are available, and the named query pool must exist already. See also Query pools.\n" +
                "\n" +
                "Sample error: ClusterError Pool name unknown_pool does not exist\n" +
                "\n" +
                "Default is empty string. The default query pool uses all available peers.",
            },
            {
              setting: "hdx_query_max_concurrent_partitions",
              type: "number",
              default: 3,
              min: 1,
              description:
                "Limit the number of partitions assignable to a single query peer.\n" +
                "\n" +
                "Default is 3",
            },
            {
              setting: "hdx_query_admin_comment",
              type: "textarea",
              description:
                "Add an admin comment to the query which is stored in Active Queries. This field can be filled automatically by Superset or Grafana to include username information in order to track user activity.\n" +
                "\n" +
                "Default is empty string.",
            },
            {
              setting: "hdx_http_proxy_enabled",
              type: "boolean",
              default: true,
              description:
                "Enable or disable HTTP proxying for queries when the data source is configured to use the HTTP proxy. When set to No, all queries are sent directly to the query heads.",
            },
            {
              setting: "hdx_http_proxy_ttl",
              type: "duration",
              description:
                "Set the TTL for queries executed through the HTTP proxy. Supports values in seconds (e.g., 60) or duration expressions (e.g., 10s, 5m, 1h).",
            },
          ],
        },
      },
    },
    query: {
      editor: {
        queryType: {
          label: "Query Type",
          tooltip: "Set query type",
        },
        round: {
          label: "Round",
          tooltip:
            "Round $from and $to timestamps to the nearest multiple of the specified value (1m rounds to the nearest whole minute). Supports time units: ms, s, m, h. No value means that the default round value will be used. A value of 0 means no rounding is applied",
        },
        showInterpolatedQuery: {
          label: "Show Interpolated Query",
        },
        hideInterpolatedQuery: {
          label: "Hide Interpolated Query",
        },
        formatQuery: {
          tooltip: "Format Query",
        },
        runQuery: {
          tooltip: "Click or hit CTRL/CMD+Return to run query",
        },
        showAiAssistant: {
          label: "Ask Hydrolix Assistant",
        },
      },
    },
  },
};
