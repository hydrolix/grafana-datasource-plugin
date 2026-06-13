## Context

The plugin's sqlds retirement has 8 child changes with this dependency graph (extracted from each child's `Sequencing:` bullets):

```
                  ┌─► C3 ──► C4 ─┐
C1 ──► C2 ────────┼─► C5 ────────┼─► C8
                  ├─► C6 ────────┤
                  └─► C7 ────────┘
                       ▲
                       └── C7 also unblocks C5's MetadataProvider sentinel
                       └── C6 also depends on C7 (getPK + MetadataProvider)
```

Effective ordering:
- **C1** ships first; independent of the sqlds revision swap.
- **C2** ships second; the substrate. Subsequent changes plug into the wrapper C2 creates.
- **C3, C4, C5, C6, C7** ship as a coordinated set. C2 alone leaves the plugin non-functional; C3-C7 collectively restore it. Within the set, file-level dependencies are: C6 depends on C7 (`getPK`); C7 depends on C5 (`Macros` registry); C5 depends on C2 (wrapper). The merge unit is "C2 through C7 together".
- **C8** ships last; calendar-gated on `grafana/sqlds` releasing a version with both extension surfaces (interpolator + connection cache).

`/opsx:apply` and `/opsx:verify` are the OpenSpec invocations that drive a single change. The migration-plan's `tasks.md` is the script that invokes them in order against the eight child changes.

## Goals / Non-Goals

**Goals:**
- Make the dependency order and shipment strategy auditable in one durable file (the `tasks.md` of this change).
- Standardise the verification gate between each child change: `/opsx:verify` must pass before the next child's `/opsx:apply` runs.
- Encode the PR-strategy choice (stacked / coalesced / feature-branch) as an early decision the operator records, then follows uniformly.
- Provide a rollback playbook keyed to where in the sequence a failure is detected.

**Non-Goals:**
- Re-stating the child changes' contents. The `proposal.md` and `design.md` of each child are normative; the migration plan references them, doesn't duplicate them.
- Automating the merge of child PRs. Merge gates (review, CI, manual approval) are governed by the team's existing GitHub workflow, not by this change.
- Driving the upstream `grafana/sqlds` release. C8's calendar gate is owned by upstream; this change just blocks until upstream releases.

## Decisions

### D1. PR strategy: pick once at the start, then commit

Operator picks one of three:

- **(a) Stacked PRs** — one PR per child change. Each PR is opened in draft, set to "ready for review" only after the prior PR merges. Within the C2-C7 coordinated set, intermediate PRs don't compile (e.g., C2 alone doesn't compile because C5's `GetMacroCTEs` is missing); set the GitHub branch-protection bypass for those, or mark them as draft until the full stack is ready.
- **(b) Coalesced PR** — one PR that combines C2-C7 as separate commits in a single merge unit. C1 still ships first as its own PR; C8 still ships last. Inside the coalesced PR, each commit message references its child change ("commit 1 of 6: pin-sqlds-extension-revision (substrate)").
- **(c) Feature branch** — long-lived `feat/sqlds-extension-migration` branch. C2-C7 merge into the feature branch as separate small PRs (CI runs against the feature-branch head). The feature branch merges to `develop` in one PR when complete.

**Recommendation: (b) coalesced.** Single PR, full diff in one place for review, single CI run, single merge moment. Stacked (a) has the cleanest commit history but the heaviest PR-management overhead. Feature branch (c) is correct for very long migrations (months); this one is days-to-weeks.

**Why call this out as a decision the operator records.** Whichever option is chosen, the rest of the orchestration (verification gates, rollback steps) is the same. Recording the choice once removes ambiguity from every later step.

### D2. Verification gates between child changes

After each child's `/opsx:apply` completes, run `/opsx:verify` before moving on:

- `/opsx:verify` confirms the implementation matches the proposal/design/tasks/specs artifacts.
- The child's "Sequencing:" bullet in its own `proposal.md` lists the gates (typically: `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`).
- Playwright e2e: C2-C6 are not individually e2e-verifiable (no functioning query path); the e2e suite runs once at the end of C7 (when interpolator + macros are complete) and once at the end of C8 (full upstream-sqlds path).

**Why split the e2e cadence.** Running Playwright after every child change wastes CI minutes for changes that can't produce a working query path. Running it twice (after C7, after C8) catches the user-facing surfaces at the points they're meaningfully testable.

### D3. Rollback playbook keyed to detection point

If a problem surfaces:

- **Before C2 merges**: revert C1 alone; the plugin is fully on the fork at `v5.0.1`. No data, dashboards, or downstream affected.
- **During C2-C7 (coordinated set incomplete)**: in strategy (a), revert the topmost stacked PR; in (b), revert the coalesced PR; in (c), reset the feature branch. The plugin's `develop` branch is unaffected because the set hadn't merged.
- **After C2-C7 merges, before C8 calendar gate**: the plugin runs on the fork at `ef925e1`. Any regression detected here implies a bug in one of C2-C7; revert the merged C2-C7 (whichever PR form was used). The plugin returns to running on the fork at `v5.0.1`.
- **After C8 merges**: the plugin runs on upstream `grafana/sqlds@<tag>`. Revert C8 to return to the fork at `ef925e1`; then revert C2-C7 if needed to return to `v5.0.1`.

**Why split by detection point.** Each phase has a different revert scope. Conflating them risks over-reverting (taking out C1 when only C5 broke) or under-reverting (leaving the plugin in an inconsistent state).

### D4. The migration-plan itself is a one-shot artifact

After the sqlds retirement completes (C8 merges, fork archived), this change is itself archived via `/opsx:archive`. The artifacts move to `openspec/archive/` as a historical record of how the retirement was sequenced.

**Why archive rather than delete.** Future migrations may follow the same pattern (granular changes + orchestration meta-change); having a worked example accessible in `archive/` lets the next operator copy the shape rather than reinvent it.

### D5. Legacy `retire-sqlds-fork/` directory is removed in this change's tasks

The pre-existing `openspec/changes/retire-sqlds-fork/` directory contains an earlier, monolithic version of this migration's proposal + design. It is superseded by the granular C1-C8 set and by this migration plan. Removing it during the orchestration prevents confusion (two competing migration documents in the same repo).

**Why remove rather than rename or archive.** It was never applied — there's nothing to preserve in `archive/`. The conversation that produced it is preserved via git history.

**Why fold the removal into this change's tasks.** The legacy directory's existence is a planning artifact, not a code artifact. Removing it is part of the same operational pass as installing the new orchestration; doing it now (rather than as a separate one-line PR later) keeps the planning artifacts coherent.

## Risks / Trade-offs

- **[Operator skips `/opsx:verify` between gates]** → Mitigation: `tasks.md` is structured so the verify step is its own checkbox; skipping it visibly leaves an unchecked box in the migration's progress. PR description for the C2-C7 merge unit lists every child change's verify status.
- **[A child change's proposal/design changes mid-migration and the migration-plan goes stale]** → Mitigation: each `tasks.md` section starts with a "confirm child proposal+design unchanged since plan was drafted" step. A `git log openspec/changes/<child>/proposal.md openspec/changes/<child>/design.md` since this plan's commit reveals any drift loudly.
- **[Upstream `grafana/sqlds` release delayed beyond reasonable wait]** → Acceptable: the plugin runs against the fork at `ef925e1` indefinitely with full Hydrolix behaviour restored. C8 remains pending; security finding is partially closed (Hydrolix code is plugin-owned and reviewable) until full closure when C8 lands.
- **[Mid-stream operator change]** → Acceptable: `tasks.md` is the durable handoff. A new operator reads the file and continues from the first unchecked task. The PR-strategy decision (D1) is recorded at the top so they don't re-pick it.

## Migration Plan

This change has no migration plan of its own — it *is* the migration plan. Forward action is to run `/opsx:apply` on `sqlds-migration-plan`, which walks `tasks.md` in order.

Rollback: `/opsx:archive` this change (without `/opsx:apply`) to remove it from the active set without losing the artifact.

## Open Questions

- Should the migration-plan's `tasks.md` also include the post-merge cleanup (archive each child change via `/opsx:archive`, remove the legacy `retire-sqlds-fork/` directory)? Yes — see `tasks.md` for the explicit cleanup section.
- Should this change have an explicit `apply-strategy` field that, when set to `coalesced` (D1's recommendation), changes how the verification gates in `tasks.md` are interpreted? Defer — the gates are the same regardless of PR strategy; only the PR-management mechanics differ.
- Should `tasks.md` reference the specific commit hash of `grafana/sqlds`'s release (when known) for C8's calendar gate? Yes — once upstream tags, this change's `tasks.md` is updated to pin the exact tag. Until then, the gate reads "upstream release containing both `Interpolator` and `ConnectionCache` extension surfaces".
