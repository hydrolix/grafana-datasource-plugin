## Why

The same injection class the catalog review flagged for ad-hoc filters is also open in the plugin's metadata lookups. `QueryPK` and `QueryKeys` build their SQL by string-formatting the database, table, and CTE names parsed from the user's query — with no identifier quoting or literal escaping — so a crafted name reaches `system.tables` or `DESCRIBE` verbatim. This must be closed before the signed plugin is published.

## What Changes

- Add a `quoteIdentifier` helper (backtick-quote + backtick-escape) in the Go backend; none exists today (`escape` only covers single-quoted literals).
- Harden `QueryPK` (`pkg/plugin/metadata.go`): `database` and `table` are string literals in the lookup, so escape them (via `escape`) before formatting.
- Rework `QueryKeys` (`pkg/plugin/metadata.go`): build `DESCRIBE` from validated AST identifier nodes (quoted) for real tables; resolve named CTEs via the AST; reject table functions / arbitrary FROM on the metadata path; keep parens only for the validated-subquery case, backed by a re-parse/shape check rather than the current "contains SELECT" heuristic.
- Constrain the explicit `$__adHocFilter(<arg>)` argument (`params[0]`) with a strict identifier check before it can reach the metadata path.
- Add Go unit tests for injection via database/table/CTE names and the explicit macro argument (run with `-race`).
- No wire-format or public-API changes. Honest queries behave identically; malformed identifiers are rejected. Non-breaking for Grafana 10.x dashboards.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `hdx-adhoc-filter-macro-secure`: adds requirements that metadata queries (`QueryPK`, `QueryKeys`) quote/escape identifiers and literals, that the metadata path rejects non-table FROM expressions and unsafe explicit args, and that a `quoteIdentifier` helper exists.

## Impact

- Code: `pkg/plugin/metadata.go` (`PrimaryKeyQuery`, `AdHocKeyQuery`, `QueryPK`, `QueryKeys`, new `quoteIdentifier`), `pkg/plugin/cte/cte.go` (AST identifier extraction reused for validation), `pkg/plugin/macros_adhoc.go` (`params[0]` check), tests in `pkg/plugin/metadata_test.go`.
- Data flow: values reaching `DESCRIBE` come from `c.CTE = expr.From.Expr.String()` (AST-reserialized) and from raw `params[0]`; `QueryPK` db/table come from `tableVisitor` AST nodes.
- Companion change `close-adhoc-filter-injection` closes the ad-hoc filter operator/map-key vectors of the same review; the two together fully close the injection class.
