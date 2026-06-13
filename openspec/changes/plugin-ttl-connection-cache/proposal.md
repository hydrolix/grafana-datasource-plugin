## Why

The fork's `Connector` (at `0f83082`) backs its per-user connection pool with `jellydator/ttlcache/v3`: a one-hour TTL, `OnEviction` calling `db.Close()`, and a bootstrap entry stored with `ttlcache.NoTTL` so it lives as long as the datasource. After C2 pins the plugin to sqlds at `ef925e1`, that machinery has to land in the plugin — the fork no longer carries it.

`sqlds.ConnectionCache` (added at `ef925e1`) is the contract: `Load` / `Store` / `Range` / `Dispose`, plus `SQLDatasource.ConnectionCacheFactory` for per-instance wiring. The default `NewSyncMapCache()` upstream is a leak-prone choice for this plugin — every distinct `(uid, OAuth-token)` pair would accumulate forever as users log in and out, with no eviction and no `Close()` on stale `*sql.DB`. The plugin needs the TTL+eviction behaviour the fork carried.

This change implements `TTLConnectionCache` against `sqlds.ConnectionCache`, preserves the fork's TTL semantics byte-for-byte (1-hour TTL, `OnEviction → Close()`, bootstrap key gets `NoTTL`), and wires it via `ConnectionCacheFactory` on the `HdxSqlDatasource` wrapper from C2. The OAuth-token-keyed pooling logic that *populates* this cache is C4 (`plugin-oauth-keyed-pooling`); this change covers the cache mechanics only.

## What Changes

- Add `pkg/plugin/connection_cache.go` defining `TTLConnectionCache` satisfying `sqlds.ConnectionCache` via `jellydator/ttlcache/v3`. One-hour default TTL. `OnEviction` calls `item.Value().Close()` (the `Close()` method on `sqlds.CachedConnection`).
- `Store(key, v)` detects the bootstrap key (`<uid>-default`) and stores with `ttlcache.NoTTL` so it survives past TTL; all other entries use the default TTL.
- `Dispose()` stops the background sweep goroutine and calls `Close()` on every live entry (per the `sqlds.ConnectionCache` `Dispose` contract).
- `Range(f)` iterates live entries; iteration order is implementation-defined (per the contract).
- Update `pkg/plugin/hdx_sqlds.go` (from C2) to wire `ds.ConnectionCacheFactory = func() sqlds.ConnectionCache { return NewTTLConnectionCache(settings.UID, time.Hour) }` after `sqlds.NewDatasource(driver)` and before the wrapper return. The constructor signature grows a `settings backend.DataSourceInstanceSettings` parameter — required for `settings.UID` to close over in the factory.
- `go.mod` promotes `github.com/jellydator/ttlcache/v3` from indirect to direct (it was transitive through the fork at `v5.0.1`; from `ef925e1` onward, sqlds itself does not import it).
- Go unit-test coverage: `Store`/`Load` round-trip preserves value identity; TTL eviction triggers `Close()` on the cached connection; bootstrap key (`<uid>-default`) stored with `NoTTL` survives past TTL; `Range` iterates the live entries; `Dispose` shuts the cache goroutine down and closes every live entry; concurrent `Load`/`Store` from many goroutines is race-free under `-race`.
- Playwright e2e coverage unchanged (no user-visible behaviour difference when combined with C4-C7).

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics.

## Capabilities

### New Capabilities

- `hdx-ttl-connection-cache`: Plugin-local `sqlds.ConnectionCache` implementation with 1-hour TTL, on-eviction `Close()`, and `NoTTL` for the bootstrap entry. Wired per `HdxSqlDatasource` instance via `ConnectionCacheFactory`.

### Modified Capabilities

- `hdx-sqlds-wrapper`: extended so `NewHdxSqlDatasource` wires `ds.ConnectionCacheFactory` from the instance constructor. The wrapper grows a `settings` parameter on its constructor.

## Impact

- **Frontend**: none.
- **Backend (Go)**: new files `pkg/plugin/connection_cache.go`, `pkg/plugin/connection_cache_test.go`; `pkg/plugin/hdx_sqlds.go` updated to wire `ConnectionCacheFactory` and accept `settings`; `pkg/plugin/datasource.go` updated to pass `settings` through.
- **Tests**: new Go unit tests for the cache (TTL eviction, bootstrap exemption, `Range`, `Dispose`, race safety). No e2e impact in isolation.
- **Dependencies**: `github.com/jellydator/ttlcache/v3` becomes a direct dependency; no new third-party packages.
- **User-visible**: none. Existing OAuth-only deployments retain their per-user TTL eviction; existing non-OAuth deployments retain their persistent bootstrap connection.
- **Security**: no surface change. Cache eviction continues to close `*sql.DB` (releases file descriptors / network connections to upstream Hydrolix).
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2). Independent of C4-C7, but ships in the same merge window as the rest of the migration sequence per C2's coordinated-shipment note.
