# sqlds-migration-plan — orchestration tasks

This `tasks.md` drives the sqlds retirement end-to-end. Run `/opsx:apply` on this change to walk the sequence.

> **Execution outcome (2026-07-09).** The migration is complete. C1–C7 were
> built and archived (`archive/2026-06-26-*`); C8 (`retire-hydrolix-sqlds-fork`)
> and the follow-on `adopt-sqlds-func-interpolator` were built, verified
> (go build/vet/`test -race` + golangci-lint + `npm run build` + 32/32
> Playwright e2e against a v5.2.0-linked backend), and archived
> (`archive/2026-07-09-*`); their spec deltas are synced into
> `openspec/specs/`. **The fork is retired**: `go.mod` no longer carries the
> `replace` directive and `github.com/hydrolix/sqlds` is absent from
> `go.mod`/`go.sum` (only attribution comments remain in `pkg/`). The
> checkboxes below were not maintained during execution and are left as-authored
> for historical reference. Remaining items are ordinary git/ops steps tracked
> outside OpenSpec: commit + merge to `develop`, deploy + soak, and the optional
> archival of the fork's GitHub repo (§11.3) — none gate the plugin-side
> retirement, which is done.

Each `## N. ...` section corresponds to one child change (or a setup/cleanup step). Inside each section, sub-tasks run in order. Move to the next section only when every box in the current section is checked.

Notation:
- `[ ]` open
- `[x]` complete
- `[~]` skipped intentionally (record the reason inline)

## 0. Operator setup (before any code change)

- [ ] 0.1 Record PR strategy choice in this section: `(a) stacked`, `(b) coalesced` *(recommended — see design D1)*, or `(c) feature branch`. Picked: ____.
- [ ] 0.2 Confirm `feature/review_remarks` (or the active working branch) is checked out and clean (`git status` shows no unexpected staged changes).
- [ ] 0.3 Confirm every child change's `proposal.md` and `design.md` are at the revision intended for execution. `git log --oneline openspec/changes/extract-hdx-query-models/ openspec/changes/pin-sqlds-extension-revision/ openspec/changes/plugin-ttl-connection-cache/ openspec/changes/plugin-oauth-keyed-pooling/ openspec/changes/plugin-hdx-interpolator/ openspec/changes/plugin-clickhouse-time-date-macros/ openspec/changes/plugin-adhoc-filter-macro-secure/ openspec/changes/retire-hydrolix-sqlds-fork/` — note the last commit. Any drift surfaced here is a clarify-before-applying signal, not a continue signal.
- [ ] 0.4 Remove legacy directory `openspec/changes/retire-sqlds-fork/` (superseded by this orchestrated set). `git rm -r openspec/changes/retire-sqlds-fork && git commit -m "openspec: retire legacy retire-sqlds-fork change (superseded by granular C1-C8)"`.

## 1. C1 — extract-hdx-query-models

Independent of every other change. Lands as its own PR.

- [ ] 1.1 Re-read `openspec/changes/extract-hdx-query-models/proposal.md` and `design.md`. Confirm the scope matches expectations.
- [ ] 1.2 Generate this change's `specs/` capability deltas (skipped during planning per openspec config). Capability: `hdx-query-models`. Spec captures: `HdxQuery`, `AdHocFilter`, `PluginSettings`, `QuerySetting` shapes; `IsValid` and `SetDefaults` semantics.
- [ ] 1.3 Generate this change's `tasks.md` (per-change implementation checklist). Tasks: create `pkg/plugin/models/` package; copy types verbatim from the fork at the plugin's current pin (`v5.0.1`); rename `HDXQuery` → `HdxQuery`; update four import sites (`pkg/api/routes.go`, `pkg/plugin/driver.go`, `pkg/plugin/driver_test.go`, `pkg/plugin/dssuit_test.go`); port `models/settings_test.go` cases.
- [ ] 1.4 Run `/opsx:apply` on `extract-hdx-query-models`. Watch for any task failure; pause and resolve before continuing.
- [ ] 1.5 Run `/opsx:verify` on `extract-hdx-query-models`. Must report green.
- [ ] 1.6 Run quality gates: `npm run typecheck && npm run lint && npm test -- --ci && go vet ./... && golangci-lint run && go test -race ./...`. All must pass.
- [ ] 1.7 Open PR (per the strategy from 0.1). PR description references this change's `proposal.md`.
- [ ] 1.8 Pass code review. Merge.

## 2. C2 — pin-sqlds-extension-revision (substrate)

The first change in the coordinated set. Lands as its own PR only under strategy (a); under (b) it's the first commit in the coalesced PR; under (c) it's the first merge into the feature branch.

- [ ] 2.1 Confirm C1 merged.
- [ ] 2.2 Re-read `openspec/changes/pin-sqlds-extension-revision/proposal.md` and `design.md`.
- [ ] 2.3 Generate this change's `specs/` capability deltas. Capability: `hdx-sqlds-wrapper`.
- [ ] 2.4 Generate this change's `tasks.md`. Tasks: edit `go.mod` to pin `hydrolix/sqlds/v5` to the pseudo-version for `ef925e1`; add `pkg/plugin/hdx_sqlds.go`; mechanical substitution `*sqlds.HydrolixDatasource` → `*HdxSqlDatasource` across plugin and tests; set `Driver.Settings().ForwardHeaders = false`.
- [ ] 2.5 Run `/opsx:apply` on `pin-sqlds-extension-revision`. Expect the plugin to *not* build at this point (deliberate — see design D4 of C2). The verify step (next) accepts this.
- [ ] 2.6 Run `/opsx:verify` on `pin-sqlds-extension-revision`. Verification mode allows the "compiles only with C3-C7" caveat; verify confirms the substrate is correctly wired, not that the package builds.
- [ ] 2.7 *No quality gates here in strategy (b) or (c); they run after C7.* Under strategy (a), open the PR as a draft and link the dependent change PRs.

## 3. C3 — plugin-ttl-connection-cache

Part of the coordinated set. Order within the set is flexible (file-level dependencies don't force a strict order).

- [ ] 3.1 Re-read `openspec/changes/plugin-ttl-connection-cache/proposal.md` and `design.md`.
- [ ] 3.2 Generate this change's `specs/` capability deltas. Capability: `hdx-ttl-connection-cache`. Modified: `hdx-sqlds-wrapper` (`NewHdxSqlDatasource` constructor signature grows `settings`).
- [ ] 3.3 Generate this change's `tasks.md`. Tasks: add `pkg/plugin/connection_cache.go` + tests; update `pkg/plugin/hdx_sqlds.go` to take `settings` and wire `ds.ConnectionCacheFactory`; update `pkg/plugin/datasource.go` to pass `settings`; `go mod tidy`.
- [ ] 3.4 Run `/opsx:apply` on `plugin-ttl-connection-cache`.
- [ ] 3.5 Run `/opsx:verify` on `plugin-ttl-connection-cache`.

## 4. C4 — plugin-oauth-keyed-pooling

- [ ] 4.1 Confirm C3 applied (file-level dep: shared `EnableMultipleConnections=true` and `ForwardHeaders=false` invariants).
- [ ] 4.2 Re-read `openspec/changes/plugin-oauth-keyed-pooling/proposal.md` and `design.md`.
- [ ] 4.3 Generate this change's `specs/` capability deltas. Capability: `hdx-oauth-keyed-pooling`.
- [ ] 4.4 Generate this change's `tasks.md`. Tasks: extend `MutateQueryData`; add `pkg/plugin/connection_args.go` + tests; update `Driver.Connect`'s `forwardOAuth` branch to return a lazy `*sql.DB` when `args == nil`; update existing `MutateQueryData` tests.
- [ ] 4.5 Run `/opsx:apply` on `plugin-oauth-keyed-pooling`.
- [ ] 4.6 Run `/opsx:verify` on `plugin-oauth-keyed-pooling`.

## 5. C5 — plugin-hdx-interpolator

- [ ] 5.1 Confirm C2 applied (need the wrapper to assign `ds.Interpolator`).
- [ ] 5.2 Re-read `openspec/changes/plugin-hdx-interpolator/proposal.md` and `design.md`.
- [ ] 5.3 Generate this change's `specs/` capability deltas. Capability: `hdx-interpolator`. Modified: `hdx-sqlds-wrapper`.
- [ ] 5.4 Generate this change's `tasks.md`. Tasks: add `pkg/plugin/macros_registry.go` with empty `Macros` map; add `pkg/plugin/cte.go` with `CTE` + `GetMacroCTEs`; add `pkg/plugin/interpolator.go` with `HdxInterpolator`; port tests verbatim from the fork; update `pkg/api/routes.go` to use plugin-local `GetMacroCTEs` / `CTE`; update wrapper to assign `ds.Interpolator`.
- [ ] 5.5 Run `/opsx:apply` on `plugin-hdx-interpolator`.
- [ ] 5.6 Run `/opsx:verify` on `plugin-hdx-interpolator`.

## 6. C7 — plugin-adhoc-filter-macro-secure

C7 ships before C6 because C6's PK-lookup macros depend on `MetadataProvider` + `getPK` defined in C7.

- [ ] 6.1 Confirm C5 applied (need the `Macros` registry and `GetMacroCTEs`).
- [ ] 6.2 Re-read `openspec/changes/plugin-adhoc-filter-macro-secure/proposal.md` and `design.md`.
- [ ] 6.3 Generate this change's `specs/` capability deltas. Capability: `hdx-adhoc-filter-macro-secure`. Modified: `hdx-interpolator` (registry gains `adHocFilter`), `hdx-sqlds-wrapper` (parses settings on construction, constructs `MetadataProvider`).
- [ ] 6.4 Generate this change's `tasks.md`. Tasks: add `pkg/plugin/metadata.go` with `MetadataProvider` + `getPK` (port from fork modulo type renames); add `pkg/plugin/macros_adhoc.go` with `AdHocFilterMacro` + `escape` + helpers (every `$$%s$$` swaps to `'%s'` with `escape(value)`); register macro via `init()`; update wrapper to parse settings and construct `MetadataProvider`; port tests; add new escape-correctness fuzz test; add one Playwright e2e for filter value with single quote.
- [ ] 6.5 Run `/opsx:apply` on `plugin-adhoc-filter-macro-secure`.
- [ ] 6.6 Run `/opsx:verify` on `plugin-adhoc-filter-macro-secure`.

## 7. C6 — plugin-clickhouse-time-date-macros

- [ ] 7.1 Confirm C5 applied (`Macros` registry exists) and C7 applied (`MetadataProvider` + `getPK` exist).
- [ ] 7.2 Re-read `openspec/changes/plugin-clickhouse-time-date-macros/proposal.md` and `design.md`.
- [ ] 7.3 Generate this change's `specs/` capability deltas. Capability: `hdx-clickhouse-time-date-macros`.
- [ ] 7.4 Generate this change's `tasks.md`. Tasks: add `pkg/plugin/macros_time.go` with time-conversion helpers; add `pkg/plugin/macros_clickhouse.go` with 11 macros + `init()` registration; port macro tests from fork.
- [ ] 7.5 Run `/opsx:apply` on `plugin-clickhouse-time-date-macros`.
- [ ] 7.6 Run `/opsx:verify` on `plugin-clickhouse-time-date-macros`.

## 8. Coordinated-set quality gates and merge (C2-C7)

At this point C2-C7 are all applied. The plugin compiles and tests pass for the first time since C2 was applied.

- [ ] 8.1 Run full quality gates: `npm run typecheck && npm run lint && npm test -- --ci && go vet ./... && golangci-lint run && go test -race ./...`. All must pass.
- [ ] 8.2 Run Playwright e2e via the `grafana-plugin-e2e` skill. Every panel-query, ad-hoc filter, and annotation scenario must pass. Pay particular attention to the new e2e for the quote-in-filter-value case (from C7's tasks).
- [ ] 8.3 Under strategy (a) stacked: mark each stacked PR ready for review in dependency order; merge top-down. Under (b) coalesced: open the coalesced PR with C2-C7 as separate commits; review; merge. Under (c) feature branch: merge the feature branch to `develop`.
- [ ] 8.4 Confirm the merge landed on the target branch. Confirm CI is green on `develop`.

## 9. Deploy + soak (between C7 and C8)

- [ ] 9.1 Deploy the post-C7 plugin to staging. Run smoke tests against a Hydrolix cluster: a non-OAuth deployment and a `forwardOAuth` deployment.
- [ ] 9.2 Monitor for one full deploy cycle (per the team's normal release cadence — typically 1-2 weeks). Watch for regression signals: panel-query error rates, datasource-instantiation failures in `forwardOAuth` deployments, connection-cache memory growth.
- [ ] 9.3 If a regression surfaces, follow the rollback playbook in `design.md` D3 (post-C7, pre-C8 phase). Revert C2-C7 as a single unit; the plugin returns to running on the fork at `v5.0.1`.
- [ ] 9.4 If no regression, proceed to C8 when upstream is ready.

## 10. C8 — retire-hydrolix-sqlds-fork (calendar gate SATISFIED)

> **Calendar gate satisfied.** Upstream `grafana/sqlds` released **`v5.2.0`**
> carrying both extension surfaces. C8 is no longer calendar-blocked. Note: the
> swap is **not** the pure no-op the plan originally assumed — upstream reshaped
> `sqlds.CachedConnection` from the fork's interface into a concrete value
> struct, so C8 now carries a connection-cache adaptation (a `hdx-ttl-connection-cache`
> spec delta + code/test changes). C8's `specs/`, `design.md`, and `tasks.md`
> have been regenerated accordingly; the interpolator surface is verified
> identical (no delta there). Also folds in `adopt-sqlds-func-interpolator`,
> whose remaining "advance the fork pin" verification is subsumed by C8's gates.

- [ ] 10.1 ~~Confirm `grafana/sqlds` has released a version with both extension surfaces~~ — **done**: `v5.2.0` contains the func-typed `Interpolator` field, the `ConnectionCache` interface, and `SQLDatasource.ConnectionCacheFactory`. Target tag: `v5.2.0` (not `v5.3.0`, which requires Go ≥ 1.26.4 — see C8 design D6).
- [ ] 10.2 Verify surface parity per C8 design D2 (build against `v5.2.0` in a throwaway worktree). Expected result: `Interpolator` / `ConnectionCacheFactory` / `ConnectionCache` identical to the fork; `CachedConnection` diverged (interface → concrete struct). The divergence is the scope of C8's connection-cache adaptation, not a blocker.
- [ ] 10.3 Re-read `openspec/changes/retire-hydrolix-sqlds-fork/proposal.md`, `design.md`, `tasks.md`, and `specs/hdx-ttl-connection-cache/spec.md`.
- [ ] 10.4 C8 `specs/` deltas: `hdx-ttl-connection-cache` (MODIFIED for the value-type `CachedConnection` + ADDED close-observation seam). No `hdx-interpolator` delta (surface unchanged upstream).
- [ ] 10.5 C8 `tasks.md` (already generated): drop the fork `replace` + pin `v5.2.0` + `go mod tidy` (no import rewrite — the path is already `grafana/sqlds/v5`); adapt `connection_cache.go` + rewrite its test; update docs.
- [ ] 10.6 Run `/opsx:apply` on `retire-hydrolix-sqlds-fork`.
- [ ] 10.7 Run `/opsx:verify` on `retire-hydrolix-sqlds-fork`.
- [ ] 10.8 Run full quality gates + Playwright e2e (as in 8.1, 8.2).
- [ ] 10.9 Open PR. The diff is `go.mod`/`go.sum` + `connection_cache.go` + its test + docs — **not** import-only (the connection-cache adaptation is real code).
- [ ] 10.10 Merge. Deploy to production.

## 11. Post-migration cleanup

- [ ] 11.1 Wait 2-4 weeks after C8 deploys to production. Monitor for regressions; if any surface, follow design D3 post-C8 rollback path.
- [ ] 11.2 Archive each child change via `/opsx:archive` in order: C1, C2, C3, C4, C5, C6, C7, C8. Each `archive` moves the change's `openspec/changes/<name>/` to `openspec/archive/`.
- [ ] 11.3 Archive `github.com/hydrolix/sqlds` repository on GitHub (add `DEPRECATED.md`, mark read-only).
- [ ] 11.4 Archive this `sqlds-migration-plan` change via `/opsx:archive`. The completed `tasks.md` (with every box checked) is the historical record.
- [ ] 11.5 If the migration produced any documentation worth keeping (e.g., the "lessons learned" from the coordinated-set landing), file it in the team's docs repository, not in the plugin repo.

## Rollback reference

If a step fails partway through, consult `design.md` D3. Summary:

- Failure during section 1 → revert C1 alone.
- Failure during sections 2-7 → revert the coordinated set (revert the coalesced PR / reset the feature branch / revert each stacked PR in reverse order).
- Failure during section 9 (post-C7 deploy) → revert C2-C7; the plugin returns to the fork at `v5.0.1` (C1 stays applied).
- Failure during section 10 (C8) → revert C8; the plugin runs on the fork at `ef925e1`.
- Failure during section 11 (post-C8 deploy) → revert C8; if needed, revert C2-C7 as a second step.

Each rollback restores a known-good state. No partial rollbacks.
