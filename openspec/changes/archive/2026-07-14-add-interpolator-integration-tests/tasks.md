## 1. Interpolator integration tests (pkg/plugin)

- [x] 1.1 Add `pkg/plugin/interpolator_integration_test.go` with a helper that builds a real interpolator: `NewHdxInterpolator(p, Macros)` where `p` is a `MetadataProvider` over `nopMetadataDS{}` with pre-seeded caches; drive it via `interpolate(ctx, *models.HdxQuery)` (or the public `Interpolate`).
- [x] 1.2 Test: `$__adHocFilter()` + `$__timeFilter(ts)` in one query — preseed keyCache for the FROM, fixed TimeRange; assert the output contains the filter condition and the time comparison on `ts`.
- [x] 1.3 Test: `$__adHocFilter()` over `WITH x AS (SELECT … ) … FROM x` — derive the resolved key via `cte.GetMacroCTEs`, preseed keyCache under it; assert the filter condition is built (proves resolve-adhoc-with-cte + dispatch together).
- [x] 1.4 Test: time macro with no column arg — preseed `pkCache` for the FROM table; assert the output references the cached PK column.
- [x] 1.5 Test: unknown macro left in place; and escaped `$$__<macro>` → `$__<macro>` not dispatched.
- [x] 1.6 Injection cases through the real pipeline: injected operator rejected (error, no rewrite); injected value stays quoted/escaped; injected Map subscript stays backtick-quoted + escaped.

## 2. Route integration test (pkg/api)

- [x] 2.1 Add a `/interpolate` test that injects a real `HdxInterpolator.Interpolate` (stubbed metadata layer, pre-seeded caches) into the handler instead of the existing `stubInterpolator`.
- [x] 2.2 POST a macro-bearing query and assert the response body is the SQL produced by the real macro pipeline.
- [x] 2.3 POST a query whose ad-hoc filter carries an injected operator and assert the route returns `error: true` (injection not present in the body).

## 3. Verification

- [x] 3.1 Run `go vet ./...`, `golangci-lint run`, and `go test -race ./...` — all green; confirm `nopMetadataDS` never panics (proves every metadata lookup hits a pre-seeded cache key).
- [x] 3.2 Confirm no non-test files changed (`git diff --stat` shows only `_test.go` additions).
