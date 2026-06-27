## Why

The fork's `$__adHocFilter` macro at `0f83082` (`macros.go:190+`, ~250 LOC including `buildArrayCondition` / `buildFilterCondition`) turns Grafana's ad-hoc filter UI into a ClickHouse `WHERE` clause. It resolves the target table via either the macro argument or by matching the macro's AST position against the parsed CTEs (`GetMacroCTEs`), queries the table's primary key and column types through a `MetaDataProvider`, and emits per-filter conditions (`<col> IN (...)`, `<col> NOT LIKE ...`, `has(<col>, ...)` for arrays, etc.).

**Security finding.** The fork emits user-supplied filter values inside ClickHouse dollar-quoted literals: `$$%s$$` (e.g., `has(arr, $$<user-value>$$)`). The `$$` sequence inside a value terminates the literal — a value containing `$$` (or `'$$' or similar) breaks out of the quoted span. Catalog review correctly flagged this as a SQL-injection vector. The fork's existence under `github.com/hydrolix/sqlds` was the primary reason this code is not part of `grafana/sqlds`'s reviewed surface.

After C2 pins the plugin to sqlds at `ef925e1`, the macro disappears from the fork. The plugin owns it. This change ports the macro into `pkg/plugin/macros_adhoc.go`, ports the supporting `MetadataProvider` into `pkg/plugin/metadata.go`, and replaces every `$$%s$$` emission with `'%s'`-style single-quoted literals where `%s` is escaped via standard ClickHouse rules (`'` → `\'`, `\` → `\\`). The macro's user-facing behaviour (arg shape, returned WHERE-clause shape) is unchanged; the emitted SQL is just safe to parse.

`MetadataProvider` ports the fork's TTL-cached schema lookups (primary key per `(database, table)`, column-type map per CTE name). The cache is `jellydator/ttlcache/v3` with a one-hour TTL — matches the connection cache (C3). Schema queries are issued through the wrapper (`*HdxSqlDatasource`) via `ds.QueryData(...)`, which routes correctly through OAuth keying (C4) and connection caching (C3).

## What Changes

- Add `pkg/plugin/metadata.go` defining `MetadataProvider` and the `getPK` helper. `MetadataProvider` holds:
  - Reference back to `*HdxSqlDatasource` (issuing schema queries through `ds.QueryData(...)`).
  - `*ttlcache.Cache[string, string]` for `(database, table) → primary_key`.
  - `*ttlcache.Cache[string, map[string]string]` for `cte_name → {column_name: column_type}`.
- Add `pkg/plugin/macros_adhoc.go` defining `AdHocFilterMacro` (the macro) plus `buildArrayCondition`, `buildFilterCondition`, and an `escape(s string) string` helper. The macro is registered via `init()` into the `Macros` registry established by C5.
- **Security fix**: replace every `fmt.Sprintf("...$$%s$$...", ..., userValue, ...)` with `fmt.Sprintf("...'%s'...", ..., escape(userValue), ...)`. `escape` applies the standard ClickHouse single-quoted-literal escape rules: `'` → `\'`, `\` → `\\`. Applies to every value the macro interpolates.
- Update `pkg/plugin/hdx_sqlds.go` (from C2 + C3) to construct `MetadataProvider` and pass it to `NewHdxInterpolator`. The wrapper's constructor stores `*models.PluginSettings` (parsed once from `settings.JSONData`) so `MetadataProvider` can read the default database without re-parsing per query.
- Add `pkg/plugin/metadata_test.go` covering: cache hit/miss for both PK and key caches; cache-key derivation; TTL eviction; schema-query failure propagation; `getPK` happy-path + error cases.
- Add `pkg/plugin/macros_adhoc_test.go` covering: no-filters case (`1=1`); arg-count errors; CTE resolution via argument; CTE resolution via AST position; unknown-key filters are dropped; known-key filter conditions for `string` / `array` / `map` / numeric column types; **escape-correctness tests** for filter values containing `'`, `\`, `$$`, `\n`, NUL byte, Unicode characters.
- Playwright e2e adds one test that exercises an ad-hoc filter with a value containing a single quote (was a known fragility in the fork; now correctness-verified). Existing ad-hoc e2e coverage stays green.

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics. Dashboards calling `$__adHocFilter(...)` continue to work; the emitted SQL is just safer. Filter values that previously contained `$$` and silently broke queries now expand to valid SQL.

## Capabilities

### New Capabilities

- `hdx-adhoc-filter-macro-secure`: Plugin-owned `$__adHocFilter` macro that turns the Grafana ad-hoc filter UI into a ClickHouse `WHERE` clause, plus the TTL-cached `MetadataProvider` it uses for primary-key and column-type lookups. Emits single-quoted, escaped literals — not ClickHouse dollar-quoted literals.

### Modified Capabilities

- `hdx-interpolator`: the `Macros` registry gains the `adHocFilter` entry. Dispatch behaviour unchanged.
- `hdx-sqlds-wrapper`: `NewHdxSqlDatasource` parses `settings.JSONData` into `*models.PluginSettings` once and stores the result on the wrapper; passes the wrapper to `NewMetadataProvider`.

## Impact

- **Frontend**: none.
- **Backend (Go)**: new files `pkg/plugin/metadata.go`, `pkg/plugin/macros_adhoc.go`, paired `_test.go` files; `pkg/plugin/hdx_sqlds.go` updated to wire `MetadataProvider`.
- **Tests**: new unit tests for the macro and provider, with explicit escape-correctness cases. New Playwright e2e for an ad-hoc filter with quote/special-character values.
- **Dependencies**: none added or removed beyond what C3 brought in (`ttlcache/v3` is already direct).
- **User-visible**: filter values containing characters that previously broke the query (`$$`, certain Unicode-adjacent bytes) now produce valid SQL. No change for filter values containing only alphanumerics and common punctuation.
- **Security**: closes the catalog-review finding for the ad-hoc filter macro's value emission. `$$…$$` is eliminated; every user-supplied value is single-quoted-and-escaped before reaching the wire.
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2) for the wrapper, `plugin-ttl-connection-cache` (C3) for `ttlcache` as a direct dependency, `plugin-hdx-interpolator` (C5) for the `Macros` registry and `GetMacroCTEs`, and `plugin-clickhouse-time-date-macros` (C6) for shared macro-error patterns. Ships in the same coordinated merge window as C2-C7.
