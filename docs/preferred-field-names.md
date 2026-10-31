# Experimental identity-alias choices

This opt-in policy shows a strict identity alias instead of its physical base in new field choices. For example, given stored `cliIP String` and `client_ip String ALIAS cliIP`, the preferred choice is `client_ip`. Both bare `cliIP` and backtick-quoted expressions are recognized. The base must exist, be physical, and have exactly the same type. Computed aliases, type changes, missing targets, and alias chains are not collapsed. If multiple identity aliases target a base, all aliases remain visible; this option does not rank them.

There are two independent opt-ins, configured through datasource/dashboard JSON (no configuration UI toggle):

- **Ad-hoc Filter keys:** set `jsonData.preferredFieldNames` to `true` on the datasource. Keep the existing `adHocTableVariable` configuration. Keys come from `DESCRIBE` and retain the plugin's supported scalar, nullable, and array types. This mode does not enumerate map keys. Existing map filters and base-column filters can still load values.
- **A dashboard field-choice query variable:** set its query object to `{"preferredColumns":"project.table"}`. A table variable can be used, for example `{"preferredColumns":"project.${table}"}`. This explicit metadata query returns preferred column names for use in a SQL field dropdown. It takes precedence over `rawSql` if both are present. It does not change Monaco completion, the query editor's schema browser, or arbitrary SQL variable results.

The resolved table name must be a simple qualified `database.table` identifier (letters, digits, underscores; each part starts with a letter or underscore). Quoted or otherwise complex identifiers are not supported by these experimental paths. Each request reads current `DESCRIBE` metadata rather than the existing cached column list.

Both options default off. They change new choices only: SQL is not rewritten, neither column is renamed, and an existing filter on a hidden base still retrieves values using that base. Value queries retain the existing time window, filters, nullable handling, and query guardrails. Keep the underlying alias and base available to preserve existing dashboards.

The local CFB-4004 demonstration uses a hand-built alias. It proves plugin behavior, not normalization-profile deployment. The separate end-to-end proof for a normal OCSF profile applied by the real publisher through the Hydrolix Config API remains pending.
