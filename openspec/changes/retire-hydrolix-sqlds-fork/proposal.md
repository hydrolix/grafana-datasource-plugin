## Why

C2 pins the plugin to `github.com/hydrolix/sqlds/v5` at the extension-points-only revision `ef925e1`. At that revision the fork is functionally equivalent to upstream `grafana/sqlds@6c09016` (release 5.1.1) plus two extension surfaces: the `Interpolator` interface + field, and the `ConnectionCache` interface + factory field. The fork carries no Hydrolix logic; that all lives in `pkg/plugin/` after C3-C7.

Once those two extension changes land in `grafana/sqlds` upstream and are released, the fork has no remaining reason to exist. The plugin's `go.mod` module path swaps from `github.com/hydrolix/sqlds/v5` to `github.com/grafana/sqlds/v5` in one line, every import in the plugin (`pkg/plugin/`, `pkg/api/`) follows the same swap, and the fork repository can be archived.

This change is intentionally trivial. It's separate from C2 because it can only land *after* upstream releases — a calendar dependency, not a code dependency. Keeping it as its own change lets C2-C7 ship as soon as the substrate + extension implementations are ready, decoupled from the upstream's release timing.

## What Changes

- `go.mod`: replace `require github.com/hydrolix/sqlds/v5 v5.0.0-20260613133402-ef925e15e15e` with `require github.com/grafana/sqlds/v5 <upstream-released-tag>`. `go.sum` regenerates via `go mod tidy`.
- Bulk import-path swap across `pkg/plugin/*.go` and `pkg/api/*.go`: every `github.com/hydrolix/sqlds/v5` → `github.com/grafana/sqlds/v5`. `goimports -w` handles the mechanical edit.
- Verify the upstream release contains both `Interpolator` interface + `SQLDatasource.Interpolator` field AND `ConnectionCache` interface + `SQLDatasource.ConnectionCacheFactory` field. Pin to the first upstream tag that has both.
- Archive the `github.com/hydrolix/sqlds` fork repository (mark read-only on GitHub, add a `DEPRECATED.md` pointing at upstream). The plugin no longer consumes it; no other downstream consumers exist.
- No code or behaviour changes in the plugin beyond the module-path swap. Same types, same methods, same wire format.
- Go unit tests run unchanged; no test additions.
- Playwright e2e runs unchanged; no test additions.

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics.

## Capabilities

### New Capabilities

<!-- None — this change closes out the fork's existence; no plugin capability is added or modified. -->

### Modified Capabilities

<!-- Capabilities established in C1-C7 are unchanged. The sqlds module path swap is below the capability surface. -->

## Impact

- **Frontend**: none.
- **Backend (Go)**: one-line `go.mod` swap; bulk import-path rewrite (`goimports -w`); `go mod tidy`.
- **Tests**: no changes. The test surface from C1-C7 exercises the same types via the new module path.
- **Dependencies**: `github.com/hydrolix/sqlds/v5` removed; `github.com/grafana/sqlds/v5` added at the released tag containing both extension surfaces. Transitive deps regenerate via `go mod tidy`.
- **User-visible**: none.
- **Security**: closes the catalog-review finding. Every line of sqlds code the plugin links against is now in maintained, reviewed `grafana/sqlds`. The fork repository is archived; no consumer can land changes to it.
- **Sequencing**: depends on C2-C7 being merged AND `grafana/sqlds` releasing a version that contains the merged `add-extension-points` (interpolator) and `add-connection-cache` extensions. Calendar-blocked until upstream releases.
