## Context

The plugin imports `github.com/hydrolix/sqlds/v5 v5.0.1`. The fork at `v5.0.1` (and at the legacy branch tip `0f83082`) carries Hydrolix-specific runtime: `HydrolixDatasource` type, AST `Interpolator`, ad-hoc + ClickHouse macros, `MetadataProvider`, `Connector` with OAuth-keyed pooling, and TTL connection cache. The fork's tip at the extension-points branch (`ef925e1`, commit dated 2026-06-13) has had all of that removed; what remains is upstream `grafana/sqlds@6c09016` (release 5.1.1) plus exactly two extension surfaces:

- `Interpolator` interface + `SQLDatasource.Interpolator` field (a nil value falls back to `DefaultInterpolator{}` wrapping `sqlutil.Interpolate`).
- `ConnectionCache` interface + `SQLDatasource.ConnectionCacheFactory` field. `NewConnector` is variadic with `WithCache(ConnectionCache)` opting per-instance caches in. The default is a nil cache (no pooling).

`SQLDatasource.Register` / `Resolve` and `RegisterMacro` / `ContextMacroFunc` / `MacroContext`, which earlier drafts of this migration assumed, do *not* exist at `ef925e1` — they were retired in commit `80cd2b4` once the fork-retirement plan settled on the interpolator owning the full rewrite call. Plugin-side state (metadata provider, macro tables, AST caches) lives as plain Go fields on the plugin's wrapper or its interpolator, reached via Go references rather than a runtime registry.

Plugin call sites today reference `sqlds.HydrolixDatasource` (`pkg/plugin/datasource.go:17`, `pkg/api/routes.go:42,148`), `sqlds.NewConnector` (`pkg/plugin/datasource.go:13`), `sqlds.Driver` and the mutator interfaces (`pkg/plugin/driver.go:34-37`), `sqlds.DriverSettings` (`pkg/plugin/driver.go:233,241`), `sqlds.HeaderKey` (`pkg/plugin/driver.go:356-357`), `sqlds.GetMacroCTEs` and `sqlds.CTE` (`pkg/api/routes.go:105,112`). At `ef925e1`: `HydrolixDatasource`, `GetMacroCTEs`, `CTE` are gone; the rest stays.

## Goals / Non-Goals

**Goals:**
- Pin `hydrolix/sqlds/v5` to `ef925e1` (extension-points-only tip) so plugin builds against the new sqlds shape.
- Introduce `HdxSqlDatasource` as the single, plugin-owned seam that wires extension points. Every subsequent change in the migration sequence attaches to this wrapper.
- Update call sites that referenced `sqlds.HydrolixDatasource` to use `*HdxSqlDatasource`. The substitution is mechanical because every method the plugin called on `HydrolixDatasource` is promoted through the embedded `*sqlds.SQLDatasource`.

**Non-Goals:**
- Bringing fork code into the plugin. Interpolator, macros, metadata provider, connection cache, OAuth keying each ship in their own change (C3-C7).
- Swapping the module path to `github.com/grafana/sqlds/v5`. That is `retire-hydrolix-sqlds-fork` (C8) and depends on upstream release.
- Touching the existing fork's `v5.0.1` tag. The fork stays — only the plugin's pin moves.
- Modifying behaviour. Combined with C3-C7, this change preserves panel-query semantics; in isolation, it cannot run a Hydrolix query (no interpolator, no macros).

## Decisions

### D1. Pseudo-version pin via `replace` directive

The fork at `ef925e1` declares its module path as `github.com/grafana/sqlds/v5` (anticipating the upstream merge). So the plugin's `go.mod` requires the upstream path and uses a `replace` directive to source the code from the hydrolix fork:

```
require github.com/grafana/sqlds/v5 v5.0.0-20260613103402-ef925e15e15e

replace github.com/grafana/sqlds/v5 => github.com/hydrolix/sqlds/v5 v5.0.0-20260613103402-ef925e15e15e
```

The pseudo-version encodes the commit's UTC timestamp (`2026-06-13T10:34:02Z` → `20260613103402`) and the 12-char prefix of the commit hash (`ef925e15e15e`), per Go module convention.

**Why both `require` and `replace`.** Importing `github.com/hydrolix/sqlds/v5` directly fails — Go sees the fork's go.mod declares `module github.com/grafana/sqlds/v5` and rejects the path mismatch. The `replace` directive tells Go to fetch from the hydrolix repo but treat it as the grafana module. At C8 (when upstream releases), the `replace` is removed and the `require` version moves to the released tag.

**Why pseudo-version, not branch.** `go.mod` requires a versioned reference; branch names are not legal. Tagging the fork commit would also work but adds maintenance overhead for a state that exists only during the migration window.

### D2. Wrapper type `HdxSqlDatasource` embeds `*sqlds.SQLDatasource`

```go
// pkg/plugin/hdx_sqlds.go
package plugin

import (
    "github.com/hydrolix/sqlds/v5"
)

type HdxSqlDatasource struct {
    *sqlds.SQLDatasource
}

func NewHdxSqlDatasource(driver sqlds.Driver) *HdxSqlDatasource {
    ds := sqlds.NewDatasource(driver)
    ds.EnableMultipleConnections = true
    return &HdxSqlDatasource{SQLDatasource: ds}
}
```

Subsequent changes mutate `ds` between `sqlds.NewDatasource(driver)` and the wrapper return:
- C3 sets `ds.ConnectionCacheFactory = ...`.
- C5 sets `ds.Interpolator = ...`.

**Why embed, not compose.** Embedding `*sqlds.SQLDatasource` promotes every public method (`QueryData`, `CheckHealth`, `Dispose`, `GetDBFromQuery`, the eventually-public `Interpolator` field, future methods) into `HdxSqlDatasource` for free. Composition would require a delegation method per surface and ongoing maintenance as the upstream evolves.

**Why a wrapper at all.** The wrapper centralizes extension-point wiring in one constructor. Without it, that wiring spreads across whichever file calls `sqlds.NewDatasource`. The wrapper also names a single type to test and to document; `*sqlds.SQLDatasource` is upstream-owned and shouldn't carry plugin-specific GoDoc.

**Why no plugin-only fields yet.** The struct is empty (only the embedded pointer). Subsequent changes may add fields (e.g., a reference to the metadata provider, or cached settings) — at that point the wrapper grows organically. Adding empty fields preemptively obscures what's actually used.

### D3. Substrate-level driver settings: `ForwardHeaders = false`

`Driver.Settings(ctx, config)` returns `sqlds.DriverSettings{... ForwardHeaders: false, ...}` from the moment the wrapper exists. The OAuth-keying flow that C4 introduces (`Driver.MutateQueryData` injecting `connectionArgs` with the OAuth token) depends critically on this — with `ForwardHeaders=true`, sqlds's `applyHeaders` writes the *whole* request header map into `ConnectionArgs` under the `HeaderKey`, polluting the surgical OAuth-only payload and re-keying the cache on every per-request header that varies.

**Why land this in C2 rather than C4.** The setting is a substrate-level invariant — every later change assumes it. Establishing it in the substrate keeps C4's diff focused on the actual mutator method and helper, not on a Driver-level setting that has nothing to do with mutation.

**Why default-false rather than true.** The plugin today (against `v5.0.1`) sets `ForwardHeaders=false` already. Combined with the OAuth-keying design that C4 ratifies, false is the only correct value for this plugin.

### D4. The migration is a coordinated set of changes

C2 alone is *not* a shippable PR. After this change lands and before C3-C7 land, `pkg/api/routes.go` references `sqlds.GetMacroCTEs` and `sqlds.CTE` that don't exist at `ef925e1`; the plugin won't compile, and dashboards have no interpolator or macros.

The implementation strategy options are:
- **(a) Stacked PRs**: C2 first, then C3 → C4 → C5 → C6 → C7 in order, each unblocking the next. Earlier PRs in the stack don't compile until all land; merges are gated on the full stack being green.
- **(b) Coalesced PR**: open one PR that combines C2-C7 in a single merge unit. The OpenSpec changes document the conceptual decomposition; the PR description maps commits to changes.
- **(c) Feature branch**: long-lived `feat/sqlds-extension-migration` branch that integrates C2-C7 incrementally, merged to `develop` only when complete.

The decision between (a), (b), (c) is operational and lives with whoever opens the migration. The OpenSpec proposal does not prescribe it.

**Why explicitly call this out.** Naive readers will see C2's small diff and assume it can ship in isolation. It can't. Surfacing the dependency in design prevents premature merges.

### D5. Type-rename mechanical substitutions

Each plugin file's update is a fixed substitution:

| Old                                  | New                              | Files                                                                                  |
|--------------------------------------|----------------------------------|----------------------------------------------------------------------------------------|
| `*sqlds.HydrolixDatasource`          | `*HdxSqlDatasource`              | `pkg/plugin/datasource.go`, `pkg/api/routes.go` (×2), test files                       |
| `&sqlds.HydrolixDatasource{...}`     | `NewHdxSqlDatasource(driver)`    | `pkg/plugin/datasource.go`                                                             |
| `sqlds.NewConnector(ctx, drv, ...)`  | (deleted — wrapper handles it)   | `pkg/plugin/datasource.go`                                                             |

The `pkg/api/routes.go` references to `sqlds.GetMacroCTEs` and `sqlds.CTE` stay broken until C5 ports them into the plugin.

**Why no shim layer or alias type.** A `type HydrolixDatasource = HdxSqlDatasource` alias would let the import-site updates be incremental. But the wrapper renames intentionally — `HdxSqlDatasource` makes it clear the type is plugin-owned, not sqlds-owned. An alias would obscure that. The mechanical substitution is small enough to do in one pass.

### D6. Constructor signature: take `sqlds.Driver`, not the plugin's `*Hydrolix`

`NewHdxSqlDatasource(driver sqlds.Driver)` accepts the interface, not the concrete `*Hydrolix` type. The plugin's `datasource.go` passes `NewHydrolix()` (which returns `*Hydrolix` implementing `sqlds.Driver`).

**Why the interface.** Tests can pass a fake driver without depending on the full Hydrolix driver setup. Future drivers (mock, in-memory, alternate auth) plug in without changing the wrapper.

**Why no additional constructor args (settings, context, options).** `sqlds.NewDatasource(driver)` itself takes only the driver; settings come later via the instance method `ds.NewDatasource(ctx, settings)`. The wrapper mirrors this lazy-init pattern. Changes C3 (which needs `settings.UID` for the cache key) and C4 (which uses settings for nothing) attach via fields, not constructor args.

## Risks / Trade-offs

- **[Premature merge of C2 alone breaks the build]** → Mitigation: `coordinated-shipment` note in proposal `What Changes`, plus D4 in this design. PR description (when this lands) must declare its place in the stack and link the dependent changes.
- **[Pseudo-version drifts if `ef925e1` is force-pushed]** → Mitigation: hash-based versions are stable; force-push would change the commit hash, making the old pseudo-version unresolvable, which surfaces as a `go mod tidy` failure — loud, not silent. As an extra guard, tag the fork at `ef925e1` (e.g., `v5.2.0-rc.extension-points`) when this change merges, and update `go.mod` to the tag. Tagging is operational, not a spec concern.
- **[Type-rename misses a call site]** → Mitigation: `grep -rn 'HydrolixDatasource'` before merge must return zero matches. `go vet ./...` and `golangci-lint run` catch missed call sites. CI must run with the dependent changes (C3-C7) staged.
- **[Embedding promotes a future upstream method that conflicts with a plugin-defined method]** → Mitigation: the wrapper is empty today (only the embedded pointer). When a plugin-only method gets added in a later change, the author checks for name collisions against the upstream surface; Go's compiler reports promoted-method ambiguity as a build error, so the collision is loud.

## Migration Plan

This change is the substrate for a coordinated multi-change set. It does not have a standalone migration story.

- **Forward**: open the PR in draft. Sequence inside the PR:
  1. `go mod tidy` after editing `go.mod` to the new pseudo-version. Confirm the fetch resolves.
  2. Create `pkg/plugin/hdx_sqlds.go` with `HdxSqlDatasource` and `NewHdxSqlDatasource`.
  3. Update `pkg/plugin/datasource.go` to call `NewHdxSqlDatasource`.
  4. Mechanical substitution of `*sqlds.HydrolixDatasource` → `*HdxSqlDatasource` in `pkg/api/routes.go`, `pkg/plugin/driver.go`, test files.
  5. Update `Driver.Settings()` to set `ForwardHeaders = false` explicitly (this is the substrate-level invariant per D3).
  6. Mark the PR ready for review only after the dependent changes (C3-C7) are also ready.
- **Rollback**: revert the PR. The fork still exists at `v5.0.1` on the module proxy; `go.mod` revert restores the previous import graph. No data, dashboards, or downstream consumers are affected.
- **Sequencing**: depends on `extract-hdx-query-models` (C1). Blocks C3-C8.

## Open Questions

- Should `HdxSqlDatasource` expose lifecycle hooks (`OnSettingsChanged`, `Reconfigure`) beyond what embedding promotes? Defer — no current consumer needs them.
- Should the wrapper carry plugin-cached settings (e.g., a parsed `*models.PluginSettings`) for downstream macros and the metadata provider to read? Defer to the changes that introduce those consumers (C5, C7) — they decide whether to cache on the wrapper or on their own structs.
- Once upstream `grafana/sqlds` releases the extension changes, should the pin move to upstream in a single step (`retire-hydrolix-sqlds-fork` swaps both the module path and the version), or in two steps (an intermediate pin to a tagged fork release)? Proposed: single step — fewer moving parts. Open to revisit when upstream timing is known.
