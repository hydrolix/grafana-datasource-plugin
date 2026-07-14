## 1. WITH-alias resolution in CTE extraction

- [x] 1.1 `queryVisitor` now tracks a `scopeStack` of WITH-clause aliases via overridden `Enter`/`Leave` (push on `*SelectQuery` enter, pop on leave); the parser stores CTEs as `<Expr> AS <Alias>`, so for `name AS (SELECT …)` the name is `CTEStmt.Expr` and the body is `CTEStmt.Alias` (`*SelectQuery`).
- [x] 1.2 When `expr.From.Expr` is a bare identifier (via `bareIdentName`, unwrapping `JoinTableExpr → TableExpr → TableIdentifier`) matching an in-scope alias, `scope` becomes `"(" + parser.Format(body) + ")"`.
- [x] 1.3 `resolveWithAlias` searches the scope stack top-down (nearest scope wins on shadowing); a non-matching identifier keeps its identifier form.
- [x] 1.4 Non-identifier FROM expressions (inline subquery, `db.table`, table function, JOIN) are left untouched.

## 2. Unit tests — CTE extraction

- [x] 2.1 `TestGetMacroCTEs_WithAliasResolvesToSubquery`: `WITH x AS (SELECT a FROM events) … FROM x` → `CTE` is `(SELECT a FROM events)`, not `x`.
- [x] 2.2 `TestGetMacroCTEs_NonAliasIdentifierStaysTable`: non-matching identifier keeps the identifier form.
- [x] 2.3 `TestGetMacroCTEs_ShadowedAliasResolvesNearestScope`: same alias name in outer + inner WITH resolves to the inner (nearest) scope.
- [x] 2.4 `TestGetMacroCTEs_InlineSubqueryUnchanged` + existing plain/`db.table` tests: unchanged (regression).
- [x] 2.5 `TestGetMacroCTEs_AliasShadowsTableName`: a WITH alias sharing a real table's name resolves to the subquery, not the table (design risk #3).

## 3. Unit tests — end-to-end through metadata

- [x] 3.1 `TestBuildDescribeSQL_ResolvedWithAliasFlowsThroughShapeCheck`: the resolved subquery flows through `buildDescribeSQL` to `DESCRIBE (SELECT a FROM events)` that re-parses to a single `DescribeStmt` over a subquery.
- [x] 3.2 `secure-metadata-identifiers` injection tests still pass in the full `-race` run — the resolved subquery goes through the same re-parse/shape check, no guard bypass.
- [x] 3.3 `TestAdHocFilterMacro_WithCTEResolvesEndToEnd`: drives the macro's own resolution path (no explicit arg → internal `GetMacroCTEs` resolves the WITH alias → `GetKeys(resolved subquery)` → condition built), asserting `status = 'active'` for a `WITH x AS (…) FROM x` query.

## 4. Verification

- [x] 4.1 `go vet ./...` clean; `golangci-lint run` clean on changed files; `go test -race ./...` all green.
- [~] 4.2 E2E smoke SKIPPED (user-approved): ad-hoc filters over a WITH-CTE panel are not in the e2e suite (gap #13), and the shared macro/interpolation path was already validated by three prior green Playwright runs on near-identical code. Behavior is fully covered by the unit + macro-level end-to-end tests above (incl. `TestAdHocFilterMacro_WithCTEResolvesEndToEnd`). Broader interpolator-integration coverage tracked as a separate follow-up change.
