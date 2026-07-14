## Context

After C2-C7:

- `pkg/plugin/` owns every Hydrolix-specific behaviour previously inside the
  fork — interpolator, macros, metadata provider, connection cache, OAuth
  keying, data shapes.
- The plugin imports `github.com/grafana/sqlds/v5` throughout. The fork is
  consumed **only** through a `replace` directive in `go.mod`; there are no
  `hydrolix/sqlds` import lines (the historical references in `macros_registry.go`,
  `interpolator.go`, and `cte/cte.go` are attribution comments, not imports).
- The fork at `ef925e1` contains only upstream-equivalent types plus the two
  extension surfaces — no Hydrolix code remains.

Upstream `grafana/sqlds` released `v5.2.0` (also `v5.3.0`, see D6) carrying both
extension surfaces. Because the import path already points at upstream, the
module move is a `replace`-drop + version-pin, not an import rewrite.

## Goals / Non-Goals

**Goals:**
- Move the plugin's sqlds resolution from the fork (`replace` → `hydrolix/sqlds@ef925e1`)
  to upstream `grafana/sqlds@v5.2.0`.
- Adapt `TTLConnectionCache` to upstream's concrete `CachedConnection` value
  type with no behavioural change to the cache's observable semantics.
- Archive the fork repository so no future drift can occur.

**Non-Goals:**
- Import-path rewriting. Not needed — the path is already `github.com/grafana/sqlds/v5`.
- Adopting new upstream features shipped alongside the extensions. If `v5.2.0`
  carries improvements (better error wrapping, extra extension points), the
  plugin adopts them in follow-up changes, not here.
- Touching the interpolator or `/interpolate` handler. Verified no-op (D3).
- Jumping to `v5.3.0`. Deferred on the Go-version bump (D6).

## Decisions

### D1. Drop the `replace`, pin the upstream tag; no import rewrite

```
// before (go.mod)
require github.com/grafana/sqlds/v5 v5.0.0-00010101000000-000000000000
...
replace github.com/grafana/sqlds/v5 => github.com/hydrolix/sqlds/v5 v5.0.0-20260613103402-ef925e15e15e

// after
require github.com/grafana/sqlds/v5 v5.2.0
// (replace directive removed)
```

```bash
go mod edit -dropreplace=github.com/grafana/sqlds/v5
go mod edit -require=github.com/grafana/sqlds/v5@v5.2.0
go mod tidy
```

**Why no sed pass.** Every plugin import already reads `github.com/grafana/sqlds/v5`.
`grep -rn 'hydrolix/sqlds' pkg/` returns only comment lines. The original C8
scope assumed a fork-path import rewrite; that assumption is stale — the C2 pin
was implemented via `replace`, not via a `hydrolix/sqlds` require path.

**Why pin an exact tag.** Reproducibility; matches how other Grafana plugins
pin upstream sqlds. Version movement happens via explicit `go get` later.

### D2. Pre-merge verification — done, against a throwaway worktree

A worktree build with the `replace` dropped and `require v5.2.0` established the
real diff surface:

| Symbol | Fork `ef925e1` | Upstream `v5.2.0` | Impact |
|---|---|---|---|
| `Interpolator` | `func(ctx, *sqlutil.Query, json.RawMessage) (string, error)` field | identical | none |
| `ConnectionCacheFactory` | `func() ConnectionCache` field | identical | none |
| `ConnectionCache` | interface (`Load`/`Store`/`Range`/`Dispose`) | identical | none |
| `CachedConnection` | **interface** (nil-able, externally implementable) | **concrete struct**, unexported `db`/`settings`, value methods `DB()`/`Settings()`/`Close()` | **breaks `connection_cache.go` + test** |

`go build ./...` against `v5.2.0` failed in exactly one production file
(`connection_cache.go:55`, the `nil` return) and in `connection_cache_test.go`.
`interpolator.go`, `routes.go`, and `hdx_sqlds.go` compiled untouched.

### D3. Interpolator + `/interpolate`: verified no-op

`v5.2.0` exports `type Interpolator func(ctx, *sqlutil.Query, json.RawMessage) (string, error)`
and the `SQLDatasource.Interpolator` field — the exact shape
`adopt-sqlds-func-interpolator` moved the plugin to. The compile-time assertion
`var _ sqlds.Interpolator = (&HdxInterpolator{}).Interpolate` (interpolator.go:45)
and the nil-check handler in `routes.go` both compile and pass against upstream
with no edit. No spec delta for `hdx-interpolator` / `hdx-sqlds-wrapper`.

### D4. `CachedConnection` value-type adaptation

Upstream's `CachedConnection` is an opaque value struct constructed only inside
`package sqlds`. Two consequences:

1. **Production (forced, one line).** `TTLConnectionCache.Load` returns
   `sqlds.CachedConnection{}` (zero value) on a miss, not `nil`. Everything else
   — `Store`, `Range`, `Dispose`, the `ttlcache.Cache[string, sqlds.CachedConnection]`
   type param, `item.Value().Close()` — compiles unchanged; `Close()` is a
   value-receiver method on the struct.

2. **Tests (rewrite).** The old tests used `*stubCachedConn` implementing the
   fork's `CachedConnection` interface, asserted reference identity via
   `assert.Same`, and counted `Close()` calls on the stub. Under `v5.2.0`:
   - `stubCachedConn` cannot satisfy a struct type — deleted.
   - From `package plugin` only the zero value `sqlds.CachedConnection{}` is
     constructible (unexported fields; no exported constructor). Its `Close()`
     is a no-op (`db == nil`), so close side-effects are not observable on a
     stored value.
   - `CachedConnection` embeds `backend.DataSourceInstanceSettings`, which is
     not comparable — `got == v` will not compile. Identity assertions go away.

### D5. Test seam for close-observation (`closeConn`)

The current cache spec makes "closes every live entry exactly once" and the
no-double-close guarantee on `Dispose` load-bearing (the `unsubscribe` dance
exists precisely to avoid double-close). The opaque value type removes the old
observation path, so preserve coverage with a minimal seam:

```go
type TTLConnectionCache struct {
	inner        *ttlcache.Cache[string, sqlds.CachedConnection]
	bootstrapKey string
	unsubscribe  func()
	closeConn    func(sqlds.CachedConnection) error // default: sqlds.CachedConnection.Close
}
```

`OnEviction` and `Dispose` call `c.closeConn(item.Value())` instead of
`item.Value().Close()` directly.

`closeConn` is set at construction, not mutated after: `NewTTLConnectionCache`
delegates to an unexported `newTTLConnectionCache(uid, ttl, closeConn)` that
takes the closer as a parameter and passes `sqlds.CachedConnection.Close` (a
method expression). Tests call the unexported constructor with a counting
closer and store zero-value entries to assert eviction/dispose invoke it exactly
once per live entry.

**Why construction-time injection, not a settable field.** The `OnEviction`
callback reads `closeConn` from the sweep goroutine (`go cache.Start()`). Setting
the field *after* construction would be an unsynchronised write racing that read
— `go test -race` flags it. Capturing the closer before the `go` statement gives
the goroutine a happens-before edge to the value, so the counting-closer tests
stay race-free.

**Why a seam over dropping the coverage.** The double-close-avoidance is subtle
concurrency-adjacent behaviour that a unit test should pin; relegating it to
sqlds's own cache tests + e2e would silently regress confidence in the one
mechanism this cache adds over `NewSyncMapCache`. The seam is a single unexported
function field with a production default — negligible surface.

**Alternative considered.** Assert only presence/absence after TTL (no close
observation) and rely on sqlds + e2e for close semantics. Rejected — loses the
no-double-close guarantee at the unit level.

### D6. Target `v5.2.0`, not `v5.3.0`

`v5.3.0` exposes the identical `Interpolator` / `CachedConnection` /
`ConnectionCacheFactory` surface but its `go.mod` requires Go ≥ 1.26.4, forcing
a toolchain bump on the plugin. The extension surfaces this change needs are
fully present in `v5.2.0`. Take `v5.2.0`; a Go-version bump + `v5.3.0` is its own
change if wanted later.

### D7. Relationship to `adopt-sqlds-func-interpolator`

That sibling adapted the plugin to the func-typed interpolator and its code is
already committed (`a2d9d6f`) and compiles against `v5.2.0` (D3). Its three open
tasks are all "advance the *fork* `replace` pin to a revision carrying
`interpolator-func-field`, then verify" — moot once we resolve straight to
upstream. This change supersedes those tasks. Recommended follow-up: archive
`adopt-sqlds-func-interpolator` as superseded once this lands (tracked in
tasks §5), and update `sqlds-migration-plan`'s C8 section to record the
calendar gate satisfied and the connection-cache surface drift.

### D8. Archive the fork repository

After the plugin runs on upstream sqlds in production for ~one release cycle
(~2-4 weeks):

1. Add `DEPRECATED.md` to the fork root pointing at `grafana/sqlds`.
2. Mark the repository archived on GitHub (read-only).
3. Remove the fork from internal release tooling.

**Why wait.** Monthly deploy cadence; the first cycle keeps fork rollback as a
safety valve. **Why archive not delete.** Audits, blame archaeology, and the
catalog-finding trail need read access preserved.

## Risks / Trade-offs

- **[The value-type adaptation changes observable cache behaviour]** →
  Mitigation: the rewritten `connection_cache_test.go` re-proves TTL eviction,
  bootstrap `NoTTL` survival, `Range` early-exit, and single close on
  eviction/dispose (via the `closeConn` seam) under `go test -race`. Behaviour
  parity is the pass condition.
- **[`go mod tidy` pulls an unintended transitive-dep upgrade]** → Mitigation:
  review the `go.sum` diff; a major bump in any transitive dep triggers a
  side-investigation before merge.
- **[A fork consumer other than this plugin exists]** → Mitigation: verified
  before archive via `gh search code` + internal-tools audit. If one surfaces it
  gets its own migration before the archive step.
- **[Reverting after archive is harder than before]** → Acceptable: the archive
  step is gated on the 2-4 week stabilisation window (D8). Pre-archive revert is
  a `go.mod` revert; post-archive revert is a one-click GitHub unarchive + PR
  revert.

## Migration Plan

- **Forward**:
  1. `go mod edit -dropreplace=…`, `go mod edit -require=…@v5.2.0`, `go mod tidy`.
  2. Fix `connection_cache.go` (`Load` zero-value return; add `closeConn` seam;
     route eviction/dispose through it).
  3. Rewrite `connection_cache_test.go` for the value type + seam.
  4. Quality gates in the dev container: `go vet ./...`, `golangci-lint run`,
     `go test -race ./...`.
  5. `npm run build` + Playwright e2e via `grafana-plugin-e2e` — connection
     pooling and query paths must pass.
  6. Merge to `develop`; cut a release on the normal cadence.
  7. After 2-4 weeks stable, archive the fork (D8).
- **Rollback (before archive)**: revert the PR — `go.mod` restores the `replace`
  pin; cache reverts to the interface-typed implementation.
- **Rollback (after archive)**: unarchive the fork on GitHub, revert the PR,
  redeploy.

## Open Questions

- Bundle unrelated dependency refreshes here? No — this change is the sqlds move
  plus its forced cache adaptation only.
- Update `README.md` / internal runbooks that name the fork? Yes, in the same PR
  (small doc-hygiene addition), listed in tasks §4.
