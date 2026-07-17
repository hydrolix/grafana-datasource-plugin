## Why

The plugin pins `github.com/hydrolix/clickhouse-sql-parser` at v0.3.0. v0.5.2 adds parsing features the metadata-hardening work needs — notably a broadened `DESCRIBE` grammar that accepts subqueries and table functions — and brings the fork's later fixes. The bump carries breaking API changes, so it is done as its own mechanical, behavior-preserving migration ahead of the security change that consumes the new grammar.

## What Changes

- Bump `github.com/hydrolix/clickhouse-sql-parser` from v0.3.0 to v0.5.2 in `go.mod` / `go.sum`.
- Migrate AST-to-SQL serialization from the removed `Expr.String()` to `parser.Format(node)` — the three call sites in `pkg/plugin/cte/cte.go` (`:96`, `:99`, `:120`).
- No changes required for `UnionAll`/`UnionDistinct`/`ArrayJoin` (unused), `DescribeQuery`/`VisitDescribeQuery` (unused), or custom visitors (all embed `parser.DefaultASTVisitor`, so the expanded visitor interface is satisfied for free).
- Behavior-preserving: CTE/macro extraction, macro positions, and interpolation output are unchanged; existing Go unit tests must stay green under `-race`.
- No frontend, wire-format, or public-API changes. Non-breaking for Grafana 10.x dashboards.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `hdx-interpolator`: pins the parser floor at v0.5.2 and records that AST serialization uses `parser.Format` (the `Expr.String()` method no longer exists), while preserving the existing `cte.GetMacroCTEs` extraction behavior.

## Impact

- Code: `go.mod`, `go.sum`, `pkg/plugin/cte/cte.go`. No other backend file references the removed/renamed symbols.
- Downstream: unblocks `secure-metadata-identifiers`, whose `QueryKeys` re-parse/shape check (D4) relies on v0.5.x's broadened `DESCRIBE` parsing. `close-adhoc-filter-injection` is independent of this upgrade.
- Risk surface: the parser underpins the whole query pipeline (interpolator, all macros, cte, routes), so regression rests on the existing test suite passing unchanged.
