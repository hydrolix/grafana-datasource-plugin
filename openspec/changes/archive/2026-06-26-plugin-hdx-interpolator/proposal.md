## Why

The fork at `0f83082` carries a CTE-aware AST `Interpolator` in `interpolator.go` (328 LOC) that replaces the regex-based `sqlutil.Interpolate`: parse `query.RawSQL` into a ClickHouse AST via `github.com/hydrolix/clickhouse-sql-parser`, walk the AST to find macro call sites with their `parser.Pos`, dispatch each macro with positional context, and rewrite the SQL. A final post-rewrite mutation pass adapts the SQL for execution against Hydrolix (table-reference fixups, CTE inlining). The same package exposes `GetMacroCTEs` and the `CTE` type, used by the `/macro-ctes` HTTP resource at `pkg/api/routes.go:85-117` for the dashboard's macro-expansion preview.

After C2 pins the plugin to sqlds at `ef925e1`, the fork no longer carries any interpolator. sqlds at `ef925e1` exposes the extension surface — `Interpolator` interface + `SQLDatasource.Interpolator` field — but the default implementation (`DefaultInterpolator`) is a one-line wrapper around `sqlutil.Interpolate` that knows only the legacy `sqlutil.MacroFunc` macros returned by `Driver.Macros()`. Without a plugin-side interpolator, the AST-aware passes disappear (CTE-aware macros stop working) and the macros C6 and C7 introduce have no dispatch path.

This change lifts the AST interpolator into the plugin as `HdxInterpolator`, satisfying `sqlds.Interpolator`. It owns the full rewrite pipeline — AST parse, position-aware macro dispatch, post-rewrite mutation. It also brings `GetMacroCTEs` and `CTE` into the plugin so `pkg/api/routes.go:85-117` compiles. Macros themselves are not in this change — `HdxInterpolator` ships with an empty registry that C6 (ClickHouse time/date) and C7 (ad-hoc filter + metadata provider) populate.

## What Changes

- Add `pkg/plugin/interpolator.go` defining `HdxInterpolator` satisfying `sqlds.Interpolator`. Owns the full `Interpolate(ctx, ds, query, rawJSON) (string, error)` call: parse `query.RawSQL` into a ClickHouse AST via `github.com/hydrolix/clickhouse-sql-parser`, dispatch macros at their AST positions, run the post-rewrite mutation pass, return the rewritten SQL.
- Add `pkg/plugin/macros_registry.go` with the package-level `Macros map[string]MacroFunc` registry and the `MacroFunc` type signature (`func(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, md *MetadataProvider) (string, error)`, mirroring the fork at `0f83082`).
- Add `pkg/plugin/cte.go` containing `CTE` struct + `GetMacroCTEs(expr parser.Stmts) (map[string]CTE, error)`. `pkg/api/routes.go:85-117` switches its `sqlds.GetMacroCTEs` / `sqlds.CTE` references to the plugin-local ones.
- Update `pkg/plugin/hdx_sqlds.go` (from C2) to wire `ds.Interpolator = NewHdxInterpolator(metadataProvider, Macros)` where `metadataProvider` is constructed from the `*HdxSqlDatasource`. `MetadataProvider` itself is defined in C7; until C7 lands, the field is initialised to a nil-safe sentinel that callers (ad-hoc macro in C7) handle.
- Add `pkg/plugin/interpolator_test.go` with golden-SQL fixture coverage of the AST passes — CTE-aware macros, position-correct dispatch, post-rewrite mutation regression cases. Test corpus mirrors the fork's `interpolator_test.go` (463 LOC at `0f83082`) ported verbatim modulo type renames (`*HDXQuery` → `*models.HdxQuery`, `*HydrolixDatasource` → `*HdxSqlDatasource`).
- Add `pkg/plugin/cte_test.go` for `GetMacroCTEs` happy-path + parse-failure surfaces.
- `go.mod` adds `github.com/hydrolix/clickhouse-sql-parser` as a direct dependency (was transitive through the fork at `v5.0.1`).
- Playwright e2e coverage unchanged in isolation; runs at the end of the C2-C7 merge window.

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics — the AST passes match the fork's behaviour byte-for-byte modulo macro-table population (C6, C7).

## Capabilities

### New Capabilities

- `hdx-interpolator`: Plugin-owned AST-based `sqlds.Interpolator` implementation. Parses ClickHouse SQL, dispatches plugin macros at their AST positions, runs a post-rewrite mutation pass, returns the rewritten SQL. Owns the macro registry (`Macros map[string]MacroFunc`), the macro signature, the CTE-extraction helper (`GetMacroCTEs`).

### Modified Capabilities

- `hdx-sqlds-wrapper`: `NewHdxSqlDatasource` now wires `ds.Interpolator = NewHdxInterpolator(...)`. The wrapper's slot from C2 gets filled here.

## Impact

- **Frontend**: none.
- **Backend (Go)**: new files `pkg/plugin/interpolator.go`, `pkg/plugin/macros_registry.go`, `pkg/plugin/cte.go`, paired `_test.go` files; `pkg/plugin/hdx_sqlds.go` updated to assign `ds.Interpolator`; `pkg/api/routes.go` import-only change from `sqlds.GetMacroCTEs` / `sqlds.CTE` to plugin-local.
- **Tests**: ported interpolator + CTE-extraction tests; existing tests adjust to the type renames.
- **Dependencies**: `github.com/hydrolix/clickhouse-sql-parser` promotes from indirect to direct (it was transitive through the fork). No new third-party package added.
- **User-visible**: none in isolation. Combined with C6 + C7, panel queries continue to expand `$__fromTimeFilter`, `$__toTimeFilter`, `$__timeFilter`, `$__dateFilter`, `$__timeInterval`, `$__adHocFilter`, etc., identically to today.
- **Security**: no surface change. The post-rewrite mutation pass moves verbatim; the security fix to ad-hoc filter values lives in C7.
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2). Independent of C3 and C4 at the file level; ships in the same coordinated merge window. C6 and C7 each populate `Macros` after this change establishes the registry.
