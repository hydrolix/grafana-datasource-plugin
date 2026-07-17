# hdx-ttl-connection-cache

## ADDED Requirements

### Requirement: `TTLConnectionCache` satisfies `sqlds.ConnectionCache`

The plugin SHALL define `TTLConnectionCache` in `pkg/plugin/connection_cache.go` that satisfies the `sqlds.ConnectionCache` interface (`Load`, `Store`, `Range`, `Dispose`). The implementation SHALL be safe for concurrent use from any number of goroutines.

#### Scenario: Interface conformance

- **GIVEN** the C3 plugin build
- **WHEN** `var _ sqlds.ConnectionCache = (*TTLConnectionCache)(nil)` is compiled
- **THEN** the assertion SHALL compile without error

#### Scenario: Concurrent access is race-free

- **GIVEN** a single `*TTLConnectionCache` shared across many goroutines
- **WHEN** the goroutines call `Store` / `Load` / `Range` concurrently
- **THEN** the cache SHALL not produce a data race under `go test -race`

### Requirement: `NewTTLConnectionCache` constructor

The plugin SHALL define `NewTTLConnectionCache(uid string, ttl time.Duration) sqlds.ConnectionCache`. The returned value SHALL hold an internal `*ttlcache.Cache[string, sqlds.CachedConnection]` configured with the supplied `ttl` and an `OnEviction` callback that invokes `Close()` on the evicted `CachedConnection`. The constructor SHALL start the cache's sweep goroutine.

#### Scenario: Constructor returns a usable cache

- **GIVEN** `NewTTLConnectionCache("uid1", time.Hour)`
- **WHEN** the return value is used to `Store` and then `Load` an entry
- **THEN** the value SHALL round-trip successfully

### Requirement: `Load` returns the exact stored value (no wrapping)

`TTLConnectionCache.Load` SHALL return the exact `sqlds.CachedConnection` reference passed to `Store` for the same key. The implementation SHALL NOT wrap, copy, or decorate the value. sqlds-internal code type-asserts the returned value back to a concrete struct.

#### Scenario: Reference equality on round-trip

- **GIVEN** a custom `sqlds.CachedConnection` value `v` (pointer interface satisfaction)
- **WHEN** `cache.Store(k, v)` then `got, ok := cache.Load(k)` are invoked
- **THEN** `ok` SHALL be `true`
- **AND** `got == v` SHALL be `true` (Go interface identity)

#### Scenario: Miss returns nil, false

- **GIVEN** a fresh cache
- **WHEN** `cache.Load("absent")` is invoked
- **THEN** the return SHALL be `(nil, false)`

### Requirement: Bootstrap key is stored with `NoTTL`

`TTLConnectionCache.Store` SHALL detect the bootstrap key (`<uid>-default`, the value `sqlds.defaultKey(uid)` produces) and store it with `ttlcache.NoTTL`. Every other key SHALL be stored with the cache's default TTL.

#### Scenario: Bootstrap entry survives past the configured TTL

- **GIVEN** a cache configured with a 50ms TTL and `uid = "u"`
- **WHEN** `Store("u-default", v)` is called, then 200ms elapses
- **THEN** `Load("u-default")` SHALL return `(v, true)`

#### Scenario: Non-bootstrap entry expires after the configured TTL

- **GIVEN** a cache configured with a 50ms TTL and `uid = "u"`
- **WHEN** `Store("u-otherkey", v)` is called, then 200ms elapses
- **THEN** `Load("u-otherkey")` SHALL return `(nil, false)`
- **AND** the `Close()` method on `v` SHALL have been invoked exactly once via the `OnEviction` callback

### Requirement: `Range` iterates live entries and supports early termination

`TTLConnectionCache.Range(f)` SHALL invoke `f` for every live entry. Iteration order is implementation-defined. Iteration SHALL stop early if `f` returns `false`.

#### Scenario: Range visits every live entry when f always returns true

- **GIVEN** a cache with three stored entries
- **WHEN** `Range(func(k, v) bool { count++; return true })` runs
- **THEN** `count` SHALL be `3`

#### Scenario: Range stops when f returns false

- **GIVEN** a cache with three stored entries
- **WHEN** `Range(func(k, v) bool { count++; return false })` runs
- **THEN** `count` SHALL be `1`

### Requirement: `Dispose` stops the sweep goroutine and closes every live entry

`TTLConnectionCache.Dispose` SHALL stop the cache's sweep goroutine, detach the `OnEviction` callback so the explicit close loop is the sole closer, call `Close()` on every live entry, and clear the underlying map. After `Dispose` returns, `Load` SHALL return `(nil, false)` for every key, and the sweep goroutine SHALL no longer be running.

#### Scenario: Dispose closes every live entry exactly once

- **GIVEN** a cache with three stored entries
- **WHEN** `Dispose()` is called
- **THEN** each stored value's `Close()` SHALL have been invoked exactly once

#### Scenario: Dispose terminates the sweep goroutine

- **GIVEN** a cache constructed via `NewTTLConnectionCache`
- **WHEN** `Dispose()` is invoked
- **THEN** the cache's internal sweep goroutine SHALL be terminated (no goroutine leak detectable via `runtime.NumGoroutine` ±tolerance)

### Requirement: `NewHdxSqlDatasource` wires per-instance `ConnectionCacheFactory`

The wrapper constructor `NewHdxSqlDatasource` SHALL take a `settings backend.DataSourceInstanceSettings` parameter and SHALL set `ds.ConnectionCacheFactory` to a closure that returns `NewTTLConnectionCache(settings.UID, time.Hour)` per invocation. The factory closure SHALL produce a fresh cache on each call so reconfiguration paths (e.g. settings updates) get an isolated cache.

#### Scenario: Factory yields a fresh cache per invocation

- **GIVEN** a `*HdxSqlDatasource` constructed with settings whose `UID == "u1"`
- **WHEN** `ds.ConnectionCacheFactory()` is invoked twice
- **THEN** the two returned values SHALL be distinct Go references
- **AND** each SHALL be a `*TTLConnectionCache` keyed off `"u1-default"`
