## Context

`github.com/hydrolix/clickhouse-sql-parser` is pinned at v0.3.0 and used across the query pipeline: `interpolator.go`, `macros_clickhouse.go`, `macros_adhoc.go`, `macros_registry.go`, `metadata.go`, `cte/cte.go`, and `api/routes.go`. The version history skips v0.4.x; tags go v0.3.0 → v0.5.1 → v0.5.2 (the in-repo `VERSION` constant lags at `0.4.17`). The diff is an upstream AfterShip sync plus fork commits (Grafana `${var}` template parsing, `$$…$$` literals, `#` comments, GLOBAL NOT IN, REGEXP operators, and a broadened `DESCRIBE`).

The bump is wanted for the broadened `DESCRIBE` grammar (accepts subqueries and table functions, plus a trailing `SETTINGS` clause), which `secure-metadata-identifiers` needs for its assemble-then-re-parse shape check.

## Goals / Non-Goals

**Goals:**
- Move the pin to v0.5.2 with a minimal, mechanical, behavior-preserving diff.
- Fix the only breaking usage (`Expr.String()` removal) with the sanctioned replacement.
- Keep the entire existing Go test suite green under `-race` as the regression signal.

**Non-Goals:**
- Adopting any new parser feature. Consuming the broadened `DESCRIBE` belongs to `secure-metadata-identifiers`.
- Refactoring CTE extraction or interpolation logic beyond the serialization-call swap.

## Decisions

**D1 — Replace `Expr.String()` with `parser.Format(node)`.** v0.5.2 removed `String()` from the `Expr` interface and every node; serialization moved to `parser/format.go`. The package-level `parser.Format(expr) string` produces the compact single-line form matching the old `String()` output. Apply at `cte.go:96` (`expr.Table`), `:99` (`expr.Database`), and `:120` (`expr.From.Expr`). Rationale: `parser.Format` is the direct, documented successor and yields equivalent SQL text, so the extracted `Table`/`Database`/`CTE` strings are unchanged.
- _Alternative considered:_ construct a `parser.NewFormatter()` per call. Rejected — more verbose with no benefit for compact output.

**D2 — No visitor changes.** All three visitors in `cte.go` embed `parser.DefaultASTVisitor`, which satisfies the expanded v0.5.2 `ASTVisitor` interface (new/renamed `Visit*` methods) without edits. Rationale: the codebase deliberately embeds the default; only direct interface implementers would need updates, and there are none.

**D3 — No migration for Union/ArrayJoin/DescribeQuery.** A repo scan shows zero references to `UnionAll`, `UnionDistinct`, `ArrayJoin`, `DescribeQuery`, or `VisitDescribeQuery`. Rationale: those breaking changes do not touch this codebase; do not add speculative handling.

**D4 — Regression is the existing suite.** No new test logic is required for the upgrade itself; the contract is that `cte_test.go`, `interpolator_test.go`, and the macro tests pass unchanged. Rationale: a behavior-preserving dependency bump is proven by unchanged tests, not new ones.

## Risks / Trade-offs

- **[`parser.Format` output differs subtly from old `String()` (spacing, quoting), shifting extracted Table/Database/CTE strings]** → The existing `cte_test.go` scenarios assert exact `Table`/`Database` values; run them post-bump. Proving signal: `go test -race ./pkg/plugin/cte/...` and `./pkg/plugin/...` green with no scenario edits.
- **[A transitive behavior change in the broadened grammar alters interpolation output for existing SQL]** → Full `interpolator_test.go` and macro suites act as the guard. Proving signal: `go test -race ./...` green; if any test needs editing, treat it as a real behavior change and escalate rather than adjusting the assertion.
- **[go.sum / transitive dependency drift]** → Run `go mod tidy` and `go mod verify`; review the `go.sum` delta. Proving signal: `go build ./...` and `go vet ./...` clean.

## Migration Plan

1. `go get github.com/hydrolix/clickhouse-sql-parser@v0.5.2` then `go mod tidy`.
2. Swap the three `cte.go` `.String()` calls to `parser.Format(...)`.
3. `go build ./... && go vet ./... && golangci-lint run && go test -race ./...`.
4. Rollback: revert the go.mod/go.sum pin and the three-line `cte.go` change — no data or schema migration is involved.
