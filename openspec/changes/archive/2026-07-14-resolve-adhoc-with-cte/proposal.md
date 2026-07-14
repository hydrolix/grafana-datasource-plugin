## Why

Ad-hoc filters don't work when a panel query selects from a WITH-clause CTE — e.g. `WITH x AS (SELECT …) SELECT … FROM x WHERE $__adHocFilter()`. The macro's schema lookup resolves the FROM expression to the bare alias `x` and issues `DESCRIBE TABLE \`x\``, which fails at ClickHouse because `x` is not a real table. The `secure-metadata-identifiers` change deliberately deferred this (keeping it safely rejected rather than injectable); this change makes it actually resolve.

## What Changes

- Enhance CTE extraction in `pkg/plugin/cte/cte.go` so that when a SELECT's FROM expression is an identifier matching a WITH-clause alias in scope, the emitted `CTE.CTE` is the alias's defining subquery (parenthesized) instead of the bare alias name.
- No change to the metadata layer's signature or public API: `buildDescribeSQL` already handles a subquery string via its wrap + re-parse + shape-check path.
- A FROM reference that matches no in-scope WITH alias continues to be treated and validated as a table identifier (unchanged).
- Go unit test coverage for CTE resolution (alias → subquery, non-match → table, nested/shadowed alias picks nearest scope). Ad-hoc filters are not yet in the e2e suite (gap #13).
- Non-breaking for Grafana 10.x dashboards.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `hdx-adhoc-filter-macro-secure`: the "Metadata key lookup builds DESCRIBE from validated shapes" requirement currently states WITH-alias resolution is out of scope; this change flips that so an in-scope WITH alias is resolved to its subquery and described.
- `hdx-interpolator`: `cte.GetMacroCTEs` now emits the resolved subquery for WITH-alias FROM references rather than the alias name.

## Impact

- Code: `pkg/plugin/cte/cte.go` (`queryVisitor` / new WITH-alias resolution), tests in `pkg/plugin/cte/cte_test.go` and `pkg/plugin/metadata_test.go`.
- Data flow: `CTE.CTE` for a WITH-alias FROM changes from `x` to `(SELECT …)`; the metadata layer consumes it unchanged.
- Security invariant preserved: the resolved value is a parser-produced subquery that still flows through `buildDescribeSQL`'s re-parse/shape check; nothing bypasses the injection guards added in `secure-metadata-identifiers`.
- Depends on the parser features already in place from `upgrade-sql-parser` (v0.5.2 `WithClause`/`CTEStmt`, broadened `DESCRIBE`).
