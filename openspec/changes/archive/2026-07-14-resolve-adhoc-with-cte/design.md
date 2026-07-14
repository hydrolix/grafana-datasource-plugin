## Context

`cte.GetMacroCTEs` (`pkg/plugin/cte/cte.go`) walks each `SelectQuery` and records, per macro call site, the surrounding FROM expression as `CTE.CTE = parser.Format(expr.From.Expr)`. The ad-hoc filter macro passes that string to `MetadataProvider.GetKeys` → `QueryKeys` → `buildDescribeSQL` (`pkg/plugin/metadata.go`), which classifies it: a table identifier becomes `` DESCRIBE TABLE `db`.`tbl` ``; a subquery is wrapped and re-parsed to confirm a single `DescribeStmt` over a subquery.

For `WITH x AS (SELECT …) SELECT … FROM x WHERE $__adHocFilter()`, `expr.From.Expr` is the identifier `x`, so `CTE.CTE = "x"` and `buildDescribeSQL` emits `` DESCRIBE TABLE `x` `` — which errors because `x` is a CTE alias, not a table. The alias's definition lives in the same query's WITH clause, which the visitor already has access to but does not consult.

Relevant v0.5.2 AST: `SelectQuery.With *WithClause`, `WithClause.CTEs []*CTEStmt`, `CTEStmt{Alias Expr, Expr Expr}` (Alias is an `*Ident`, Expr is the defining subquery/expression).

## Goals / Non-Goals

**Goals:**
- When a SELECT's FROM is a bare identifier matching a WITH alias in scope, emit the alias's defining subquery (parenthesized) as `CTE.CTE`, so `buildDescribeSQL` describes the subquery.
- Preserve all existing behavior for non-alias FROM references (plain tables, `db.table`, inline subqueries).
- Keep the metadata layer untouched — it already handles a subquery string.

**Non-Goals:**
- Changing `QueryKeys`/`buildDescribeSQL` or the `GetKeys` signature.
- Relaxing any injection guard from `secure-metadata-identifiers` — the resolved subquery still passes through the re-parse/shape check.
- Resolving CTE references that are not simple identifiers (e.g. an alias used as a table function argument), or recursive/`WITH RECURSIVE` CTEs — out of scope; they fall back to the existing table-identifier path.

## Decisions

**D1 — Resolve aliases during CTE extraction, not in the metadata layer.** The visitor in `cte.go` already holds the enclosing `SelectQuery` (hence its `With` clause). Collect the in-scope WITH aliases there and, when `From.Expr` is an identifier equal to an alias, set `scope = "(" + parser.Format(aliasExpr) + ")"`. Rationale: keeps the metadata layer a pure consumer of a FROM-expression string; the AST needed to resolve the alias is only available at extraction time. No new parameter has to be plumbed into `QueryKeys`.
- _Alternative considered:_ pass the full query AST into `QueryKeys` and resolve there. Rejected — widens the metadata signature and duplicates scope tracking the visitor already does.

**D2 — Nearest enclosing scope wins for shadowed aliases.** WITH aliases are collected per SELECT; when a name is defined at multiple nesting levels, the innermost SELECT containing the macro resolves against its own then outer scopes, nearest first. Rationale: matches SQL scoping and the visitor's existing "innermost SELECT wins" preference for macro-to-CTE association.

**D3 — Only plain-identifier FROM references are resolved.** If `From.Expr` is not a bare identifier (it's already a subquery, `db.table`, table function, or JOIN), leave `scope` as today. Rationale: those cases are already handled correctly by `buildDescribeSQL`; alias resolution only needs to cover the bare-identifier-matching-a-WITH-alias case.

**D4 — Format the alias's defining expression through `parser.Format` and parenthesize.** The emitted `CTE.CTE` is `"(" + parser.Format(cteStmt.Expr) + ")"`. Rationale: `buildDescribeSQL` re-parses this and asserts a single `DescribeStmt` over a subquery, so the value is validated downstream exactly like an inline subquery — no new trust is introduced.

## Risks / Trade-offs

- **[A resolved subquery is itself invalid or unsupported by DESCRIBE, surfacing a new error where before there was a (different) error]** → Behavior only changes from "always errors" to "resolves when the CTE body is describable"; `buildDescribeSQL`'s re-parse guard already rejects anything malformed. Proving signal: unit test that a WITH alias over a plain `SELECT … FROM table` resolves and produces a `DESCRIBE (…)` that re-parses to a single `DescribeStmt`.
- **[Shadowed / nested aliases resolve to the wrong scope]** → Nearest-scope-wins is asserted by a test with an alias name reused at two nesting levels. Proving signal: unit test picks the innermost definition.
- **[An alias name collides with a real table name]** → An in-scope WITH alias takes precedence (matches ClickHouse resolution). Documented; covered by a test where an alias shadows a table name. Proving signal: unit test asserts the subquery is described, not the table.
- **[Security regression — a crafted CTE body reaches DESCRIBE unchecked]** → The resolved value flows through the same `buildDescribeSQL` re-parse/shape check as any subquery; no path bypasses it. Proving signal: existing `secure-metadata-identifiers` injection tests remain green, plus a test that a resolved subquery still goes through the shape check.
