## Why

The plugin's retirement of `github.com/hydrolix/sqlds` ships as eight related OpenSpec changes (`extract-hdx-query-models`, `pin-sqlds-extension-revision`, `plugin-ttl-connection-cache`, `plugin-oauth-keyed-pooling`, `plugin-hdx-interpolator`, `plugin-clickhouse-time-date-macros`, `plugin-adhoc-filter-macro-secure`, `retire-hydrolix-sqlds-fork`). Each child change describes its own scope, capability, and migration plan. None of them describe the *meta* concerns: the dependency order between them, the merge-strategy options, the calendar gate on the upstream sqlds release, the sequence in which `/opsx:apply` / `/opsx:verify` should be invoked, the verification points between gates, the rollback playbook for partial failures.

That meta-information lives nowhere durable today — it's scattered across "Sequencing:" prose in each child's proposal, the in-conversation ASCII dependency graph, and ephemeral task-tool state. Anyone restarting the migration without a memory of the planning conversation has to reconstruct it from eight files.

This change is the durable, single-source-of-truth meta-orchestration. Its `tasks.md` is the executable sequence: ordered, dependency-aware, with explicit verification gates between changes. Running `/opsx:apply` on this change walks the sequence — apply C1, verify C1, merge C1, apply C2, verify C2, … through C8.

The migration-plan is a process artifact, not a code artifact. It adds no capability spec, no production code, no test code. It exists to make the eight-change sequence reproducible and to fail loudly if any prerequisite is unmet.

## What Changes

- Add `openspec/changes/sqlds-migration-plan/` containing `proposal.md`, `design.md`, and `tasks.md`.
- `tasks.md` is the orchestration checklist: one section per child change, with sub-tasks for reviewing the proposal, invoking `/opsx:apply`, invoking `/opsx:verify`, running the quality gates the child specifies, opening the PR, and merging.
- Dependency order codified in `tasks.md`: C1 first (independent), C2 second (substrate, blocks everything below), C3-C7 in any order within the merge window (coordinated set; PR strategy choice noted in `design.md`), C8 last (calendar-gated on upstream `grafana/sqlds` release).
- Verification gates between sections: each child's "Sequencing" bullets in its own `proposal.md` are normative; this change enforces they have been satisfied before the next section can start.
- Supersedes the legacy `openspec/changes/retire-sqlds-fork/` (pre-existing, never landed) — that change's scope is now distributed across C2-C7 and C8.
- No code, test, or capability changes. Playwright e2e unchanged. Go test surface unchanged.

Not breaking for anything. This change adds documentation artifacts only.

## Capabilities

### New Capabilities

<!-- None. This change is process orchestration; it does not codify a runtime capability. -->

### Modified Capabilities

<!-- None. Each child change in C1-C8 owns its capability deltas. -->

## Impact

- **Frontend**: none.
- **Backend (Go)**: none.
- **Tests**: none. The child changes own their test surfaces.
- **Dependencies**: none.
- **User-visible**: none.
- **Security**: indirect. The catalog-review finding closes when C7 (security fix) and C8 (fork retirement) both land; this change makes the path to that closure auditable.
- **Sequencing**: this change can be drafted at any time before C1 starts. It is the entry point for executing the sqlds retirement; its `tasks.md` references but does not block on C1-C8 directly. Removing the legacy `retire-sqlds-fork/` directory is part of this change's `tasks.md`.
