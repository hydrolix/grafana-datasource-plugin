# Design: adhoc-filter-values-topk-guardrails

## Context

Ad-hoc filter value dropdowns are populated by `DataSource.getTagValues`
(`src/datasource.ts`), which renders `AD_HOC_VALUE_QUERY`
(`src/constants.ts`) via `getColumnValuesStatement` (`src/ast.ts`) and runs
it through `metadataProvider.executeQuery` → `getQueryRunner`
(`src/editor/metadataProvider.ts`). The current shape is:

```sql
SELECT ${column}, COUNT(${column}) as count FROM ${table}
WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter() ${condition}
GROUP BY ${column} ORDER BY count DESC LIMIT 100
```

For a high-cardinality column over 30–90 days of raw data the `GROUP BY`
materializes one aggregation-state entry per distinct value and sorts them
before `LIMIT 100` applies — unbounded memory and a full-range scan with no
time or row cap. Grafana's `DataSourceGetTagValuesOptions` (v13.1 types)
carries no typed-text field, so there is no server-side narrowing while the
user types: this one query must return useful values quickly or the dropdown
is dead.

Constraints:
- Macro expansion (`$__timeFilter`, `$__adHocFilter`) is server-side; the
  frontend only assembles the SQL template.
- `querySettings` on a query target reach the backend and are applied as
  session-level settings (`driver.go` `MutateQueryData`); DS-level settings
  are merged first, target-level settings win (`prepareTarget`,
  `src/datasource.ts`).
- `getTagValues` derives synthetic `__empty__` / `__null__` dropdown entries
  from the raw value list; this UX must survive.
- Production is Hydrolix (ClickHouse-compatible engine); the dev stack is
  stock ClickHouse. The query must work on both.

## Goals / Non-Goals

**Goals:**
- Value preload completes (possibly with partial results) in bounded time
  regardless of column cardinality and range length.
- Returned values remain the most *useful* ones (approximate popularity
  order), since client-side typing can only filter what was fetched.
- Preload scans are bounded to the trailing 24h of the dashboard range, with
  5-minute-stable SQL text so repeated dropdown opens issue an identical,
  deduplicatable statement.
- All frontend metadata queries (value preload, map-key discovery, schema /
  autocomplete lookups) gain the same guardrails.
- No backend changes, no new config UI, no breaking changes.

**Non-Goals:**
- Any caching in the plugin — not allowed by policy. Dropdown results are
  never stored; every open issues a (bounded, repeatable) query.
- Server-side typed-text narrowing (Grafana API does not support it for
  ad-hoc filters).
- Guardrails for dashboard/panel queries — user-authored queries stay
  untouched.

## Decisions

### D1: Replace `GROUP BY … ORDER BY count DESC LIMIT 100` with `topK(100)`

New `AD_HOC_VALUE_QUERY` shape (flat single-column form; `${column}` may be
a plain column, `arrayJoin(col)` for Array types, or a map access like
`tags['env']` — same substitution as today; the `SETTINGS` suffix is the
SQL-channel guardrail from D2):

```sql
SELECT arrayJoin(topK(100)(${column})) AS value
FROM ${table}
WHERE $__timeFilter(${timeColumn}) AND $__adHocFilter() ${condition}
SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000
```

Rationale: `topK` (Filtered Space-Saving) keeps a fixed number of counters,
so aggregation memory is constant in column cardinality, there is no
full-width sort, and approximate popularity ordering is preserved — the
values users most likely want stay at the top. Single scan, one query.

The spike confirmed this flat form with the `timeout_overflow_mode` setting
end-to-end on a real Hydrolix cluster (2026-08-24);
`hdx_query_max_timerange_sec` is a documented Hydrolix query option whose
acceptance in the same clause task 1 confirms. Task 1 also verifies
dev-ClickHouse parity and that server-side
`$__timeFilter`/`$__adHocFilter` expansion resolves the target table; the
subquery variant (`SELECT arrayJoin(vals) FROM (SELECT topK(100)(…) AS
vals …)`) remains the fallback if stock ClickHouse rejects the flat form.

Alternatives considered:
- `SELECT DISTINCT ${column} … LIMIT 100` — the only shape with true early
  termination (stops reading at 100 distinct values), but returns an
  arbitrary 100. For high-cardinality columns an arbitrary sample is nearly
  useless in a dropdown that can only be client-filtered. Rejected on result
  quality; scan cost is instead bounded by D2.
- Keep `GROUP BY` and cap it with `max_rows_to_group_by` +
  `group_by_overflow_mode='any'` — bounds memory but silently biases toward
  values seen early, still sorts, and needs the same guardrails anyway.
  `topK` does the same job with fewer knobs.

### D2: Guardrails split across the two settings channels

The plugin has two transports for query settings, and they accept different
vocabularies:

- **Driver channel** — target `querySettings` → `MutateQueryData` → session
  settings, shipped by the driver as URL query parameters on the Hydrolix
  HTTP interface. Hydrolix validates these against its known option set, so
  only settings with a Hydrolix mirror survive.
- **SQL channel** — a `SETTINGS` clause inside the statement text, passed to
  the engine with the query.

`hdx_query_max_execution_time` and ClickHouse's `max_execution_time` are the
**same setting** on Hydrolix (aliases), so there is no soft-cap/backstop
layering to be had — there is exactly one execution-time knob.
`timeout_overflow_mode` has **no** Hydrolix mirror, so it is rejected on the
driver channel and can only travel in the SQL text. The guardrails are
therefore split:

| guardrail                      | channel                                                | default | effect |
|--------------------------------|--------------------------------------------------------|---------|--------|
| `hdx_query_max_execution_time` | driver (`querySettings` injected in `getQueryRunner`) | `10` | Hydrolix-native circuit breaker: cancels the query after 10s, guaranteed enforcement; effective value = min(10, DS-level value under either alias name) (D3) |
| `timeout_overflow_mode = 'break'` | SQL (`SETTINGS` suffix on the value-preload and map-key templates) | `break` | where honored, hitting the execution-time cap returns the `topK` aggregate over rows read so far — fast partial values — instead of a cancellation error |
| `hdx_query_max_timerange_sec` | SQL (same `SETTINGS` suffix) | `87000` | Hydrolix-native backstop for the D6 lookback cap: cancels any preload whose WHERE-derived range exceeds 24h + rounding slack — an invariant assertion that never fires unless the cap regresses |

Degradation ladder: `break` honored → partial top values at the cap; `break`
ignored or unsupported → the breaker cancels at 10s and the UI shows the
beautified Hydrolix timeout error (`errorBeautifier` already has that
template), leaving manual value entry. Either way the worst case is bounded
by the Hydrolix-native breaker.

**Spike result (2026-08-24, real cluster):** the split is confirmed working.
`hdx_query_max_execution_time` as a driver/URL parameter combined with
SQL-level `SETTINGS timeout_overflow_mode = 'break'` returns partial `topK`
values when the cap leaves enough scan budget (3s over a 90-day range
returned values) and returns an **empty but successful** result when the cap
is too tight (1s often yielded no rows). The flat
`SELECT arrayJoin(topK(100)(col))` form is also confirmed valid on Hydrolix.
Consequences: the 10s default has comfortable headroom over the measured 3s,
and the frontend treats an empty preload result as "no suggestions", never
as an error.

Other `hdx_query_*` breakers (`hdx_query_max_rows`,
`hdx_query_max_partitions`, `hdx_query_max_result_rows/bytes`) are
deliberately **not** defaulted: they cancel rather than degrade, there is no
row/partition count that is simultaneously safe for wide tables and harmless
for narrow ones, and the execution-time breaker already bounds the
user-visible symptom. Operators can add any of them per-datasource (D3).

Placement rationale: injecting at the metadata query runner scopes the
guardrails to exactly the queries the plugin issues on its own behalf
(value preload, `AD_HOC_MAP_KEY_QUERY`, `DESCRIBE`, schema/table/column/PK
lookups — all trivially cheap except the first two, and harmless for the
rest). Dashboard and Explore queries are untouched.

Alternative considered: shipping both guardrails on a single channel.
All-driver is impossible — `timeout_overflow_mode` has no Hydrolix mirror
and is rejected as a URL query parameter. All-SQL (hardcoding the breaker
into the templates too) would bypass the `querySettings` merge that lets
DS-level settings tighten the breaker (D3), and `sql_settings.go`
already demonstrates why SQL-level and session-level settings should not
compete for the same knob. The split keeps each setting on the only channel
where it both works and stays overridable where override matters.

### D3: Breaker value is min(default, DS-level) — tighten-only, no new config UI

The breaker is always injected into metadata targets (target-level settings
win the `prepareTarget` merge, so injection is authoritative). Its value is
`min(10, DS-level value)`, where the DS-level value is read from
`instanceSettings.jsonData.querySettings` under either alias name
(`hdx_query_max_execution_time` / `max_execution_time` — same setting on
Hydrolix). A DS-level value that is missing, non-numeric, or `≤ 0` is
ignored (`0` means *unlimited* on Hydrolix — the one value the metadata
path must never adopt), leaving the default 10.

Rationale: DS-level execution-time settings are sized for dashboards and
can be huge; value-preload queries must never inherit that budget.
Operators can tighten the metadata timeout via the existing "Query
settings" editor but cannot loosen it past 10s — raising the ceiling
requires a code change (deliberate: no escape hatch until a real need
appears). The SQL-level `break` suffix is unaffected by DS settings, so a
tightened breaker keeps the partial-results behavior.

Alternative considered: suppress injection when a DS-level setting with the
same name exists, letting the DS value win — rejected: a dashboard-sized
DS-level `max_execution_time` (e.g. 300) would silently lift the metadata
breaker to that value, exactly the resource spend the guardrail exists to
prevent.

### D4: Preserve synthetic `__empty__` / `__null__` entries

- Empty string: `topK` treats `''` as an ordinary value, so the existing
  `getTagValues` logic (map `''` → `SYNTHETIC_EMPTY`) keeps working on the
  single `value` column unchanged.
- NULL: aggregate functions skip NULLs, so NULL presence cannot be inferred
  from the value list — and it doesn't need to be. `getTagValues` already
  looks up the column's type from the already-fetched `DESCRIBE` metadata
  (`tableKeys`) to decide `arrayJoin` wrapping; the same metadata gates the
  synthetic entry: **for plain columns, append `SYNTHETIC_NULL` iff the type
  is in `NULLABLE_TYPES`; for map-access keys (`attrs['env']`), resolve the
  base map column's type (strip the `['…']` suffix) and append iff it is a
  `Map(String, Nullable(…))` variant.** On Nullable-valued maps a missing
  key reads as NULL, so `__null__` doubles as a "key absent" filter — and it
  was reachable data-driven under the old query shape, so the map gate
  restores parity. Zero extra query cost, response parsing stays "field 0
  only" (`getValuesFromResponse` unchanged for every query).

Semantics shift, accepted deliberately: `__null__` now means "this column
can be NULL" rather than "NULLs were observed among the top values in this
range". For a filter dropdown that is at least as useful (selecting
`__null__` to check for NULLs is a legitimate query that just matches
nothing when none exist), and the old semantics were popularity-conditional
anyway — `__null__` only appeared when NULLs cracked the top 100 by count.

Alternatives considered:
- `countIf(${column} IS NULL)` companion column in the same scan — exact
  data-driven detection, but it was the only unverified SQL left in the
  template, forced two-column response parsing for this one query, and its
  exactness buys nothing the type gate doesn't. Rejected for complexity.
- Append `__null__` unconditionally on every column — simplest, but plants
  a permanently dead entry in every non-Nullable column's dropdown.
  Rejected as UX noise.

### D5: `topK` size stays a constant (100)

Kept equal to today's `LIMIT 100` as a named constant next to the template.
Grafana's dropdown virtualizes fine at 100 and the client-side filter works
over it; making it configurable is more surface for no demonstrated need.

### D6: Preload lookback capped at 24h, range rounded via the existing `round` mechanism

The preload scans at most the trailing 24h of the dashboard range, and the
range is snapped to 5-minute boundaries so the interpolated SQL text repeats
verbatim across dropdown opens — a deterministic, repeatable statement
instead of a new query variant per second. Implementation reuses existing
machinery end-to-end:

- **Cap (frontend, one line per call site):** in `getTagValues` and
  `getTagKeysForMap`, the range passed to `executeQuery` becomes
  `{from: max(range.from, range.to − 86400s), to: range.to}`. The SQL
  template keeps plain `$__timeFilter(${timeColumn})` — no new WHERE shapes.
- **Rounding (backend, existing `round` path):** the metadata target carries
  `round: "5m"` instead of today's `round: ""` (`getQueryRunner`). The
  backend's `roundTimeRange` (`MutateQuery` / interpolator) snaps both
  endpoints to the nearest 5m *before* macro expansion, so the
  `$__timeFilter` literals are 5-minute-stable. An explicit per-query value
  also cleanly overrides any operator-set DS-level "Default round".
- **Enforcement (SQL channel, D2 table):**
  `hdx_query_max_timerange_sec = 87000`. The slack over 86400 exists because
  Go's `time.Round` rounds to *nearest*: when the cap binds, both endpoints
  shift identically (24h is a multiple of 5m) and the span stays exactly
  86400, but a natural range just under 24h can round outward past 86400 —
  exact-value enforcement would cancel precisely the harmless case. 87000 =
  cap + 2×round-interval, derived from the named constants so they cannot
  drift.

Semantics shift, accepted deliberately: on a 90-day dashboard, suggestions
come from the trailing 24h of the range — a value that last occurred 30 days
ago is not suggested (manual entry and the applied filter itself are
unaffected). This is the recency-bounded behavior the ticket asks for.

Alternative considered: expressing the cap in the SQL template
(`${timeColumn} >= toStartOfInterval($__toTime, …) - INTERVAL … `) —
rejected: `$__interval_s` is always `1` in the metadata-runner context
(interval is hardcoded to `"0"` and the macro floors to 1), SQL-side
rounding cannot stabilize the query text (the interpolated `$__toTime`
literal still changes every second), and non-trivial bounds on the primary
column risk defeating Hydrolix's WHERE-derived timerange detection and
partition pruning.

### D7: E2E verification of guardrail application at runtime

Unit tests pin the *configuration* (settings present on targets, capped
range object, `round` value); three e2e mechanisms pin *runtime* behavior,
each chosen to stay deterministic when test suites run in parallel:

- **Transmission — request interception.** A per-browser-context Playwright
  route on `**/api/ds/query**` (the existing `captureRequestBodies` pattern
  in `tests/helpers.ts`) asserts the preload payload carries the breaker in
  `querySettings` (and no `timeout_overflow_mode`), `round: "5m"`, the
  capped `from`/`to`, and the `SETTINGS` suffix in `rawSql`. This pins the
  browser→Grafana contract — everything this change modified; the
  payload→URL-parameter hop is unchanged `driver.go` behavior exercised by
  the Go testcontainers suite.
- **Rounding stability — UUID-tagged `query_log` lookup.** Interception
  cannot observe rounding (`roundTimeRange` runs in the backend *before*
  macro expansion, and the rounded SQL never returns to the browser), so the
  interceptor also *rewrites* the outgoing `rawSql`, appending a SQL comment
  with a fresh UUID per dropdown open. The comment rides through rounding
  and macro expansion untouched and the executed query is then found in
  `system.query_log` by that UUID — exactly one match, immune to parallel
  suites (unlike filtering by table name or event time). Two opens a few
  seconds apart on a `now`-relative range must yield byte-identical executed
  SQL once their UUID comments are stripped; the test waits past a 5-minute
  boundary when the first open would land too close to one, so the pair
  never legitimately straddles rounding windows.
- **Slow-source tolerance — `sleepEachRow` view.** ClickHouse has no
  read-blocking transactions, so deterministic slowness is a view over the
  fixture with `WHERE sleepEachRow(…) = 0` tuned to ≈9.99s total — just
  under the 10s breaker. The dropdown must still populate, proving no
  client-side layer (datasource `queryTimeout`, Grafana proxy, dropdown UI)
  gives up before the breaker budget. The *enforcement* half (cancel at the
  cap, partial results via `break`) cannot be observed on stock ClickHouse —
  `hdx_` settings are prefix-whitelisted but semantically inert there — and
  stays in task 1.3 on the real cluster, where the spike already
  demonstrated the break path.

## Risks / Trade-offs

- [`topK` is approximate; borderline values may be missing or misranked] →
  Acceptable by definition for a suggestions dropdown; free-text entry
  remains for anything not in the top 100. Proving signal: e2e asserts
  dominant fixture values are present, not exact ordering.
- [Cap too tight for a given range/cluster → empty (successful) preload
  result, dropdown shows no suggestions] → observed in the spike at 1s over
  90 days; the 10s default is >3× the budget that already returned values
  there, the breaker is overridable per-datasource (D3), and an empty list
  degrades to manual value entry — never an error dialog. Proving signal:
  e2e asserts an empty preload renders an empty dropdown without an error
  toast.
- [A DS-level timeout value parses oddly (template variable, `0`,
  non-numeric) and the min computation adopts it] → missing, non-numeric,
  and `≤ 0` values are ignored and the default 10 stands (D3). Proving
  signal: unit tests for larger / smaller / `0` / non-numeric / alias-named
  DS values.
- [A value the user needs last occurred earlier than the trailing 24h of
  the range, so it is not suggested (D6)] → accepted, this is the ticket's
  intended recency trade; manual entry and the applied filter are
  unaffected. Documented in the config/README task. Proving signal: e2e
  fixture places a value outside the window and asserts it is absent from
  suggestions but usable as a typed filter.
- [`hdx_query_max_timerange_sec` cancels a legitimate capped preload] →
  87000 slack covers the worst-case nearest-rounding expansion (D6 math);
  the capped-binding case lands on exactly 86400. Proving signal: spike
  query with a >24h range + the full `SETTINGS` suffix returns rows.
- [`$__adHocFilter()` macro fails to resolve the target table from inside
  the new subquery (AST-position resolution)] → Task 1 verifies expansion
  via the `/interpolate` resource; fallback is passing the CTE/table
  argument explicitly or keeping the filter in the outer query. Proving
  signal: Go/interpolate unit test with the new template.
- [Partial (timeout-truncated) results are indistinguishable from complete
  ones in the dropdown] → Accepted for v1; the dropdown is best-effort and
  values are still clickable/filterable. Grafana's tag-values API has no
  "partial" affordance to surface it.
- [A DS-level guardrail override leaks to dashboard queries (D3)] →
  Documented in the config help text task; existing DS-settings semantics.

## Open Questions

- None blocking. Verified on the real cluster (2026-08-24): the flat
  template with `timeout_overflow_mode='break'` in `SETTINGS`; driver-level
  `hdx_query_max_execution_time` + SQL-level `break` returning partial
  results (empty-but-successful under very tight caps). Still to settle in
  task 1: `hdx_query_max_timerange_sec` in the same `SETTINGS` clause on the
  cluster (capped >24h range must pass), dev-stack ClickHouse parity
  (including whether stock ClickHouse accepts `hdx_` settings on either
  channel or needs `custom_settings_prefixes`), and macro expansion of the
  final template via `/interpolate`.
