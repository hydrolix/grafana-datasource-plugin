# Ad-hoc keys prefer aliases (identity aliases)

Hydrolix normalization profiles publish *identity aliases*: one stored
column carrying both a canonical and a customer-facing name, for example
`cliIP String` with `client_ip String ALIAS cliIP`. This opt-in policy shows
the alias instead of its physical base in new field choices, so a dashboard
author can present the canonical name alone.

## What counts as an identity alias

A column is hidden only when some other column is an `ALIAS` whose
expression is exactly that column's name and whose declared type is the
same. Both `DESCRIBE` spellings of the expression are recognized: the bare
form (`cliIP`, emitted by plain ClickHouse and by the metadata-only Hydrolix
`DESCRIBE`) and the backtick-wrapped form (`` `cliIP` ``).

Other `ALIAS` columns are handled the same with the option on or off:

- A computed alias, whose expression is anything other than one column name
  (`lower(cliIP)`, ``lower(`cliIP`)``, ``countMerge(`count()`)``), is not an
  ad-hoc filter key and gets no value suggestions. Before 0.14.0 the plugin
  listed one whose expression contained a backticked column, under that
  column's type; it no longer does. A saved filter on such a column still
  executes, because the backend's ad-hoc filter reads its own `DESCRIBE`.
- An alias of a missing column is not listed.
- A type-changing alias (`IPv4 ALIAS cliIP` over a `String`) and an
  alias-to-alias chain are listed when the column they name has a supported
  type, but they collapse nothing: that column stays listed as well.

If several identity aliases target one base, all of the aliases remain; the
policy does not rank them.

## Opt-in

The option defaults off. Turn on **Ad hoc filter keys prefer aliases** in the
datasource's additional settings (`jsonData.adHocKeysPreferAliases: true` when
provisioning) and keep the existing `adHocTableVariable` configuration. The
ad-hoc key list is then the plugin's usual key list for that table with the
identity-alias bases removed: Map columns still expand into `column['key']`
entries, array and nullable columns keep their handling, the default database
is applied to an unqualified table variable, and quoted view names such as
``akamai.`logs#redacted` `` keep working. The schema is read once per table,
exactly as without the option.

The option affects ad-hoc filter keys only. Monaco completion, the query
editor's schema browser and query-variable results are unchanged.

## Field-choice variables

The option does not change query-variable results, and there is no documented
field-choice recipe for Hydrolix. A dashboard variable that offers column names
for use in panel SQL (`SELECT $field, count() ... GROUP BY $field`) is usually a
Query variable over `system.columns`. Hydrolix lists `ALIAS` columns in
`DESCRIBE` only, not in `system.columns` (see `QueryAliases` in
`pkg/plugin/metadata.go`). A `system.columns` query on Hydrolix therefore cannot
offer the aliases, and cannot tell which base an alias stands for. A
field-choice variable that reads `DESCRIBE` is deferred until a dashboard needs
it.

## What never changes

The option changes new choices only. SQL is not rewritten and neither column
is renamed: a filter named `client_ip` reaches the query as `client_ip`, and
a panel that selects `cliIP` keeps `cliIP`. An existing
filter on a hidden base still retrieves values, with the same time window,
filters, nullable handling and query guardrails as any other key. Keep the
alias and its base available in the table to preserve existing dashboards.

The local CFB-4004 demonstration uses a hand-built alias. It proves plugin
behavior, not normalization-profile deployment; the end-to-end proof for an
OCSF profile applied by the real publisher through the Hydrolix Config API
remains pending.
