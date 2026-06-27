# plugin-adhoc-filter-macro-secure — implementation tasks

## 1. `MetadataProvider` (replaces C5 stub)

- [x] 1.1 Rewrite `pkg/plugin/metadata.go`. Delete the C5 stub (`MetadataProvider struct{}`, `ErrMetadataProviderUnavailable`, stub `getPK`).
- [x] 1.2 Define a small `metadataDS` interface (`QueryData`, `InstanceSettings`, `DefaultDatabase`) so tests can supply a fake without going through sqlds. `*HdxSqlDatasource` implements it via the embedded `*sqlds.SQLDatasource` + the new accessors added in task 4.
- [x] 1.3 `MetadataProvider` struct: `ds metadataDS`, `pkCache *ttlcache.Cache[string, string]`, `keyCache *ttlcache.Cache[string, map[string]string]`. 1-hour TTL on both caches.
- [x] 1.4 `NewMetadataProvider(ds metadataDS) *MetadataProvider` — constructs both caches and starts their sweep goroutines.
- [x] 1.5 Port `GetPK`, `GetKeys` (cache-then-query), `QueryPK`, `QueryKeys`, `executeQuery`, `getDefaultDatabase`, `GetStringSafe`. Constants `PrimaryKeyQuery`, `AdHocKeyQuery`. Sentinel errors `ErrPrimaryKeyNotFound`, `ErrAdHocKeysNotFound` (wrap with `backend.PluginError`).
- [x] 1.6 `executeQuery` builds the synthetic `*backend.QueryDataRequest` using `req.SetHTTPHeader(key, value)` rather than writing directly to `req.Headers` — non-special headers (`X-Grafana-Org-Id`) only round-trip through `GetHTTPHeaders` when stored under the `http_` prefix. Without this, schema queries lose the org id and route through the wrong cache entry.
- [x] 1.7 Free function `getPK(ctx, rawSQL, pos, mdProvider, headers)` — parses the SQL, locates the CTE at `pos` via `cte.GetMacroCTEs`, calls `mdProvider.GetPK(ctx, headers, cte.Database, cte.Table)`. Drop the `//nolint:unused` from the C5 stub since C7's macros call it.

## 2. `pkg/plugin/macros_adhoc.go` (new)

- [x] 2.1 Create `pkg/plugin/macros_adhoc.go`. Constants `SyntheticNull`, `SyntheticEmpty`, `RegexPrefix`; package-level `mapTypeFilterKey` regex.
- [x] 2.2 `escape(s string) string` — D1 implementation. Single-pass `strings.Builder`. Escape `'`, `\`, `\n`, `\r`, `\t`, NUL.
- [x] 2.3 `AdHocFilterMacro` — port the fork's resolver: argument-first CTE, AST-position fallback (`cte.GetMacroCTEs(...)`), `mdProvider.GetKeys`, per-filter dispatch, `1=1` on no matches, AND-join on multiple matches.
- [x] 2.4 `buildFilterCondition` — port modulo D2: every `$$%s$$` site becomes `'%s'` with `escape(value)`. Affects: `=~`/`!~` LIKE / `match` paths; the generic `<col> <op> $$<val>$$` fallback.
- [x] 2.5 `buildArrayCondition` — same D2 treatment. `has(col, '<escaped>')`, `not has(...)`, OR-joined for `=|` / `!=|`.
- [x] 2.6 `getJoinedValues` — same. `''` for synthetic empty, escape every other.
- [x] 2.7 `escapeWildcard` ported verbatim (handles `*` → `%` for LIKE).
- [x] 2.8 `getRegexValue` ported verbatim (`regex:` prefix detection).
- [x] 2.9 `init()` block: `Macros["adHocFilter"] = AdHocFilterMacro`. Matches the C5 registry shape.

## 3. Wire `MetadataProvider` in the wrapper

- [x] 3.1 In `pkg/plugin/hdx_sqlds.go`, add fields to `HdxSqlDatasource`: `Settings *models.PluginSettings` (parsed best-effort; nil on failure), `instanceSettings backend.DataSourceInstanceSettings` (cached for synthetic schema-query requests).
- [x] 3.2 Add methods `InstanceSettings() backend.DataSourceInstanceSettings` and `DefaultDatabase() string` (returns `""` when `Settings == nil`).
- [x] 3.3 In `NewHdxSqlDatasource(driver, settings)`: parse settings via `models.NewPluginSettings(context.Background(), settings)` once; construct `wrapper` first (so `MetadataProvider` can close over it); then set `ds.Interpolator = NewHdxInterpolator(NewMetadataProvider(wrapper), Macros)`.
- [x] 3.4 Confirm the order still respects C3's `ds.ConnectionCacheFactory` assignment.

## 4. Tests

- [x] 4.1 Add `pkg/plugin/metadata_test.go`. Tests:
  - `GetStringSafe` for `string`, `*string`, `nil *string`, unsupported type.
  - `GetPK` cache hit (pre-seeded `pkCache`) returns without calling `ds`.
  - `GetPK` cache miss calls `QueryPK` once, stores result, second call returns cached value (assert call count on fake).
  - `QueryPK` with an empty frame yields `ErrPrimaryKeyNotFound`.
  - `QueryPK` happy-path with a one-row frame yields the cell.
  - `GetKeys` cache hit + cache miss + multi-column happy path.
  - `executeQuery` builds the request with `SetHTTPHeader` so `X-Grafana-Org-Id` survives (assert via a fake `metadataDS` that inspects the request's `GetHTTPHeaders().Get(OrgIdHeaderKey)`).
  - `getDefaultDatabase` returns `Settings.DefaultDatabase`; returns an error when `Settings == nil`.
- [x] 4.2 Add `pkg/plugin/macros_adhoc_test.go`. Ported test cases from the fork — every expected output updated from `$$%s$$` to `'%s'` (escape applied):
  - `TestAdHocFilterMacro` matrix (~22 cases): no-filters, single eq, multi-filter, null/empty values, regex match/not match, multi-value IN/NOT IN, array `has`, map column with key syntax, mixed string+array+map.
  - `TestBuildFilterCondition` (~30 cases).
  - `TestBuildArrayCondition` (~10 cases).
  - `TestBuildFilterConditionWithMaps`.
  - `TestEscapeWildcard`.
  - `TestAdHocFilterMacroWithExplicitTable` (argument-resolution path).
  - `TestAdHocFilterMacroWithTooManyParams` (arg-count error).
- [x] 4.3 New escape-correctness tests (no fork equivalent):
  - `TestEscape_SingleQuote`: `O'Reilly` → `O\'Reilly`.
  - `TestEscape_Backslash`: `a\b` → `a\\b`.
  - `TestEscape_DollarDollar`: `payload$$end` does NOT contain `$$` in the output (i.e. is now single-quoted, not dollar-quoted).
  - `TestEscape_Newline`: `\n` → `\\n` (the two-char sequence backslash-n).
  - `TestEscape_NUL`: `\x00` → `\\0`.
  - `TestEscape_UnicodeRoundTrip`: multi-byte UTF-8 passes through verbatim.
  - `TestEscape_QuotedRoundTrip` (property-style): for a corpus of values containing every byte from 0..127 plus a few Unicode codepoints, `'<escape(v)>'` lexes as the literal `v` per ClickHouse's grammar (asserted by re-parsing in the test rather than calling the server).

## 5. Quality gates

- [x] 5.1 `go build ./...` clean.
- [x] 5.2 `go vet ./...` clean.
- [x] 5.3 `golangci-lint run --new-from-rev=HEAD` clean.
- [x] 5.4 `go test -race ./...` green.
- [x] 5.5 `npm run typecheck && npm run lint && npm run test:ci` green.
- [x] 5.6 Playwright e2e — defer to coordinated-set verification (C5 + C6 + C7 together produce the full query path). The single new e2e (`adhoc filter with O'Reilly`) is captured in `tests/` for that pass.

## 6. Commit

- [x] 6.1 Single commit including code + design + tasks + specs.
- [x] 6.2 Commit message: `pkg/plugin: secure ad-hoc filter macro + MetadataProvider (C7)`. Body summarises: (a) value emissions switch from `$$...$$` to single-quoted escaped literals — security fix; (b) MetadataProvider replaces C5 stub with TTL-cached PK/keys lookups; (c) wrapper parses settings once at construction.
