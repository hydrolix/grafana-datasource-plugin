## Context

`sqlds.ConnectionCache` at `ef925e1` (added in the same revision the plugin's C2 pins to) is the contract for the per-`Connector` cache of `(uid, ConnectionArgs hash)`-keyed `*sql.DB` instances. The interface is small — `Load`, `Store`, `Range`, `Dispose` — with a strict no-wrapping contract on `Load`: the value returned must be the exact reference passed to `Store`, because sqlds-internal code type-asserts it back to the concrete `dbConnection` struct for field access.

The default `NewSyncMapCache()` upstream is a never-evicting `sync.Map` wrapper. Plugins that fan connections out per-user (this one does — every distinct OAuth token gets its own entry) need eviction to avoid unbounded growth, and need `db.Close()` on eviction to release sockets to upstream Hydrolix.

The fork's `Connector` at `0f83082` (and at the plugin's current pin `v5.0.1`) implements that behaviour inline: `ttlcache.New[string, dbConnection](ttlcache.WithTTL[string, dbConnection](time.Hour))`, `OnEviction(func(_, _, item) { _ = item.Value().db.Close() })`, bootstrap entry stored with `ttlcache.NoTTL`. With C2 pinning to `ef925e1`, none of that is left in the fork; the plugin owns it.

`sqlds.NewConnector` at `ef925e1` calls `driver.Connect(ctx, settings, nil)` once at construction and stores the result under `defaultKey(uid)` (returns `<uid>-default`). Subsequent calls to `Store` arrive from sqlds's internal `storeDBConnection` as queries open new per-user connections (C4's territory). The cache's `Store` is the only place to distinguish bootstrap-vs-per-user.

## Goals / Non-Goals

**Goals:**
- Implement `sqlds.ConnectionCache` with 1-hour TTL, on-eviction `Close()`, and `NoTTL` for the bootstrap entry — byte-for-byte parity with the fork's `connector.go` TTL behaviour.
- Wire the cache via `SQLDatasource.ConnectionCacheFactory` on the `HdxSqlDatasource` wrapper. Each datasource instance gets its own cache, scoped to the instance's `uid`.
- Honour the no-wrapping contract on `Load` — return the exact `sqlds.CachedConnection` reference passed to `Store`.

**Non-Goals:**
- OAuth-token keying. Per-user entry creation lives in C4 (`Driver.MutateQueryData` writes `connectionArgs` so sqlds keys correctly).
- Configurable TTL. The fork hardcodes 1 hour; this change matches. A plugin setting to expose TTL is a future change if a deployment surfaces a need.
- Activity-weighted eviction (touch-to-extend TTL). The fork uses a fixed absolute TTL from insert; this change matches. `ttlcache` supports activity-weighted modes via `WithLoadAfterExpiration` etc., but adopting one is a separate decision.
- Replacing `ttlcache` with a hand-rolled sweep goroutine. `ttlcache` was already transitive through the fork; staying on the same library minimises behaviour drift.
- Per-user health-check semantics. `CheckHealth` continues to use the bootstrap entry exactly as the fork did; per-user health is out of scope and would need a different upstream hook.

## Decisions

### D1. `TTLConnectionCache` struct and constructor signature

```go
// pkg/plugin/connection_cache.go
type TTLConnectionCache struct {
    inner        *ttlcache.Cache[string, sqlds.CachedConnection]
    bootstrapKey string
}

func NewTTLConnectionCache(uid string, ttl time.Duration) sqlds.ConnectionCache {
    cache := ttlcache.New[string, sqlds.CachedConnection](
        ttlcache.WithTTL[string, sqlds.CachedConnection](ttl),
    )
    cache.OnEviction(func(_ context.Context, _ ttlcache.EvictionReason, item *ttlcache.Item[string, sqlds.CachedConnection]) {
        _ = item.Value().Close()
    })
    go cache.Start()
    return &TTLConnectionCache{
        inner:        cache,
        bootstrapKey: uid + "-default",
    }
}
```

**Why `*ttlcache.Cache[string, sqlds.CachedConnection]`** (interface as the value type, not a concrete struct): satisfies the no-wrapping contract. `ttlcache`'s `item.Value()` returns the original interface value stored in `Set`, no copy or decoration. A `Load` therefore returns exactly what `Store` was handed.

**Why `(uid, ttl)` as constructor args, not a config struct.** Two args, both required. A struct with two fields is more ceremony than the call site needs. A future configurable TTL would still pass a `time.Duration`.

**Why store `bootstrapKey` as a field** rather than computing on every `Store`: the key is fixed for the cache's lifetime; pre-computing saves an allocation per `Store` (the hot path is per-query).

### D2. Bootstrap-key detection via suffix on `Store`

```go
func (c *TTLConnectionCache) Store(key string, v sqlds.CachedConnection) {
    if key == c.bootstrapKey {
        c.inner.Set(key, v, ttlcache.NoTTL)
        return
    }
    c.inner.Set(key, v, ttlcache.DefaultTTL)
}
```

sqlds's `defaultKey(uid)` produces `<uid>-default`. The plugin's cache, holding the closed-over `uid`, knows the bootstrap key without a runtime call into sqlds.

**Why exact-match on the closed-over key.** sqlds's `defaultKeySuffix == "default"` is unexported. Hard-coding the suffix is the only way to detect it from outside the package. Exact-match (`key == c.bootstrapKey`) is safer than `strings.HasSuffix(key, "-default")` — a uid that contains the substring `-default` (unlikely but possible in a test scenario) would never wrongly trigger the exempt path.

**Why detect at `Store` rather than at `Load`.** TTL is set at insert time in `ttlcache`. `Load` is read-only; per-call TTL adjustments are not part of the library's surface.

**Why no log line on the special-cased path.** The bootstrap-entry path runs once per datasource instance at construction, not per query. Observability isn't worth a log line. If the special case stops triggering (e.g. sqlds renames the suffix), the integration test catches it.

### D3. `OnEviction` calls `Close()` on the `sqlds.CachedConnection`

```go
cache.OnEviction(func(_ context.Context, _ ttlcache.EvictionReason, item *ttlcache.Item[string, sqlds.CachedConnection]) {
    _ = item.Value().Close()
})
```

`sqlds.CachedConnection`'s `Close() error` method is documented to close the underlying `*sql.DB` and to be idempotent. The plugin discards the error — there's nothing the cache can do with it.

**Why discard the error.** `ttlcache`'s eviction callback runs in a sweep goroutine. There is no caller to surface the error to. Logging would add a logger dependency to the cache (which currently has none); the cost-benefit is poor for an idempotent close.

**Why not handle eviction reasons separately.** `ttlcache` distinguishes `EvictionReasonDeleted` (manual remove), `EvictionReasonExpired` (TTL), `EvictionReasonCapacityReached` (max-size). The plugin's behaviour is identical in all three: close the connection. Branching on reason would be code without behaviour.

### D4. `Range` iterates `inner.Items()`; iteration order is implementation-defined

```go
func (c *TTLConnectionCache) Range(f func(key string, v sqlds.CachedConnection) bool) {
    for key, item := range c.inner.Items() {
        if !f(key, item.Value()) {
            return
        }
    }
}
```

**Why use `inner.Items()`.** `ttlcache.Cache` does not expose an idiomatic Go `range` over a `map`-like surface, but `Items()` returns a snapshot map. The snapshot is fine for the contract — `Range`'s iteration order is "implementation-defined", and "live entries" excludes anything evicted concurrently.

**Why short-circuit on `f` returning false.** The contract: "Iteration stops early if f returns false." Single-pass over the snapshot supports this trivially.

### D5. `Dispose` stops the sweep goroutine and closes every live entry

```go
func (c *TTLConnectionCache) Dispose() {
    c.inner.Stop()           // halts the sweep goroutine
    if c.unsubscribe != nil {
        c.unsubscribe()      // detach OnEviction so DeleteAll can't double-close
        c.unsubscribe = nil
    }
    for _, item := range c.inner.Items() {
        _ = item.Value().Close()
    }
    c.inner.DeleteAll()
}
```

`ttlcache.Cache.OnEviction(fn)` returns an unsubscribe function — passing `nil` to `OnEviction` panics. The implementation captures the returned unsubscribe at construction and invokes it during `Dispose`. The returned function blocks until any in-flight async callback invocations complete, so by the time the explicit close loop runs, no eviction-driven `Close` is racing the explicit `Close`.

The order matters: stop the sweep first so it doesn't race with the close loop; detach the eviction callback so `DeleteAll`'s `EvictionReasonDeleted` callback doesn't double-close; close every entry synchronously; then clear the map.

**Why explicit close in `Dispose` rather than relying on `DeleteAll` triggering eviction callbacks.** Two reasons. First, `ttlcache.Stop()` doesn't drain the eviction queue, so closes could be missed if the sweep goroutine had pending evictions. Second, the eviction-callback path is the *background* close path (each callback runs in its own goroutine spawned by `ttlcache`); `Dispose` is a *foreground* close path where the caller wants every connection closed before the call returns. Explicit close loop guarantees the latter.

**Why not just close-and-clear without `Stop`/`unsubscribe`.** The sweep goroutine continues running after the loop and would interact with already-closed connections on subsequent ticks. `Stop` ensures the goroutine exits cleanly; `unsubscribe` ensures `DeleteAll` doesn't spawn async closes on already-closed entries.

### D6. Constructor takes `settings.UID`, not `*HdxSqlDatasource`

The factory closure on `SQLDatasource.ConnectionCacheFactory` runs *inside* sqlds's `ds.NewDatasource(ctx, settings)`, which calls `NewConnector(..., WithCache(factory()))`. The factory has access to whatever the plugin closes over — most simply, the `settings.UID` captured by `NewHdxSqlDatasource(driver, settings)`.

```go
// pkg/plugin/hdx_sqlds.go
func NewHdxSqlDatasource(driver sqlds.Driver, settings backend.DataSourceInstanceSettings) *HdxSqlDatasource {
    ds := sqlds.NewDatasource(driver)
    ds.EnableMultipleConnections = true
    ds.ConnectionCacheFactory = func() sqlds.ConnectionCache {
        return NewTTLConnectionCache(settings.UID, time.Hour)
    }
    return &HdxSqlDatasource{SQLDatasource: ds}
}
```

**Why pass `settings.UID` and not the whole `*HdxSqlDatasource`.** The cache shouldn't hold a back-reference to its owning datasource. It needs the UID and nothing else.

**Why a factory closure rather than a single shared cache.** `sqlds.NewDatasource` may be called multiple times across the plugin's lifetime (e.g., on settings reconfiguration); each call should get a fresh cache. A factory closure is the natural shape.

### D7. TTL hardcoded to 1 hour; not exposed via plugin settings

The fork hardcodes `time.Hour`. This change matches. Adding a plugin setting (e.g., `connectionCacheTTL`) is a separate concern that would need a UI affordance, default rendering, schema migration in `models.PluginSettings`, and a clear operator story for why anyone would change it.

**Why one hour.** Long enough that a user holding a dashboard open for an entire session keeps their connection warm; short enough that abandoned per-user entries clear out within a working day. The fork's choice; no deployment has surfaced a reason to change.

**Alternative considered**: expose via `PluginSettings.ConnectionCacheTTL` defaulting to "1h". Rejected because it's a knob without a known need. Re-evaluate if a deployment surfaces TTL tuning as a problem.

## Risks / Trade-offs

- **[Bootstrap-key suffix `"default"` drifts in a future sqlds release]** → Mitigation: unit test asserts `Store("<uid>-default", v)` results in `NoTTL` storage; renames upstream fail this test loudly. Long-term: petition sqlds to expose `defaultKey(uid)` (or an `IsDefaultKey(key, uid) bool` helper); not a blocker today.
- **[`ttlcache`'s sweep goroutine leaks if `Dispose` is not called]** → Mitigation: sqlds's `Connector.Dispose` is invoked from `SQLDatasource.Dispose`, which the Grafana SDK invokes on instance teardown. The plugin's wrapper's `Dispose` promotes through embedding. Test: `Dispose()` invocation closes every live entry and halts the goroutine (assertable via `runtime.NumGoroutine` before/after).
- **[No-wrapping contract violated by future code change]** → Mitigation: unit test stores a custom `sqlds.CachedConnection` mock and asserts `Load` returns the same Go pointer via `==` reference equality. Any wrapping introduced later fails this test.
- **[Concurrent `Store`+`Range` race]** → Mitigation: `ttlcache.Cache` is documented as concurrent-safe; the plugin doesn't add cross-method state. Run unit tests with `-race`.
- **[1-hour TTL evicts an actively-used connection mid-query]** → Mitigation: `ttlcache.Cache.Get` defaults to touch-on-hit (`item.touch()` resets the expiration timestamp), so a hot user's connection stays warm as long as queries keep arriving. Idle entries expire only after a full TTL of no `Load` activity. The underlying `*sql.DB` is held by any in-flight query during eviction-callback close, so close happens *after* the query returns the connection to the pool. Matches the fork's production behaviour (the fork inherited the same touch-on-hit default).
- **[`*sql.DB.Close()` on eviction blocks while in-flight queries drain]** → Acceptable: the close is on the sweep goroutine, doesn't block any query path, and the drain happens once per evicted DB instance. The "1-hour cache" effectively functions as a 1-hour-plus-drain-time cache, which matches the fork.

## Migration Plan

- **Forward**: ships in the C2-C7 coordinated merge window. Sequence inside its PR commit (or PR if stacked):
  1. Add `pkg/plugin/connection_cache.go` + `pkg/plugin/connection_cache_test.go`.
  2. Update `pkg/plugin/hdx_sqlds.go` (from C2) to take `settings backend.DataSourceInstanceSettings` and wire `ConnectionCacheFactory`.
  3. Update `pkg/plugin/datasource.go` (from C2) to pass `settings` to `NewHdxSqlDatasource`.
  4. `go mod tidy` (promotes `ttlcache/v3` to direct).
  5. Run quality gates: `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`. All must pass — including the new cache tests.
  6. E2E via `grafana-plugin-e2e`: deferred until C4-C7 also land (in isolation, no Hydrolix query path is functional).
- **Rollback**: revert this change's commit/PR. The wrapper falls back to `ds.ConnectionCacheFactory == nil`, which sqlds resolves to `NewSyncMapCache()` — the upstream default. Behaviour drifts (no eviction, no `Close()` on stale entries), but the plugin still builds.
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2). Ships in the same window as C4-C7.

## Open Questions

- Should `Dispose` time-bound the foreground-close loop (e.g., 5-second `*sql.DB.Close()` deadline per entry) to prevent a single slow upstream from blocking datasource teardown? The fork doesn't; the SDK doesn't enforce a deadline either. Defer unless a deployment surfaces hang-on-shutdown behaviour.
- Should the cache log eviction reasons at debug level for observability? The fork doesn't log; the plugin's current logger surface is minimal (`pkg/`'s structured logger). Defer until observability requirements surface — the cache layer is well-contained and rarely the diagnostic target.
- Should `NewTTLConnectionCache` accept an explicit `*ttlcache.Cache` for tests (so tests can drive `Stop`/`Start` directly)? Current shape uses the constructor-owned cache; tests work via wall-clock or the `ttlcache.Set(key, v, shortTTL)` shortcut. Re-evaluate only if a test case proves awkward.
