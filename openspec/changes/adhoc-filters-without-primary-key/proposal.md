## Why

Ad-hoc filters assume every table has a primary key to use as its time
column. Tables with no primary key are common in Hydrolix, and today they
fail in two different ways: the value dropdown silently comes back empty,
and Map-column key discovery emits invalid SQL. Users of those tables have
no working ad-hoc filtering at all, with no error explaining why.

## What Changes

- Ad-hoc value preload degrades to an unfiltered scan when the target table
  has no primary key, instead of returning an empty suggestion list.
- Map-column ad-hoc key discovery degrades the same way, instead of
  emitting a predicate against an empty column name.
- The value and map-key SQL templates gain an optional time-filter slot;
  the statement builders leave it empty when the resolved primary key is
  the empty string.
- The trailing-24h window cap and 5-minute endpoint rounding apply only to
  the time-filtered form — there is no scan window to cap or round without
  a time predicate.
- The `topK` bound, the execution-time circuit breaker, and the existing
  guardrail settings suffix continue to apply unconditionally, so the
  keyless form stays bounded.
- A primary-key lookup that fails is no longer memoized on the frontend and
  never selects the unfiltered form; only a lookup that returns the empty
  string does.
- The PK-resolving backend time macros surface a table with no primary key
  as a typed, actionable error rather than expanding into invalid SQL.
- Template slots are filled literally, closing a pre-existing `$`-pattern
  substitution bug in the ad-hoc condition and column slots.
- Not **BREAKING**: tables with a primary key keep their current query
  shape, settings, and window behavior byte-for-byte.
- Covered by frontend unit tests, Go unit tests, and Playwright e2e against
  a keyless ClickHouse fixture.

## Capabilities

### New Capabilities

None. The change extends two existing capabilities rather than introducing
a new surface.

### Modified Capabilities

- `adhoc-value-preload`: the value preload and map-key discovery queries
  are no longer unconditionally time-filtered; a keyless table gets a
  variant with no time predicate, and the window-capping and rounding
  requirements become conditional on that predicate being present. The
  requirement that the dropdown populate for a keyless table is new.
- `hdx-clickhouse-time-date-macros`: the PK-lookup macros must treat an
  empty primary key as a distinct, typed failure instead of interpolating
  it as a column name.
- `hdx-adhoc-filter-macro-secure`: the primary-key lookup returns the
  empty string as a legitimate, already-fetched value for a table that
  declares no primary key; only a missing table is an error, and a failed
  lookup is never stored.

## Impact

- `src/constants.ts` — `AD_HOC_VALUE_QUERY`, `AD_HOC_MAP_KEY_QUERY` and
  their keyless counterparts.
- `src/ast.ts` — the value and map-key statement builders.
- `src/datasource.ts` — `getTagValues` and `getTagKeysForMap`, including
  the preload-range resolution that only applies to the filtered variant.
- `pkg/plugin/metadata.go` — primary-key lookup and its cache.
- `pkg/plugin/macros_clickhouse.go` — column resolution for the
  zero-argument time macros.
- No changes to `src/plugin.json`, the datasource config surface, or any
  dashboard-query path. No new dependencies.
