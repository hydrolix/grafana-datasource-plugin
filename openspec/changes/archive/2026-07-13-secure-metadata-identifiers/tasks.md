## 0. Prerequisite

- [x] 0.1 Ensure `upgrade-sql-parser` (clickhouse-sql-parser v0.5.2) is applied first; the D4 re-parse check uses `parser.DescribeStmt` and the broadened `DESCRIBE` grammar unavailable in v0.3.0.

## 1. Identifier quoting helper

- [x] 1.1 Add `quoteIdentifier(s string) (string, error)` in `macros_adhoc.go` (backtick-wrap; reject embedded backtick and NUL — the lexer does not unescape inside backtick identifiers so escaping cannot round-trip); documented that `escape` remains for single-quoted literals only.
- [x] 1.2 Unit test (`TestQuoteIdentifier`): round-trips quotes, backslashes, spaces, dots, and multi-byte UTF-8; rejects embedded backtick and NUL.

## 2. Harden QueryPK (literal escaping)

- [x] 2.1 In `QueryPK` (`pkg/plugin/metadata.go`), route `database` and `table` through `escape()` before `fmt.Sprintf(PrimaryKeyQuery, ...)`.
- [x] 2.2 Unit test (`TestMetadataProvider_QueryPK_EscapesLiterals`): a `'`-bearing table name stays inside the escaped literal and does not add an `OR '1'='1'` clause.

## 3. Rework QueryKeys (DESCRIBE from validated shapes)

- [x] 3.1 Removed the `strings.Contains(..., "SELECT")` heuristic; `buildDescribeSQL` determines the target shape from the parsed AST. Dead `AdHocKeyQuery` template removed.
- [x] 3.2 Real table / `db.table`: emit `` DESCRIBE TABLE `db`.`tbl` `` via `quoteIdentifier` from the AST identifier nodes.
- [~] 3.3 DEFERRED (user-approved): WITH-alias named CTEs are not expanded to their subquery — a bare alias is treated as a quoted table identifier (errors at ClickHouse, as before; no injection). Full-AST resolution tracked as follow-up change `resolve-adhoc-with-cte`. Spec + design amended accordingly.
- [x] 3.4 Genuine subquery: `describeSubquery` assembles `DESCRIBE (...)`, re-parses, and asserts exactly one `parser.DescribeStmt` whose `Target` is a subquery; rejects on parse failure or shape mismatch.
- [x] 3.5 Reject table functions (`url`, `remote`, `s3`, `file`, `numbers`, …), JOINs, and other arbitrary FROM expressions with a typed error.

## 4. Constrain explicit macro argument

- [x] 4.1 In `AdHocFilterMacro`, validate `params[0]` against `explicitCTEArg` (strict identifier, optionally `database.table`) before it reaches the metadata path; reject otherwise.
- [x] 4.2 Unit tests: injected explicit argument is rejected before any key lookup (`nopMetadataDS` proves no upstream call); honest `events` is accepted.

## 5. Injection regression tests

- [x] 5.1 `TestBuildDescribeSQL` / `TestMetadataProvider_QueryKeys_*`: real table → quoted DESCRIBE; table function → typed error (before any query); `t) UNION ALL SELECT ... --` cannot inject; `t; DROP ...` rejected; `t UNION ALL SELECT ... secrets` reduces to `DESCRIBE TABLE \`t\``.
- [x] 5.2 Honest shapes (plain table, `db.table`, quoted `` `db`.`events` ``, subquery) still resolve; existing `GetKeys`/`QueryKeys` cache tests still pass.

## 6. Verification

- [x] 6.1 `go vet ./...` clean; `golangci-lint run` clean on changed files; `go test -race ./...` all green.
- [x] 6.2 E2E smoke via the `grafana-plugin-e2e` skill: rebuilt the container backend (confirmed `clickhouse-sql-parser v0.5.2`), full Playwright suite **32/32 passed (1.9m)** on Grafana 13.0.1. The `macroFunctions` tests exercise the `QueryPK`/PK-resolution metadata path, confirming the escaped-literal path resolves schema end-to-end. Ad-hoc filters themselves are not yet in the e2e suite (gap #13).
