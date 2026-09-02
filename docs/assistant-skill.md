# Querying Hydrolix

Use this skill whenever a question involves Hydrolix data — logs, events,
traces, or any table reachable through the Hydrolix MCP server. Hydrolix speaks
the ClickHouse SQL dialect, but several behaviours differ from stock ClickHouse
in ways that silently produce wrong or failing queries. Follow the rules below
rather than stock ClickHouse habits.

## Tools

The Hydrolix MCP server exposes four tools:

| Tool | Use it for |
| --- | --- |
| `list_databases` | Discovering which databases exist |
| `list_tables` | Listing tables in a database |
| `get_table_info` | Column names, types, summary-table metadata for one table |
| `run_select_query` | Executing a `SELECT`. Runs with `readonly = 1` |

Discover schema with `get_table_info` before writing a query against a table
you have not seen. If page context already supplied the table's columns and
primary time column, trust it and skip the round trip.

These tools come from a custom MCP server integration, conventionally named
**"Hydrolix"** or after the cluster it serves (for example `hydrolix-prod` or
the cluster hostname). When a question involves Hydrolix data, search the
available MCP capabilities for these tool names or a server named this way
before concluding you cannot query.

Page context from the Grafana query editor may include `datasourceHost` — the
cluster the user's dashboard actually queries. If it differs from the cluster
this MCP server is configured against, say so before answering: results from
the wrong cluster are worse than no results. If more than one Hydrolix MCP
server is registered, prefer the one whose name matches `datasourceHost`; if
none does, list the Hydrolix servers you found and ask the user which cluster
they mean rather than guessing. With a single Hydrolix server registered, use
it without asking.

## Rule 1 — every query needs a time filter

Hydrolix clusters commonly run with `hdx_query_timerange_required`. A query
whose `WHERE` clause has no time filter is rejected outright:

> Your query needs a time range filter in a WHERE clause

This applies to exploratory queries too. There is no such thing as a quick
`SELECT * FROM table LIMIT 5` here — it will fail. Always constrain the primary
time column, and pass an explicit time zone: `toDateTime('...')` otherwise
parses in the server's zone, not the user's.

```sql
SELECT timestamp, message
FROM cicd.logs
WHERE timestamp >= toDateTime('2026-08-14 00:00:00', 'UTC')
  AND timestamp <= toDateTime('2026-08-15 00:00:00', 'UTC')
ORDER BY timestamp DESC
LIMIT 100
```

## Rule 2 — write literal time bounds, not macros

The MCP server does not expand Grafana macros. Any `$__` token sent to
`run_select_query` reaches the cluster verbatim and fails. Convert the time
range you were given into literal bounds, as above. See "Reading saved panel
queries" for the reverse direction.

## Rule 3 — read the primary key from `system.tables`

In Hydrolix, `system.columns.is_in_primary_key` and
`system.columns.is_in_sorting_key` are **false for every column**. Do not use
them to identify the time column — you will conclude the table has no primary
key.

The primary key is populated in `system.tables.primary_key`, and that is where
`get_table_info` reads it from. It is normally a single timestamp column, and
it is the column your time filter must constrain for the query to be efficient.

## Rule 4 — do not trust reported nullability

Because Hydrolix reports `default_kind != 'DEFAULT'` for ordinary columns,
schema introspection marks essentially every column as nullable. This is an
artefact, not a fact about the data. Do not tell users their columns are
nullable on this basis, and do not add defensive `isNotNull()` wrapping for
that reason alone.

## Rule 5 — summary tables need `-Merge` functions

`get_table_info` flags summary tables (`is_summary_table`) — tables whose
aggregate columns hold partial aggregation states, not values. Selecting such
a column raw returns unusable binary state. Wrap each aggregate column in its
`-Merge` function — the per-column `merge_function` in the tool response says
which (`count(...)` → `countMerge(...)`, `sum(...)` → `sumMerge(...)`, and
parameterized forms like `quantileMerge(0.5)(...)`). Dimension columns are
selected and grouped normally. When a summary table covers the aggregation the
user asked for, prefer it over aggregating the raw table — it is much cheaper.

## Rule 6 — bound expensive queries

`SETTINGS max_execution_time = N` is supported and is a good habit on
exploratory queries:

```sql
SELECT count(*)
FROM cicd.logs
WHERE timestamp >= toDateTime('2026-08-14 00:00:00', 'UTC')
  AND timestamp <= toDateTime('2026-08-15 00:00:00', 'UTC')
SETTINGS max_execution_time = 30
```

Prefer `ORDER BY <primary timestamp> DESC` with a `LIMIT` for "latest N"
questions — it takes advantage of Hydrolix's primary key layout. Quote
identifiers containing non-word characters with double quotes:
`"my-project"."logs"`.

## Engine names

Hydrolix tables report their engine as `TurbineStorage`. This is expected and
is not a sign of a misconfigured or unsupported table.

## Reading saved panel queries

A query copied out of a Grafana panel may contain plugin macros, which the MCP
server cannot expand. Translate them yourself before executing, using the
conversation's time range for `from` and `to`.

| Macro | Expands to |
| --- | --- |
| `$__timeFilter(col)` | `col >= toDateTime(<from>) AND col <= toDateTime(<to>)` |
| `$__timeFilter_ms(col)` | same, with `fromUnixTimestamp64Milli(<from ms>)` bounds |
| `$__dateFilter(col)` | `col >= toDate('<from>') AND col <= toDate('<to>')` |
| `$__dateTimeFilter(dateCol, timeCol)`, `$__dt(...)` | the date filter and the time filter, AND-joined |
| `$__fromTime()`, `$__toTime()` | `toDateTime(<unix>)` for the range bounds |
| `$__fromTime_ms()`, `$__toTime_ms()` | `fromUnixTimestamp64Milli(<unix ms>)` |
| `$__timeInterval(col)` | `toStartOfInterval(toDateTime(col), INTERVAL <n> second)` |
| `$__timeInterval_ms(col)` | `toStartOfInterval(toDateTime64(col, 3), INTERVAL <n> millisecond)` |
| `$__interval_s()` | the panel's interval in seconds, as a bare number |
| `$__conditionalAll(cond, $var)` | `cond` when `$var` has a real selection; `1=1` only when `$var` is "All" or empty |
| `$__adHocFilter()` | the dashboard's ad hoc filters (see below); `1=1` when none apply |

`$__timeFilter` and `$__timeInterval` may appear with no argument, in which
case the plugin resolves the column from the table's primary key. Use the
primary time column from `get_table_info` when translating those.

### Translating ad hoc filters

When context supplies the dashboard's ad hoc filters, translate each one
exactly as the plugin does — do not invent your own SQL for an operator:

| Filter operator | Plugin's SQL |
| --- | --- |
| `=`, `!=`, `<`, `<=`, `>`, `>=` | `col <op> 'value'` |
| `=\|` (one of) | `col IN ('a', 'b')` |
| `!=\|` (not one of) | `col NOT IN ('a', 'b')` |
| `=~` with a plain value | `toString(col) LIKE 'value'`, with `*` translated to `%` |
| `=~` with a `regex:` prefix | `match(toString(col), 'pattern')` |
| `!~` | `NOT LIKE` / `not match(...)`, same prefix rule |
| any operator on an `Array` column | `has(col, 'value')` / `not has(col, 'value')` per value |
| `= NULL` / `!= NULL` | `col IS NULL` / `col IS NOT NULL` |

The most common mistake is emitting `match()` for `=~`: **`=~` means a LIKE
wildcard match unless the value explicitly starts with `regex:`.** Example —
the filter `hostname =~ web-*` becomes:

```sql
toString(hostname) LIKE 'web-%'
```

not `match(hostname, 'web-.*')`. Map columns are addressed as
`col['key']`, and every value is single-quoted.

## When a query fails — triage first

Hydrolix error messages are specific and usually say exactly what to fix.
Classify the error before acting: only the first bucket below is fixable by
editing SQL. Never "fix" an infrastructure error by mutating the query, and
never retry more than once.

### Fix the query

| Error contains | Fix |
| --- | --- |
| `needs a time range filter in a WHERE clause` | Apply Rule 1 |
| `Maximum time range exceeded ... (maximum is N)` | Narrow the range to ≤ N seconds, or split into ≤ N-second queries and merge the results |
| `Syntax error: failed at position ... (line L, col C)` | Correct the token at that position |
| `Could not apply macros` | Translate or fix the macro (see the reference above) |
| `Unknown function` | Fix the spelling or use a ClickHouse-supported alternative |
| `Missing columns` / `Not found column ... in block` | Re-check the schema with `get_table_info`; column names are case-sensitive |
| `Table ... does not exist` / `namespace=... does not exist` / `db=... does not exist` | `list_databases` + `list_tables`; qualify as `database.table` |
| `Cannot convert` / `Illegal type` / `no supertype` / `Nested type` | Add explicit `CAST`; align types across `UNION` and `CASE` branches |
| `Number of arguments ... doesn't match` | Fix the function call signature |
| `Aggregate function ... found in WHERE` | Move the condition to `HAVING` |
| `not under aggregate function and not in GROUP BY` | Add the column to `GROUP BY` or wrap it in an aggregate |
| `Different expressions with the same alias` | Rename the duplicate alias |
| `is not a constant expression` (IN/VALUES/LIMIT) | Never emit an empty `IN tuple()`; inline constant values |
| `cannot compile re2` / `invalid regex` | Fix the regex pattern |
| `Cannot parse quoted string` | Single-quote array elements; escape embedded quotes |
| `The text does not contain` | An unsubstituted `${var}` is still in the query; replace it with a literal |
| `Limit for result exceeded` | Add `LIMIT`, tighter filters, or aggregate instead of returning raw rows |
| `Timeout exceeded: elapsed` | Narrow the time range; add `SETTINGS max_execution_time`; add `LIMIT` |
| `Not enough privileges` | Not fixable by SQL alone — tell the user which grant on which table is missing |
| `std::bad_alloc` | Reduce data processed (filters, `LIMIT`); if it persists, treat as infrastructure |

### Retry once, then escalate

`Broken pipe`, socket timeouts, `Attempt to read after eof`, `Cannot read all
data`, `Query was cancelled`, `Unknown packet`, partition `io timed out`,
storage `SlowDown` / `ServerBusy`, catalog `transaction is aborted` /
`Lost connection`. These are transient network, storage, or catalog
conditions: retry the identical query once. If it recurs, report it as an
infrastructure issue — do not keep retrying or rewriting.

### Stop — not fixable by editing SQL

`SSL_read` / `SSL_connect` failures, `getaddrinfo failed` (DNS), storage
`AuthenticationFailed`, `disk space used% > redzone`, `filesystem error`,
`failed to create tmp file`, `dictionaries not loaded`, `config not loaded`,
`Pool name ... does not exist`. Explain what the message means and direct the
user to their Hydrolix administrator or infrastructure team. Changing the
query cannot resolve these.
