## Why

C2 pins the plugin to the `hydrolix/sqlds` fork via a `replace` directive: the
require path is already `github.com/grafana/sqlds/v5`, redirected to
`github.com/hydrolix/sqlds/v5` at the extension-points-only revision `ef925e1`.
At that revision the fork is functionally upstream `grafana/sqlds@5.1.1` plus
two extension surfaces the plugin consumes — the func-typed `Interpolator`
field and the `ConnectionCache` interface + `ConnectionCacheFactory` field. The
fork carries no Hydrolix logic; that all lives in `pkg/plugin/` after C3-C7.

Upstream `grafana/sqlds` has now released **`v5.2.0`**, which contains both
extension surfaces. The calendar gate this change was always blocked on is
satisfied. The fork has no remaining reason to exist: drop the `replace`, pin
the real tag, and archive the fork repository.

The swap is **not** the pure no-op the original scope assumed. A pre-merge
build against `v5.2.0` (see `design.md` D2) shows the interpolator surface is
byte-identical to the fork, but the connection-cache value type diverged:
upstream reshaped `sqlds.CachedConnection` from the fork's **interface** into a
**concrete struct** with unexported fields. The plugin's `TTLConnectionCache`
needs a one-line production fix plus a test rewrite to match. That adaptation
rides in this change.

## What Changes

- `go.mod`: remove the `replace github.com/grafana/sqlds/v5 => github.com/hydrolix/sqlds/v5 …`
  directive and pin `require github.com/grafana/sqlds/v5 v5.2.0`
  (was the pseudo-version `v5.0.0-00010101000000-000000000000` resolved through
  the replace). `go.sum` regenerates via `go mod tidy`. **No import-path rewrite** —
  every plugin import already reads `github.com/grafana/sqlds/v5`; only the
  module resolution moves from fork to upstream.
- `pkg/plugin/connection_cache.go`: adapt `TTLConnectionCache` to the concrete
  `sqlds.CachedConnection` value type. `Load` returns `sqlds.CachedConnection{}`
  (the zero value) on a miss instead of `nil` — the only production change the
  compiler forces. Add a `closeConn func(sqlds.CachedConnection) error` seam
  (defaulting to `sqlds.CachedConnection.Close`) so eviction/dispose close
  behaviour stays unit-observable now that the opaque value type cannot be
  fabricated with a counting `Close()` from `package plugin`.
- `pkg/plugin/connection_cache_test.go`: rewrite. The `stubCachedConn`
  interface stub is dead (the value type's fields are unexported and
  unconstructable outside `package sqlds`). Round-trip tests store the zero
  value and assert presence/absence + TTL semantics; eviction/dispose
  close-count tests assert against the `closeConn` seam.
- `pkg/plugin/interpolator.go`, `pkg/api/routes.go`, `pkg/plugin/hdx_sqlds.go`:
  **unchanged** — verified to compile against `v5.2.0` as-is (func-typed
  `Interpolator`, `ConnectionCacheFactory` identical to the fork).
- Archive the `github.com/hydrolix/sqlds` fork repository (read-only,
  `DEPRECATED.md` pointing at upstream) after a stabilisation window.

**Not BREAKING** for the plugin's frontend, HTTP wire format, dashboards, or
query semantics. The connection-cache adaptation preserves observable cache
behaviour (TTL eviction, bootstrap `NoTTL`, foreground close on `Dispose`).

## Capabilities

### New Capabilities

<!-- None — this change closes out the fork's existence; no plugin capability is added. -->

### Modified Capabilities

- `hdx-ttl-connection-cache`: the `Load`-miss return value and the
  eviction/dispose close-observation contract are restated against upstream's
  concrete `sqlds.CachedConnection` value type (was the fork's interface). No
  behavioural change to TTL, bootstrap, or dispose semantics — only the type
  the cache traffics in and how tests observe `Close`.

<!-- hdx-interpolator / hdx-sqlds-wrapper: unchanged. The v5.2.0 Interpolator surface is identical to the fork's func-typed shape; verified by build, guarded by the existing compile-time assertion. No delta. -->

## Impact

- **Frontend**: none.
- **Backend (Go)**: `go.mod` (drop replace, pin `v5.2.0`), `go.sum` (tidy),
  `pkg/plugin/connection_cache.go` (+ its test). Three files plus module
  metadata. No import-path churn.
- **Tests**: `connection_cache_test.go` rewritten for the value type + seam. No
  other test surface moves.
- **Dependencies**: `github.com/hydrolix/sqlds/v5` removed; `github.com/grafana/sqlds/v5 v5.2.0`
  pinned directly. Transitive deps regenerate via `go mod tidy`. Target
  `v5.2.0`, **not** `v5.3.0` — the latter requires Go ≥ 1.26.4 (see design D6).
- **User-visible**: none.
- **Security**: closes the catalog-review finding. Every line of sqlds the
  plugin links against is now maintained, reviewed `grafana/sqlds`. The fork is
  archived; no consumer can land changes to it.
- **Sequencing**: depends on C2-C7 merged (plugin owns every Hydrolix behaviour
  locally) and on `adopt-sqlds-func-interpolator` — that sibling already lands
  the func-typed interpolator code in-tree (commit `a2d9d6f`) and its remaining
  "advance the fork pin" tasks are superseded here (design D5). Upstream release
  is now available; the change is no longer calendar-blocked.
