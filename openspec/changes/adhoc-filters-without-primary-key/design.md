## Context

Both ad-hoc preload statements hardcode a time predicate.
`AD_HOC_VALUE_QUERY` carries `$__timeFilter(${timeColumn})`, where
`timeColumn` is the table's primary key, and `AD_HOC_MAP_KEY_QUERY` carries
a zero-argument `$__timeFilter()` that the backend resolves against the same
primary key.

On a table with no primary key the two paths diverge:

- `getTagValues` resolves the key via `metadataProvider.primaryKey()`, gets
  an empty string, fails the `if (table && timeFilter)` guard, and returns
  `[]`. No warning, no error — the dropdown is simply empty.
- `getTagKeysForMap` never consults the primary key; it sends the statement
  and lets the backend resolve the column. `QueryPK` only returns
  `ErrPrimaryKeyNotFound` when `system.tables` yields no row, i.e. the table
  does not exist. A real table with no primary key yields a row whose
  `primary_key` is the empty string. `GetPK` stores and returns that string
  like any other lookup result, and `resolveColumnArg` interpolates it as a
  column name, so the cluster rejects the SQL with a parse error that names
  neither the table nor the cause.

The guardrails that make an unfiltered preload acceptable are already in
place and independent of the time predicate: the metadata query runner pins
`hdx_query_max_execution_time = 10` (min-wins against datasource-level
settings), the statement templates carry `timeout_overflow_mode = 'break'`
so the cap yields partial results rather than an error, and the value query
is bounded by `topK(100)`. The third setting in the suffix,
`hdx_query_max_timerange_sec`, derives the query's covered range from the
WHERE-clause filter on the primary column (Hydrolix query circuit-breaker
docs); a statement with no such filter is simply not measured by it.

## Goals / Non-Goals

**Goals:**

- Ad-hoc key and value dropdowns populate for tables with no primary key.
- Tables with a primary key keep their current SQL, settings, and time
  window byte-for-byte.
- The backend stops emitting invalid SQL when a zero-argument time macro
  cannot resolve a column, and says why.
- A primary-key lookup that *fails* (cluster error, missing grant, breaker
  trip) is never mistaken for a table that *has* no primary key.

**Non-Goals:**

- Inferring a substitute time column from the table's schema.
- Any new memoization. The existing primary-key memoization in
  `metadataProvider` and the existing `pkCache` in `MetadataProvider` are
  reused exactly as they are; this change adds no new entry class to either.
- Changing time semantics for dashboard, Explore, or alerting queries.
- Making partial (broken-off) preload results deterministic.

## Decisions

### D1: The empty string is the value that means "no primary key", on both sides

A table with no primary key is represented as the primary key `""` — a
successful lookup result, not an error — in the frontend
`metadataProvider.primaryKey()` memo and in the backend `MetadataProvider`
`pkCache` alike. Both already behave this way at HEAD; the change makes the
consumers honor it instead of treating `""` as "bail" (frontend) or as a
column name (backend).

Rationale: the fact is a stable property of the schema, exactly like a
non-empty key, so it belongs in the same place with the same lifetime. It
also means a transient lookup failure stays an error — uncached, retried on
the next call — and can never be confused with a keyless table.

*Alternative considered:* have `QueryPK` return a typed error for the empty
cell and store it as a negative entry. Rejected: it introduces a new
memoized-error class (against the project's no-new-caching rule), it makes
`GetPK` decide via `errors.Is` on an SDK `ErrorWithSource` — whose `Is()`
compares only the *source*, so any downstream error would have matched and
poisoned the entry for the TTL — and it puts the "is this usable?" decision
in the lookup rather than in the one consumer that needs a column.

### D2: Branch on `""` in the frontend; an unresolved key keeps the old behavior

`getTagKeys` resolves the primary key once, alongside `tableKeys`, and
passes it into every `getTagKeysForMap` call. `getTagValues` resolves it as
today. Both statement builders take a required `timeColumn: string` and
select the keyless form when it is `""`.

`metadataProvider.primaryKey()` returns `Promise<string | undefined>`: `""`
for a table that declares no key, `undefined` when `system.tables` has no
row for it (memoized, as at HEAD — the Assistant asks on a 300ms debounce
while a table name is half-typed, so an un-memoized miss would cost one
lookup per keystroke), and a rejection — not memoized — when the response
carries `errors`. An unresolved key (`undefined` or rejection) never selects
the keyless form; each path keeps its pre-change behavior instead:
`getTagValues` returns `[]` without a preload, and `getTagKeysForMap` issues
the time-filtered statement with the capped range and lets the backend
resolve `$__timeFilter()` from its own lookup, as it always did.

Rationale: pre-change, a failed lookup produced an empty value dropdown and
no scan, while map-key discovery did not consult the frontend key at all.
Treating `undefined` as keyless would have turned a transient failure into a
full-table scan on a keyed table, bounded only by the breaker, on every
dropdown open; suppressing map-key discovery on the same failure would have
made it degrade *worse* than before. Resolving once in `getTagKeys` also
avoids N identical `system.tables` queries racing the memo when a table has
N Map columns.

*Alternative considered:* expand a zero-argument `$__timeFilter()` to a
tautology when the primary key is empty. Rejected — the macro is shared with
dashboard queries, where silently dropping a time filter turns a bounded
panel query into a full-table scan.

### D3: One template per statement with an optional `${timeFilter}` slot

`AD_HOC_VALUE_QUERY` and `AD_HOC_MAP_KEY_QUERY` gain a `${timeFilter}` slot
in place of the hardcoded conjunct, filled with `$__timeFilter(<col>) AND `
(value) or `$__timeFilter() AND ` (map keys) when the primary key is
non-empty and with the empty string otherwise — the same mechanism the
existing `${condition}` slot uses. Both forms keep the full
`AD_HOC_QUERY_GUARDRAIL_SETTINGS` suffix.

Rationale: `hdx_query_max_timerange_sec` is inert on a statement with no
primary-column predicate, so there is no reason to vary the suffix, and one
template keeps the `topK` arity, `AS value`, `$__adHocFilter()` and
`${condition}` handling in a single place instead of four hand-synced
copies. The keyed output is byte-identical to today.

*Alternative considered:* separate `*_NO_TIME_FILTER` constants that drop
`hdx_query_max_timerange_sec`. Rejected once the premise — that the setting
would reject a predicate-free query — was checked against the Hydrolix
documentation and found false; the dev stack (stock ClickHouse with
`custom_settings_prefixes hdx_`) cannot enforce any `hdx_*` setting, so it
could neither confirm nor refute it.

### D4: The template slots are filled with a function replacer

`replaceAll` with a *string* replacement applies JavaScript's
`GetSubstitution`, so `$'`, `` $` ``, `$&` and `$$` inside the user-authored
ad-hoc condition — or a map-key column like `attrs['$ref']` — splice the
`SETTINGS` clause into the predicate. The builders fill every slot with
`() => value` so the text is inserted literally.

Rationale: the defect predates this change, but the change touches every
one of those call sites and adds a new slot; copying the bug into it would
be worse than fixing it.

### D5: Keyless preloads pass no time range and skip the cap

`getTagValues` and `getTagKeysForMap` call `adHocPreloadRange()` only when
the primary key is non-empty. The keyless form passes no range, so
`executeQuery` substitutes `ZERO_TIME_RANGE` — which the codebase already
defines as "this metadata query has no time macro", the same contract the
`system.*` and `DESCRIBE` lookups rely on.

Rationale: the trailing-24h cap and 5-minute rounding exist to bound and
stabilize the scanned window. With no predicate there is nothing to bound,
and passing a real range would imply a filter the SQL does not contain.

### D6: The backend raises the error where the column is needed, not where it is looked up

`QueryPK` and `GetPK` are unchanged from HEAD. `getPK` — the helper on the
macro path that maps the macro position to `(database, table)` — returns
`backend.DownstreamError(fmt.Errorf("%w: %s.%s; …", ErrPrimaryKeyEmpty,
db, table))` when `GetPK` yields `""`, with a hint that the time column
must be passed as the macro's argument (wording that is correct for all
four PK-lookup macros). `resolveColumnArg` stays the one-line delegation it
is at HEAD.

`ErrPrimaryKeyEmpty` is a plain `errors.New` sentinel with no source
attached. The source is added on the wrapper at the return site, so
`errors.Is` is an identity check and `backend.IsDownstreamError` still finds
the source on the chain.

Rationale: a keyless table is a fact about the user's schema, not a plugin
defect, so downstream is the honest classification for Grafana's error
attribution. Putting the sentinel's source on the wrapper rather than the
sentinel is what keeps `errors.Is` from matching every downstream error.

### D7: Keyless values stay ordered by `topK`, with no time ordering

The keyless value query is the existing `topK(100)` aggregation with the
time conjunct removed. Values reflect whatever the scan reached before the
breaker fired, not the most recent rows.

*Alternative considered:* add `ORDER BY <pk> DESC LIMIT n` to approximate
recency. Rejected — there is no primary key to order by, and an unindexed
sort is exactly the unbounded work the `topK` shape was introduced to avoid.

## Risks / Trade-offs

- **An unfiltered scan on a large keyless table returns partial or empty
  results** → the 10s breaker with `break` overflow keeps it bounded and
  non-fatal; empty results already degrade to manual value entry. Proven by
  a frontend unit test asserting the full guardrail suffix and breaker
  travel with the keyless form, and by an e2e asserting the dropdown
  populates against a keyless fixture.

- **`hdx_query_max_timerange_sec` on a predicate-free query is documented as
  inert, not measured on a real Hydrolix cluster** → the setting is kept on
  both forms, so a wrong reading of the docs would surface as a rejected
  preload on a keyless table, not as a silent unbounded scan. Proving signal:
  the first run of the keyless e2e against a real Hydrolix query head; the
  dev stack's ClickHouse cannot exercise it.

- **A failed primary-key lookup on a keyed table is mistaken for "keyless"
  and triggers an unfiltered scan** → D1/D2: only `""` selects the keyless
  form, error responses are not memoized, and an unresolved key keeps the
  pre-change `[]`. Proven by a frontend unit test feeding an error response
  and asserting no preload query is issued and nothing is memoized, and by a
  Go test feeding a generic downstream error and asserting it is neither
  `ErrPrimaryKeyEmpty` nor stored in `pkCache`.

- **Partial results make the dropdown non-deterministic between opens** →
  accepted; the dropdown is a suggestion list and manual entry is always
  available. The filtered form keeps its 5-minute rounding, so this
  variability is confined to keyless tables.

- **The new empty-primary-key error reaches user-authored dashboard SQL that
  previously "worked"** → it never worked; it produced a cluster syntax
  error. The typed error is strictly more informative. Proven by a Go test
  asserting the sentinel, message, and downstream source for a keyless
  table, and one asserting `ErrPrimaryKeyNotFound` still fires for a missing
  table.
