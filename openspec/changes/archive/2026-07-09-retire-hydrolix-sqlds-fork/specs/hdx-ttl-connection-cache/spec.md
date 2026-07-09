# hdx-ttl-connection-cache Specification (delta)

Adapts the connection-cache contract to upstream `grafana/sqlds@v5.2.0`, whose
`CachedConnection` is a concrete value struct (unexported fields, value-receiver
`DB()`/`Settings()`/`Close()`) rather than the fork's interface. Behaviour is
unchanged; the type the cache traffics in and the way tests observe `Close` are
restated. All other requirements in this capability are unchanged.

## ADDED Requirements

### Requirement: Close-observation seam for the opaque `CachedConnection` value

`TTLConnectionCache` SHALL route every connection close through an unexported
`closeConn func(sqlds.CachedConnection) error` field, initialised by
`NewTTLConnectionCache` to the method expression `sqlds.CachedConnection.Close`.
The eviction callback and `Dispose` SHALL invoke `closeConn` rather than calling
`Close()` on the value directly. Tests MAY override `closeConn` to observe close
invocations. This seam exists because upstream `sqlds.CachedConnection`
(`grafana/sqlds@v5.2.0`) is a value struct whose fields are unexported and
unconstructable outside `package sqlds`, so a stored entry's `Close()` cannot
otherwise be instrumented from `package plugin`.

#### Scenario: production default routes to `CachedConnection.Close`

- **GIVEN** a cache from `NewTTLConnectionCache` with no override
- **WHEN** an entry is evicted or the cache is disposed
- **THEN** `closeConn` SHALL be the `sqlds.CachedConnection.Close` method expression
- **AND** the underlying `*sql.DB` (if any) SHALL be closed exactly once

#### Scenario: tests observe close via the seam

- **GIVEN** a cache whose `closeConn` is overridden with a counting function
- **WHEN** a stored zero-value `sqlds.CachedConnection{}` entry is evicted
- **THEN** the counting function SHALL be invoked exactly once for that entry

## MODIFIED Requirements

### Requirement: `Load` returns the exact stored value (no wrapping)

`TTLConnectionCache.Load` SHALL return the `sqlds.CachedConnection` value stored
under the key with `ok == true`, and SHALL return `(sqlds.CachedConnection{}, false)`
for an absent key. `sqlds.CachedConnection` is a concrete value type (upstream
`grafana/sqlds@v5.2.0`); the cache SHALL store and return it by value without
wrapping, copying into a pointer, or decorating. `sqlds.CachedConnection`
embeds a non-comparable `backend.DataSourceInstanceSettings`, so equality of
returned values SHALL NOT be asserted with `==`.

#### Scenario: round-trip returns ok

- **GIVEN** a fresh cache
- **WHEN** `cache.Store(k, sqlds.CachedConnection{})` then `_, ok := cache.Load(k)` are invoked
- **THEN** `ok` SHALL be `true`

#### Scenario: miss returns the zero value, false

- **GIVEN** a fresh cache
- **WHEN** `cache.Load("absent")` is invoked
- **THEN** the return SHALL be `(sqlds.CachedConnection{}, false)`

### Requirement: `NewTTLConnectionCache` constructor

The plugin SHALL define `NewTTLConnectionCache(uid string, ttl time.Duration) sqlds.ConnectionCache`.
The returned value SHALL hold an internal `*ttlcache.Cache[string, sqlds.CachedConnection]`
configured with the supplied `ttl`, an `OnEviction` callback that invokes the
cache's `closeConn` seam on the evicted `CachedConnection`, and a `closeConn`
field initialised to `sqlds.CachedConnection.Close`. The constructor SHALL start
the cache's sweep goroutine.

#### Scenario: Constructor returns a usable cache

- **GIVEN** `NewTTLConnectionCache("uid1", time.Hour)`
- **WHEN** the return value is used to `Store` and then `Load` an entry
- **THEN** `Load` SHALL return `ok == true` for the stored key

### Requirement: Bootstrap key is stored with `NoTTL`

`TTLConnectionCache.Store` SHALL detect the bootstrap key (`<uid>-default`, the
value `sqlds.defaultKey(uid)` produces) and store it with `ttlcache.NoTTL`. Every
other key SHALL be stored with the cache's default TTL. On expiry of a
non-bootstrap entry the eviction callback SHALL route the evicted value through
`closeConn` exactly once.

#### Scenario: Bootstrap entry survives past the configured TTL

- **GIVEN** a cache configured with a 50ms TTL and `uid = "u"`
- **WHEN** `Store("u-default", sqlds.CachedConnection{})` is called, then 200ms elapses
- **THEN** `Load("u-default")` SHALL return `ok == true`

#### Scenario: Non-bootstrap entry expires after the configured TTL

- **GIVEN** a cache configured with a 50ms TTL, `uid = "u"`, and a counting `closeConn` override
- **WHEN** `Store("u-otherkey", sqlds.CachedConnection{})` is called, then 200ms elapses
- **THEN** `Load("u-otherkey")` SHALL return `(sqlds.CachedConnection{}, false)`
- **AND** the `closeConn` override SHALL have been invoked exactly once for that entry

### Requirement: `Dispose` stops the sweep goroutine and closes every live entry

`TTLConnectionCache.Dispose` SHALL stop the cache's sweep goroutine, detach the
`OnEviction` callback so the explicit close loop is the sole closer, invoke
`closeConn` on every live entry, and clear the underlying map. After `Dispose`
returns, `Load` SHALL return `(sqlds.CachedConnection{}, false)` for every key,
and the sweep goroutine SHALL no longer be running.

#### Scenario: Dispose closes every live entry exactly once

- **GIVEN** a cache with three stored entries and a counting `closeConn` override
- **WHEN** `Dispose()` is called
- **THEN** the `closeConn` override SHALL have been invoked exactly three times (once per entry)
- **AND** no entry SHALL be closed a second time by an async eviction callback

#### Scenario: Dispose terminates the sweep goroutine

- **GIVEN** a cache constructed via `NewTTLConnectionCache`
- **WHEN** `Dispose()` is invoked
- **THEN** the cache's internal sweep goroutine SHALL be terminated (no goroutine leak detectable via `runtime.NumGoroutine` ±tolerance)
