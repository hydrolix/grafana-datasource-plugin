## Why

The plugin pins `github.com/hydrolix/sqlds/v5 v5.0.1` — the legacy fork carrying `HydrolixDatasource`, the AST interpolator, ad-hoc + ClickHouse macros, `MetadataProvider`, OAuth-token-keyed `Connector`, and TTL connection cache. Catalog review has flagged the fork: security-relevant code (query interpolation, OAuth header construction) lives outside maintained, reviewed sqlds. The migration target is the same fork repository at revision `ef925e1` (branch `feat/sqlds-extension`), which has been stripped to *only* the extension surfaces — `Interpolator` interface + `SQLDatasource.Interpolator` field, and `ConnectionCache` interface + `SQLDatasource.ConnectionCacheFactory` field. Once upstream `grafana/sqlds` merges and releases those extension points, the module path swaps in one line (the `retire-hydrolix-sqlds-fork` change). Until then, this change pins the fork at its extension-points-only tip.

The substrate this change establishes — pinning the new revision and adding a plugin-owned wrapper that embeds `*sqlds.SQLDatasource` — is the single seam every subsequent change in the migration sequence plugs into. Without it, the Hydrolix-specific code that lives in the fork today has no home in the plugin. With it, each fork capability (interpolator, macros, metadata, cache, OAuth keying) becomes a small, focused follow-up change that populates the wrapper's extension slots.

This change is intentionally narrow: it pins the revision, adds the wrapper, and updates plugin call sites to use it. Bringing the fork's Hydrolix-specific implementations into the plugin is deferred to changes C3 (cache), C4 (OAuth keying), C5 (interpolator), C6 (ClickHouse macros), C7 (ad-hoc filter + metadata). Those changes ship as a coordinated set; this change alone does not produce a working query path.

## What Changes

- **BREAKING (internal Go API)**: swap the imported module path from `github.com/hydrolix/sqlds/v5 v5.0.1` to `github.com/grafana/sqlds/v5 v5.0.0-20260613103402-ef925e15e15e` with a `replace` directive pointing at the hydrolix fork (the fork at this commit declares its module path as `github.com/grafana/sqlds/v5`). The fork at this revision no longer carries `HydrolixDatasource`, `Connector` as a public type with Hydrolix behaviour, the AST interpolator, or any Hydrolix macros — only the extension-point interfaces.
- Add `pkg/plugin/hdx_sqlds.go` defining `HdxSqlDatasource` — a plugin-owned wrapper that embeds `*sqlds.SQLDatasource`. Constructor `NewHdxSqlDatasource(driver)` returns the wrapper with `EnableMultipleConnections = true` and slots ready for subsequent changes to fill (`Interpolator`, `ConnectionCacheFactory`).
- Update `pkg/plugin/datasource.go` to construct via `NewHdxSqlDatasource`. The factory returns `*HdxSqlDatasource`; the embedded pointer's methods are promoted, so existing call sites continue to use `ds.QueryData(...)`, `ds.CheckHealth(...)`, `ds.Dispose()` unchanged.
- Update `pkg/api/routes.go`, `pkg/plugin/driver.go`, `pkg/plugin/driver_test.go`, `pkg/plugin/dssuit_test.go` to swap `*sqlds.HydrolixDatasource` references for `*HdxSqlDatasource`. The `sqlds.Driver` / `sqlds.QueryMutator` / `sqlds.QueryDataMutator` / `sqlds.QueryErrorMutator` / `sqlds.DriverSettings` / `sqlds.HeaderKey` references stay — those interfaces exist at the new pin.
- `pkg/plugin/driver.go`'s `Settings()` method asserts `ForwardHeaders = false` as a fixed slot. (The OAuth-keying flow in C4 depends on this; setting it now keeps the substrate-level invariant in one place.)
- Build-green stubs for the coordinated-set transition: `pkg/api/routes.go`'s `MacroCTEs` handler is temporarily stubbed (returns an empty CTE list); the `Interpolate` handler routes through `sqlds.DefaultInterpolator{}` when `ds.Interpolator == nil`, so the `/interpolate` endpoint returns the raw SQL unchanged. Behaviour parity is restored in C5 when the plugin-local interpolator + `GetMacroCTEs` land. **`go build`, `go test -race`, and Jest stay green at every commit in the coordinated set.**
- Go unit-test coverage: `HdxSqlDatasource` constructor smoke test (wires `EnableMultipleConnections=true`, returns a non-nil embedded `*sqlds.SQLDatasource`, panics on nil driver). No e2e regression expected once C3-C7 land alongside.

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics. Breaking is the Go-level import-graph swap, internal to this plugin.

## Capabilities

### New Capabilities

- `hdx-sqlds-wrapper`: Plugin-owned `HdxSqlDatasource` type that embeds `*sqlds.SQLDatasource`, centralizes extension-point wiring, and is the single place subsequent changes attach plugin-specific behaviour (interpolator, connection cache, OAuth keying).

### Modified Capabilities

<!-- None — the fork's HydrolixDatasource is not codified in any current spec. -->

## Impact

- **Frontend**: none.
- **Backend (Go)**: `go.mod` revision swap (no module-path change); new file `pkg/plugin/hdx_sqlds.go`; existing files updated to reference the wrapper instead of `sqlds.HydrolixDatasource`.
- **Tests**: constructor smoke test for the wrapper. Existing Go tests adjusted for the type-name swap. No e2e impact in isolation; full e2e re-runs once C3-C7 land.
- **Dependencies**: `github.com/hydrolix/sqlds/v5` revision moves to `ef925e1`. Module path stays — the public path-swap to `grafana/sqlds/v5` is `retire-hydrolix-sqlds-fork` (C8), gated on upstream release.
- **User-visible**: none in isolation. Combined with C3-C7, no user-visible change either; the migration preserves panel queries, ad-hoc filters, annotations, OAuth-keyed pooling, and TTL eviction.
- **Security**: this change does not itself address the catalog-review finding — the security fix to the ad-hoc filter macro (`$$…$$` → escaped single-quoted) lives in `plugin-adhoc-filter-macro-secure` (C7). This change establishes the substrate that lets C7 land cleanly.
- **Sequencing**: depends on `extract-hdx-query-models` (C1) landing first so data shapes are already plugin-owned. Blocks C3-C8 — every later change in the migration sequence assumes the wrapper exists.
