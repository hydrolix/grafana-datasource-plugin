# Tasks — swap to grafana/sqlds@v5.2.0 (retire hydrolix/sqlds fork)

## 1. Module move

- [x] 1.1 Remove the fork `replace` directive and its explanatory comment block (`go.mod` ~166-170). _(Edited `go.mod` directly rather than via `go mod edit -dropreplace`.)_
- [x] 1.2 Pin `require github.com/grafana/sqlds/v5 v5.2.0` (replaced the `v5.0.0-00010101000000-000000000000` pseudo-version).
- [x] 1.3 `go mod tidy`; reviewed the `go.sum` diff. `github.com/hydrolix/sqlds/v5` is gone; only patch/minor transitive bumps (grafana-plugin-sdk-go 0.292.0→0.292.1, arrow-go 18.5.2→18.6.0, etc.), no major bumps. Go directive unchanged at 1.26.3 (v5.2.0 forces no toolchain bump; v5.3.0 would — not taken).
- [x] 1.4 `grep -rn 'hydrolix/sqlds' pkg/` returns only attribution comments in `macros_registry.go`, `interpolator.go`, `cte/cte.go` — no import lines.

## 2. Connection-cache adaptation (`pkg/plugin/connection_cache.go`)

- [x] 2.1 `Load` returns `sqlds.CachedConnection{}` (zero value) on a miss instead of `nil`.
- [x] 2.2 Added unexported `closeConn func(sqlds.CachedConnection) error` field; initialised to `sqlds.CachedConnection.Close`. Injected via an unexported `newTTLConnectionCache(uid, ttl, closeConn)` constructor (the exported `NewTTLConnectionCache` delegates with the production closer) so the seam is set before `go cache.Start()` — race-free under `-race`.
- [x] 2.3 `OnEviction` and the `Dispose` close loop route through `c.closeConn(item.Value())`.
- [x] 2.4 Updated the `TTLConnectionCache` doc comment for the concrete value type (the fork's "exact reference / type-assert" wording is gone).

## 3. Tests (`pkg/plugin/connection_cache_test.go`)

- [x] 3.1 Deleted `stubCachedConn` and its methods; replaced with a `closeCounter` test double for the seam.
- [x] 3.2 Round-trip test stores `sqlds.CachedConnection{}` and asserts `ok == true`; `assert.Same` reference-identity assertion dropped.
- [x] 3.3 Miss test asserts `(sqlds.CachedConnection{}, false)`.
- [x] 3.4 Eviction test (bootstrap `NoTTL` survives; non-bootstrap expires) injects a counting closer at construction (`newCountingCache`) and asserts it fires exactly once on eviction.
- [x] 3.5 `Dispose` test asserts the counter fires exactly three times (once per entry) and does not increment again after a post-dispose beat (no double-close).
- [x] 3.6 Kept the `Range` early-exit and goroutine-leak tests; entry construction switched to zero values.
- [x] 3.7 `go test -race ./pkg/plugin/` green (cache tests + full package).

## 4. Repo hygiene + quality gates

- [x] 4.1 No `README.md` / doc references to `github.com/hydrolix/sqlds` exist — no-op.
- [x] 4.2 `go vet ./...` clean. `golangci-lint run` reports zero issues in the touched files (`connection_cache.go`, `connection_cache_test.go`); the 6 findings are pre-existing in untouched files (`driver.go`, `routes.go`, `datasource.go`, `driver_conv_test.go`).
- [x] 4.3 `go test -race ./...` green (all 6 packages).
- [x] 4.4 `npm run build` clean `dist/` (3 pre-existing bundle-size warnings, no errors).
- [x] 4.5 Playwright e2e via the `grafana-plugin-e2e` skill — 32/32 passed (2.2m, Grafana 10.4.16, 4 workers) against the freshly-rebuilt v5.2.0-linked plugin binary. Grafana restarted post-rebuild so the running plugin subprocess was the v5.2.0 build, not the stale startup binary.

## 5. Sibling-change reconciliation

- [x] 5.1 `adopt-sqlds-func-interpolator` §4 annotated superseded (banner + struck 4.1; 4.2/4.3 re-pointed to this change's gates); proposal `go.mod` bullet marked superseded. Not archived yet — archives with the migration sequence after the swap lands.
- [x] 5.2 `sqlds-migration-plan` C8 section updated: calendar gate marked satisfied, v5.2.0 pinned, the `CachedConnection` interface→struct drift + non-no-op nature recorded in both `tasks.md` §10 and `design.md`.

## 6. Fork archival (post-merge, after stabilisation window)

- [ ] 6.1 Confirm no other consumer of `github.com/hydrolix/sqlds` (`gh search code` + internal-tools audit). _Post-merge._
- [ ] 6.2 After ~2-4 weeks of stable production on upstream sqlds: add `DEPRECATED.md` to the fork root, mark the repo archived (read-only) on GitHub, remove it from internal release tooling. _Post-merge._
