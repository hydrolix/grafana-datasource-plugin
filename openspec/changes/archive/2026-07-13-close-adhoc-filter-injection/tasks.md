## 1. Operator allowlist (scalar/map path)

- [x] 1.1 Added `scalarComparisonOperators` set (`=`, `!=`, `<`, `<=`, `>`, `>=`) in `pkg/plugin/macros_adhoc.go`; the `=|`/`!=|`/`=~`/`!~` operators keep their own switch cases.
- [x] 1.2 In `buildFilterCondition`, the `default` branch now rejects any operator not in the allowlist with `operator %q unsupported for scalar value`, mirroring `buildArrayCondition`.
- [x] 1.3 Confirmed the `=|`/`!=|`/`=~`/`!~`/NULL/empty cases still fall through; only the `default` branch changed (existing matrix tests pass).

## 2. Map key/subscript validation + quoting

- [x] 2.1 Reused the shared `quoteIdentifier` introduced by `secure-metadata-identifiers` (backtick-wrap; rejects embedded backtick/NUL) instead of a local helper.
- [x] 2.2 `AdHocFilterMacro` computes the SQL key via `adHocMapKey`: base column (schema-validated) → `quoteIdentifier`, subscript → `escape` inside `['...']`, so the key is `` `col`['<escaped subscript>'] ``.
- [x] 2.3 `buildFilterCondition`/`buildArrayCondition` now take the pre-built `key` param; the raw `filter.Key` is no longer used for map columns. Non-map keys pass the validated column through unchanged.

## 3. Unit tests

- [x] 3.1 `TestBuildFilterCondition_OperatorAllowlist`: every allowlisted operator yields a nil error + non-empty condition.
- [x] 3.2 `TestBuildFilterCondition_RejectsInjectedOperator` + `TestAdHocFilterMacro_RejectsInjectedOperator`: injected operators (`= 'x' OR 1=1 -- `, `= 'x') OR (1=1`, `BETWEEN`, `; DROP TABLE t`, empty, whitespace) return an error and emit no condition.
- [x] 3.3 `TestAdHocFilterMacro_MapKeySubscriptCannotBreakOut`: `mapColumn['a'] OR 1=1 --']` is neutralized — the injected text survives only inside the escaped map-access literal (`` `mapColumn`['a\'] OR 1=1 --'] = 'v' ``), the quote is escaped so the literal is not terminated early.
- [x] 3.4 `TestAdHocFilterMacro_MapKeyHonestIsQuoted` (golden `` `mapColumn`['env'] = 'prod' ``) and `TestAdHocFilterMacro_MapKeySubscriptQuoteEscaped` (subscript quote backslash-escaped). Existing matrix map cases updated to the quoted form.
- [x] 3.5 `TestBuildFilterCondition_ValueMetacharactersEscaped`: quotes/backslashes/SQL metacharacters at the value position stay inside the quoted literal, escaped.

## 4. Verification

- [x] 4.1 `go vet ./...` clean; `golangci-lint run` clean on changed files; `go test -race ./...` all green.
- [x] 4.2 E2E smoke via the `grafana-plugin-e2e` skill: rebuilt the container backend, full Playwright suite **32/32 passed (2.0m)** on Grafana 13.0.1. Ad-hoc filters are not yet in the suite (gap #13), so this smoke-tests the shared macro/interpolation path; the filter behavior itself is fully covered by the unit tests above.
