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
`DESCRIBE`) and the backtick-wrapped form (`` `cliIP` ``). Computed aliases
(`lower(cliIP)`), type-changing aliases (`IPv4 ALIAS cliIP` over a `String`),
aliases of a missing column and alias-to-alias chains collapse nothing: every
such column, and its target, stays visible. If several identity aliases
target one base, all of the aliases remain; the policy does not rank them.

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

The option does not change query-variable results. A dashboard variable that
offers column names for use in panel SQL (`SELECT $field, count() ... GROUP BY
$field`) is a plain Query variable over `system.columns`. To apply the same
identity-alias rule there, hide every column that is the same-type target of an
`ALIAS` column; `trim` removes the backticks Hydrolix puts around the alias
expression, and the `type` join keeps type-changing aliases from hiding their
base:

```sql
SELECT c.name
FROM system.columns AS c
LEFT JOIN (
    SELECT trim(BOTH '`' FROM default_expression) AS base, type
    FROM system.columns
    WHERE database = 'akamai' AND table = 'logs' AND default_kind = 'ALIAS'
) AS a ON a.base = c.name AND a.type = c.type
WHERE c.database = 'akamai' AND c.table = 'logs' AND a.base = ''
ORDER BY c.position
```

For `cliIP String` plus `client_ip String ALIAS cliIP` this lists `client_ip`
and not `cliIP`, matching the ad-hoc key list. Substitute a table variable for
the literal names if the dashboard switches tables.

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
