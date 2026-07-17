## 1. Dependency bump

- [x] 1.1 `go get github.com/hydrolix/clickhouse-sql-parser@v0.5.2` and run `go mod tidy`.
- [x] 1.2 Review the `go.sum` / transitive dependency delta; run `go mod verify`.

## 2. API migration

- [x] 2.1 Replace `expr.Table.String()` (`pkg/plugin/cte/cte.go:96`) with `parser.Format(expr.Table)`.
- [x] 2.2 Replace `expr.Database.String()` (`cte.go:99`) with `parser.Format(expr.Database)`.
- [x] 2.3 Replace `expr.From.Expr.String()` (`cte.go:120`) with `parser.Format(expr.From.Expr)`.
- [x] 2.4 Confirm `go build ./...` compiles — no remaining references to removed/renamed symbols (`Expr.String`, `DescribeQuery`, `UnionAll`, `UnionDistinct`, `ArrayJoin`).

## 3. Regression verification

- [x] 3.1 Run `go test -race ./pkg/plugin/cte/...` — CTE extraction scenarios pass with no assertion edits.
- [x] 3.2 Run `go test -race ./...` — interpolator and macro suites pass unchanged; if any assertion must change, treat it as a real behavior change and escalate rather than editing the expectation.
- [x] 3.3 Run `go vet ./...` and `golangci-lint run` — vet clean; `cte.go` is lint-clean (0 issues). Pre-existing lint warnings remain in untouched files (`build/`, `converters/`, `datasource.go`, `driver.go`) and are out of scope for this change.
- [x] 3.4 Backend binary rebuilt via `mage build:debug` (host + in-container linux/arm64); confirmed the running `gpx_plugin_linux_arm64` embeds `clickhouse-sql-parser v0.5.2` via `go version -m`. Full Playwright e2e suite green: **32/32 passed (1.9m)** on Grafana 13.0.1 — including all `macroFunctions` (CTE + PK metadata resolution), interpolation, template-variable substitution, and annotations.
