# plugin-ttl-connection-cache — implementation tasks

## 1. `TTLConnectionCache` implementation

- [x] 1.1 Add `pkg/plugin/connection_cache.go` defining `TTLConnectionCache` with fields `inner *ttlcache.Cache[string, sqlds.CachedConnection]` and `bootstrapKey string`.
- [x] 1.2 Add constructor `NewTTLConnectionCache(uid string, ttl time.Duration) sqlds.ConnectionCache`. Constructs the `ttlcache.Cache`, installs `OnEviction(... item.Value().Close())`, starts the sweep goroutine via `go cache.Start()`, returns `&TTLConnectionCache{inner: cache, bootstrapKey: uid + "-default"}`.
- [x] 1.3 Implement `Load(key) (sqlds.CachedConnection, bool)`: returns `(nil, false)` on miss; returns the exact stored value on hit (no wrapping).
- [x] 1.4 Implement `Store(key, v)`: if `key == c.bootstrapKey` use `ttlcache.NoTTL`; otherwise `ttlcache.DefaultTTL`.
- [x] 1.5 Implement `Range(f)`: iterate `c.inner.Items()`, short-circuit on `f` returning false.
- [x] 1.6 Implement `Dispose()`: `Stop()` the sweep, `OnEviction(nil)` to detach the callback (avoid double-close), explicit close loop, then `DeleteAll`.
- [x] 1.7 Compile-time assertion `var _ sqlds.ConnectionCache = (*TTLConnectionCache)(nil)`.

## 2. Wire `ConnectionCacheFactory` in the wrapper

- [x] 2.1 Update `pkg/plugin/hdx_sqlds.go`: `NewHdxSqlDatasource` grows a `settings backend.DataSourceInstanceSettings` parameter.
- [x] 2.2 Inside the constructor, after `sqlds.NewDatasource(driver)` and before the return, set `ds.ConnectionCacheFactory = func() sqlds.ConnectionCache { return NewTTLConnectionCache(settings.UID, time.Hour) }`.
- [x] 2.3 Update `pkg/plugin/datasource.go::NewDatasource` to pass the `settings` argument through: `NewHdxSqlDatasource(NewHydrolix(), settings)`.

## 3. Dependency management

- [x] 3.1 `go get github.com/jellydator/ttlcache/v3@latest`.
- [x] 3.2 `go mod tidy` — confirm `ttlcache/v3` is direct, not indirect.

## 4. Tests

- [x] 4.1 Add `pkg/plugin/connection_cache_test.go`.
- [x] 4.2 `TestStore_Load_RoundTrip`: store a `stubCachedConn`, `Load` returns the exact pointer (`==` reference equality).
- [x] 4.3 `TestStore_BootstrapKey_NoTTL`: store under `<uid>-default` with a short configured TTL; advance wall-clock past the TTL via a short cache TTL and `time.Sleep`; assert the entry is still loadable. (Use a short TTL like 50ms in the test, with explicit sleep guards.)
- [x] 4.4 `TestStore_PerUserKey_EvictsAndCloses`: store under any non-bootstrap key; wait past the configured TTL; assert `Load` misses and the stub's `Close()` was invoked exactly once.
- [x] 4.5 `TestRange_VisitsLiveEntries_AndShortCircuits`: store 3 entries; `Range` collects all 3 when `f` returns true; returning false after first stops iteration.
- [x] 4.6 `TestDispose_ClosesEverythingAndStopsSweep`: store several entries; call `Dispose`; assert each stub's `Close()` was invoked exactly once, `Load` returns miss, and the sweep goroutine count returns to baseline (use `runtime.NumGoroutine` snapshots ±tolerance).
- [x] 4.7 `TestConcurrent_LoadStoreRangeDispose_RaceFree`: spawn N goroutines doing mixed `Store`/`Load`/`Range`; assert no race under `-race` flag. (Use `go test -race`; no extra logic needed in the test beyond ensuring concurrent traffic.)
- [x] 4.8 `TestNoWrappingContract`: a custom `sqlds.CachedConnection` interface value stored and loaded back must be the same Go reference (catches accidental decoration).

## 5. Quality gates

- [x] 5.1 `go build ./...` — clean.
- [x] 5.2 `go vet ./...` — clean.
- [x] 5.3 `golangci-lint run --new-from-rev=HEAD~1` — no new issues vs C4.
- [x] 5.4 `go test -race ./...` — green.
- [x] 5.5 `npm run typecheck && npm run lint && npm run test:ci` — green (no frontend changes; regression-only check).
- [x] 5.6 Playwright e2e — deferred to coordinated-set verification.

## 6. Commit

- [x] 6.1 Single commit including code + design + tasks + specs + go.mod + go.sum.
- [x] 6.2 Commit message: `pkg/plugin: TTL connection cache wired via ConnectionCacheFactory (C3)`. Body summarises: 1h TTL, OnEviction → Close, bootstrap key gets NoTTL, wrapper now takes settings.
