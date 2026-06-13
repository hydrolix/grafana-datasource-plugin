## Context

After C2-C7 land:

- `pkg/plugin/` owns every Hydrolix-specific behaviour previously inside the fork — interpolator, macros, metadata provider, connection cache, OAuth keying, data shapes.
- The plugin imports `github.com/hydrolix/sqlds/v5` for upstream-equivalent types: `SQLDatasource`, `Driver`, `DriverSettings`, `NewConnector`, `NewDatasource`, `CachedConnection`, `ConnectionCache`, `Interpolator` interface, etc.
- The fork at `ef925e1` contains only those upstream-equivalent types — no Hydrolix code remains.

Upstream `grafana/sqlds` has been tracking the migration via two OpenSpec changes (one for the interpolator extension, one for the connection cache extension). When upstream merges and releases a tagged version containing both, the plugin's module-path swap becomes a one-line `go.mod` edit plus a mechanical import-path rewrite.

The pseudo-version pinned in C2 (`v5.0.0-20260613133402-ef925e15e15e`) is unique to the fork's commit. Upstream's release will have a real tag (e.g., `v5.2.0`); the swap also moves from pseudo-version to tag.

## Goals / Non-Goals

**Goals:**
- Swap the plugin's sqlds module path from the fork to upstream, pinned to the first upstream tag containing both extension surfaces.
- Archive the fork repository so no future drift can occur.
- Verify byte-equivalent behaviour pre- and post-swap (the upstream release must export the same symbols the plugin consumes).

**Non-Goals:**
- Migrating other consumers of the fork. There are no other consumers — the plugin is the only one.
- Adopting any new upstream features released alongside the extension changes. If upstream's tagged release contains improvements (e.g., better error wrapping, additional extension points), the plugin adopts them in follow-up changes, not in this one.
- Rewriting any plugin code. The substitution is purely the module path; types and methods are identical.

## Decisions

### D1. Single-step module-path swap; no intermediate pin

```
// before
require github.com/hydrolix/sqlds/v5 v5.0.0-20260613133402-ef925e15e15e

// after
require github.com/grafana/sqlds/v5 v5.2.0  // exact tag TBD at landing time
```

**Why single-step.** Two sqlds modules can technically coexist in `go.mod` (different module paths), but they'd export different types under the same package alias `sqlds`. The plugin would have to alias one of them locally to avoid the collision, then de-alias once the migration completes — extra ceremony for no benefit. A direct swap is cleaner.

**Why pin to exact upstream tag, not a `^` or `~` constraint.** Reproducibility. Other Grafana plugins pin upstream sqlds to exact versions; the Hydrolix plugin follows the same convention. Major / minor / patch movement happens via explicit `go get` later.

### D2. Verify the upstream release before swapping

Before merging:

1. `go list -m -versions github.com/grafana/sqlds/v5` shows the target tag.
2. `go doc github.com/grafana/sqlds/v5.Interpolator` returns the same interface as the fork at `ef925e1`.
3. `go doc github.com/grafana/sqlds/v5.ConnectionCache` returns the same interface as the fork at `ef925e1`.
4. `go doc github.com/grafana/sqlds/v5.SQLDatasource` shows the same public fields (`Interpolator`, `ConnectionCacheFactory`, `EnableMultipleConnections`, `CallResourceHandler`, etc.).

If any of those checks fail, this change does not merge. Upstream may have shipped a renamed surface (e.g., `MacroDispatcher` instead of `Interpolator`); the plugin would adapt to the renamed surface in a follow-up change before retiring the fork.

**Why call this out as a decision.** The whole point of the C2 fork-pin is to keep the plugin running against the fork's interface shape while upstream finalises. Verifying the shape match before swapping is the safety net.

### D3. Mechanical import-path swap via `goimports`

```bash
find pkg -name '*.go' -exec sed -i '' 's|github.com/hydrolix/sqlds/v5|github.com/grafana/sqlds/v5|g' {} +
goimports -w pkg/
go mod tidy
go build ./...
go test -race ./...
```

**Why a sed-based swap.** Single token, no false positives within the plugin's code. The import path is unique and not a substring of any other identifier. Verified by `grep -rn 'hydrolix/sqlds' pkg/` returning zero lines after the sed pass.

**Why `goimports -w` after sed.** Cleans up import-block formatting (groups stdlib / third-party / internal, removes any leftover blank lines).

**Why `go mod tidy`.** Removes the now-unused `hydrolix/sqlds/v5` from `go.sum`, adds `grafana/sqlds/v5` and its transitive deps. The diff is `go.mod` + `go.sum` + every file containing the import.

### D4. Archive the fork repository

After this change merges and the plugin is running on upstream sqlds in production for ~one release cycle (operationally, 2-4 weeks), the fork repository is archived:

1. Add a `DEPRECATED.md` to the fork's root pointing at `grafana/sqlds`.
2. Mark the repository as archived on GitHub (read-only; no new issues, PRs, or commits).
3. Remove the fork from any internal release tooling that pulls from it.

**Why wait 2-4 weeks.** The plugin's deployment cadence is monthly; rolling back to the fork in case of a regression is a meaningful safety valve in the first cycle. Once the rollback window passes, the fork is dead weight.

**Why archive rather than delete.** Future audits, blame archaeology, and historical references to the catalog finding need the fork accessible. Archive preserves read access without enabling new changes.

### D5. No code or behaviour changes in the plugin

This change is module-path-only. Every symbol used from `github.com/hydrolix/sqlds/v5` exists in `github.com/grafana/sqlds/v5` at the released tag with the same signature (verified per D2). The plugin's `pkg/plugin/` and `pkg/api/` files compile against the new import without any source change beyond the import block.

**Why this is checkable, not just asserted.** The `go build ./...` and `go test -race ./...` runs after the sed swap are the verification. Any incompatibility surfaces immediately as a compile error or test failure. The PR description includes a diff summary: "N files changed, N+0 insertions, N+0 deletions" — the +0 confirms that only imports moved.

## Risks / Trade-offs

- **[Upstream release ships a renamed or restructured extension surface]** → Mitigation: D2's pre-merge verification checks each consumed symbol. If any fails, the change pauses and a separate adaptation change lands first to rename the plugin's references.
- **[Upstream takes longer than expected to release]** → Acceptable: the plugin runs against the fork at `ef925e1` indefinitely; no behaviour difference. The catalog-review finding sees partial closure once C2-C7 ship (the security-sensitive code moves into the plugin); full closure depends on this change.
- **[A consumer of the fork repository other than this plugin exists]** → Mitigation: verified before archive. `gh search code github.com/hydrolix/sqlds` plus the internal-tools audit confirms no other consumer. If one surfaces, it gets its own migration plan before the archive step.
- **[`go mod tidy` introduces an unintended transitive-dep upgrade]** → Mitigation: review the `go.sum` diff line-by-line; major version bumps in transitive deps trigger a side-investigation before merging. Most transitive deps are stable across sqlds versions.
- **[Reverting after archive is harder than reverting before archive]** → Acceptable: the archive step is gated on operational confidence (D4's 2-4 week wait). Pre-archive revert is a `go.mod` revert, trivial; post-archive revert would require unarchiving the fork, which is fast on GitHub.

## Migration Plan

- **Forward**:
  1. Confirm upstream `grafana/sqlds` has a released tag containing the merged extension changes. Run D2's pre-merge checks.
  2. Open this change's PR. Sequence:
     - Edit `go.mod` to swap the require line.
     - Run `find pkg -name '*.go' -exec sed -i '' 's|github.com/hydrolix/sqlds/v5|github.com/grafana/sqlds/v5|g' {} +`.
     - Run `goimports -w pkg/`, `go mod tidy`.
     - Run quality gates: `go vet ./...`, `golangci-lint run`, `go test -race ./...`.
     - Run Playwright e2e via the `grafana-plugin-e2e` skill — every panel-query and ad-hoc-filter scenario must pass.
     - PR description lists every file touched and confirms each is import-only (+N, -N for the same N per file).
  3. Merge to `develop`. Cut a release per the plugin's normal cadence.
  4. After 2-4 weeks of stable production on upstream sqlds, archive the fork (D4).
- **Rollback (before archive)**: revert the PR. `go.mod` and import paths go back to the fork pin from C2. Plugin behaviour is unchanged.
- **Rollback (after archive)**: unarchive the fork on GitHub (one-click), revert the PR, redeploy.
- **Sequencing**: depends on C2-C7 (so the plugin owns every Hydrolix behaviour locally), and on the upstream release being available. Calendar-gated, not code-gated.

## Open Questions

- Should this change also bump any other dependencies that have piled up since the last refresh? Defer — this change is import-path-only. Dependency refreshes are their own change.
- Should the plugin's documentation (`README.md`, internal runbooks) be updated to reference upstream `grafana/sqlds` instead of the fork? Yes, in the same PR — a small documentation hygiene addition is appropriate alongside the swap. Listed under "Forward" step 2 as part of the PR.
- Should the fork's git history be force-imported into a `legacy` branch of the upstream `grafana/sqlds` for historical preservation? No — upstream wouldn't want plugin-specific history in their main repository; the fork stays archived as its own repo.
