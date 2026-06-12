# Release Pipeline and Branching

This document describes how the plugin is branched, built, and released. It covers three things:

1. The GitHub repository settings that gate releases.
2. The branching model and how a developer interacts with it.
3. The CI pipeline that runs before and after a release PR is merged.

The model is built around one rule: **the plugin's version manifest is the source of truth.** Branch names, tags, S3 keys, GitHub releases, and signed artifacts are all derived from the version recorded in `package.json` / `plugin.json`. They cannot disagree because one is computed from the other.

---

## 1. Repository Settings

### Branch protection rulesets

Configure rulesets covering two branch patterns: `main` and `maintenance/**`. Both share the same rules.

| Setting | Value | Why |
|---|---|---|
| Require a pull request before merging | On | No direct pushes to release-bearing branches |
| Required approvals | At least 1 (prefer 2) | Releases need a second pair of eyes |
| Dismiss stale pull request approvals when new commits are pushed | On | Prevents rc1 approval from silently covering rc5 |
| Require status checks to pass | On | Real validation gate |
| Required status checks → list | **One entry: `ci-required`** (source: GitHub Actions) | Single umbrella check that wraps all CI jobs |
| Require branches to be up to date before merging | On | Closes the divergence race at merge time |
| Block force pushes | On | Tags depend on stable history |
| Restrict deletions | On | Losing `maintenance/vX.Y.x` loses the release line |
| Required merge method | Merge commit only (if your ruleset version supports per-pattern restriction) | Preserves rc + merge-from-base history |
| Require linear history | **Off** | Linear history would force squash and break the merge-commit model |
| Require signed commits | Off (unless compliance requires) | Operationally heavy, not load-bearing for our model |

> The single required status check `ci-required` is the **umbrella job** defined in the CI workflow. It depends on every other CI job; if any dep fails or is skipped, `ci-required` fails. See [Pre-merge validation](#pre-merge-validation) for what the umbrella covers.

### Repository-wide settings

| Setting | Recommendation |
|---|---|
| Allow merge commits | Enabled |
| Allow squash merging | Enabled (used for feature → develop) |
| Allow rebase merging | Disabled (or team's choice) |
| Automatically delete head branches | Enabled (cleans up `release/*`, `hotfix/*`, `feature/*` after merge) |
| Always suggest updating pull request branches | Enabled (cosmetic nudge) |
| Allow auto-merge | Enabled (lets release engineers flip auto-merge once all checks are ready) |

### Merge method discipline

GitHub may not enforce per-branch merge method directly. The team-wide convention is:

| PR target | Allowed merge method |
|---|---|
| `main` | **Merge commit only** |
| `maintenance/**` | **Merge commit only** |
| `develop` | Squash (preferred) or merge commit |

Squash-merging a release PR collapses rc iteration history, the "merged main into release" commit, and conflict resolutions into a single opaque commit. Don't.

---

## 2. Branching Model

### Branch types

| Branch | Cut from | Targets PRs to | Version it carries | Lifetime |
|---|---|---|---|---|
| `develop` | — | — | `X.Y.Z-dev` (trunk) | Forever |
| `feature/*` | `develop` | `develop` | Inherits `X.Y.Z-dev` | Short |
| `release/vX.Y.Z` | `develop` | `main` | `X.Y.Z-rc1` → `X.Y.Z-rcN` → `X.Y.Z` | Short |
| `hotfix/vX.Y.Z` | `main` (or `maintenance/vX.Y.x`) | `main` (or `maintenance/vX.Y.x`) | `X.Y.Z-rc1` → `X.Y.Z-rcN` → `X.Y.Z` | Short |
| `maintenance/vX.Y.x` | `vX.Y.0` tag commit | — (only receives hotfix PRs) | n/a — long-lived base | Long, created lazily |
| `chore/back-merge-vX.Y.Z` | `main` | `develop` | Inherits develop's version | Short, auto-generated |

### Maintenance branches are created lazily

`maintenance/vX.Y.x` only exists if a hotfix is needed for an older line *after* a newer release has shipped. To create one:

```bash
# Example: v1.3.0 has shipped on main; a bug needs fixing in 1.2.x
git checkout -b maintenance/v1.2.x v1.2.0
git push -u origin maintenance/v1.2.x
```

The branch lives on indefinitely until the team stops supporting that minor version. Dormant maintenance branches stay around; nothing has to be cleaned up.

### Versioning rules

- **Manifest is source of truth.** The version in `package.json` (and propagated `plugin.json`) determines everything: zip filename, tag, S3 key, GH release.
- **Branch name encodes the base version.** `release/v1.2.3` ⇒ manifest base must be `1.2.3` (allowed manifest values: `1.2.3-rc1`, `1.2.3-rc2`, …, `1.2.3`).
- **rc cycle is in the manifest, not in tags.** rc iterations are `1.2.3-rc1` → `1.2.3-rc2` → … → `1.2.3`. No tags are pushed during the rc cycle.
- **Develop bumps at branch-cut time, not at back-merge time.** When you cut `release/v1.2.0` from develop, immediately bump develop's manifest to the next minor (`1.3.0-dev`). This frees develop for the next cycle's work and enables parallel rc iteration.

### Tag conventions

- `vX.Y.Z` on `main` for stable releases shipped from the trunk.
- `vX.Y.Z` on `maintenance/vX.Y.x` for maintenance hotfixes.
- **No tags for rc iterations.** rc artifacts exist on S3 but aren't tagged in git.

### Parallel releases

You **can** have multiple release branches open simultaneously, each iterating their own rc cycle. What you **cannot** do is land them on `main` out of order — semver requires that a higher version contain everything lower versions contain.

- Allowed: `release/v1.2.0` and `release/v1.3.0` both open, both running rc cycles.
- Enforced: `v1.2.0` must land on main before `v1.3.0`. If `v1.3.0` lands first, attempting to land `v1.2.0` fails the monotonicity check (manifest base must be > highest tag on the target line).
- If a release is blocked: either hold the higher one until it lands, or abandon the blocked release entirely.

### Hotfixes on a maintenance line

The branching for maintenance hotfixes uses the maintenance branch as the base, not `main`:

```bash
git checkout -b hotfix/v1.2.1 maintenance/v1.2.x
# bump manifest to 1.2.1-rc1, work, iterate
# open PR targeting maintenance/v1.2.x (NOT main)
```

This way the hotfix only includes 1.2.x's content plus the fix — not the newer features on `main`. When this PR merges to `maintenance/v1.2.x`, the post-merge job tags `v1.2.1` on the maintenance branch (not main).

Fixes that also apply to `main` are cherry-picked forward as a separate operation; there is **no automatic back-merge** from maintenance lines to develop.

---

## 3. Developer Workflow

### Starting a feature

```bash
git checkout develop
git pull
git checkout -b feature/short-description
# work, commit
git push -u origin feature/short-description
# open PR targeting develop
```

The PR runs validation (build, tests, lint). Merge with squash when approved.

### Cutting a release

When develop is ready to be released:

```bash
# 1. Cut the release branch from develop
git checkout develop
git pull
git checkout -b release/v1.2.0

# 2. Bump the manifest to the first rc
# (set-version script or manual edit — bumps 1.2.0-dev → 1.2.0-rc1)
./.github/scripts/set-version.sh 1.2.0-rc1
git commit -am "chore: cut release v1.2.0-rc1"
git push -u origin release/v1.2.0

# 3. IMMEDIATELY bump develop to the next minor
git checkout develop
./.github/scripts/set-version.sh 1.3.0-dev
git commit -am "chore: bump develop to 1.3.0-dev"
git push

# 4. Open PR: release/v1.2.0 -> main
gh pr create --base main --head release/v1.2.0 --draft \
  --title "Release v1.2.0" --body "rc cycle in progress"
```

Open the PR as a draft until the rc cycle completes. Each push triggers full CI; on success, the rc zip is uploaded to S3.

### Iterating an rc

```bash
# Work on fixes on the release branch
git checkout release/v1.2.0
# ... commits ...

# Bump rcN before pushing if the change is meaningful
./.github/scripts/set-version.sh 1.2.0-rc2
git commit -am "chore: bump to rc2"
git push
```

Each push runs the full pre-merge pipeline. If the manifest stayed at `1.2.0-rc2` for multiple pushes, the same S3 key is overwritten — latest sync wins.

### Keeping up with main while iterating

If another release or hotfix lands on `main` while your release is in flight, your branch is no longer up-to-date. Branch protection will block the merge button.

```bash
git checkout release/v1.2.0
git merge main
# resolve conflicts (often just package.json version)
# bump rcN if changes are non-trivial — re-validate
./.github/scripts/set-version.sh 1.2.0-rc3
git commit -am "chore: merge main into release, bump rc3"
git push
```

This creates a merge commit on the release branch (intentional — preserves history).

### Promoting to stable

When the rc cycle is done and ready to ship:

```bash
git checkout release/v1.2.0
./.github/scripts/set-version.sh 1.2.0
git commit -am "chore: promote to v1.2.0"
git push

# Mark PR as ready (out of draft)
gh pr ready
```

The PR runs the stable validation path (plugin-validator still runs; no S3 upload). When all checks are green, an approver merges with a merge commit.

### What happens when you merge a stable release PR

1. `ci-required` confirms green (all pre-merge checks passed).
2. Reviewer clicks "Merge pull request" (must be merge commit method).
3. The post-merge workflow (`pull_request_target` on closed+merged) fires:
   - Asserts manifest is clean (no `-rcN`).
   - Checks out the merge commit.
   - Rebuilds frontend + backend from the merge commit.
   - Packages the zip.
   - Signs it.
   - Runs the e2e suite against the **signed** artifact.
   - Uploads the signed zip to S3.
   - Creates a build-provenance attestation against the merge commit SHA.
   - Pushes tag `vX.Y.Z` on the merge commit.
   - Creates a **draft** GH release (manual promotion required).
4. The back-merge workflow opens a PR `chore/back-merge-vX.Y.Z` from `main` into `develop`. This PR carries the CHANGELOG section and rc fixes; it does not bump develop's manifest (that already happened at branch-cut time). Review and merge.

### Hotfix on the current release (main line)

Same shape as a release, but cut from `main`:

```bash
git checkout -b hotfix/v1.2.1 main
./.github/scripts/set-version.sh 1.2.1-rc1
# work, push, iterate rc cycle
# promote to clean 1.2.1, merge to main
```

A hotfix can ship without an rc cycle if urgency demands (commit clean `1.2.1` directly). The pipeline doesn't enforce minimum rc rounds; it just enforces correctness.

### Hotfix on a maintenance line

If the bug exists in an older minor that's still supported (e.g., `v1.2.0` has shipped, then `v1.3.0` has shipped, and now `v1.2.0` needs a patch):

```bash
# 1. Create the maintenance branch (if it doesn't exist yet)
git checkout -b maintenance/v1.2.x v1.2.0
git push -u origin maintenance/v1.2.x

# 2. Create the hotfix branch from maintenance/v1.2.x
git checkout -b hotfix/v1.2.1 maintenance/v1.2.x
./.github/scripts/set-version.sh 1.2.1-rc1
# work, push, iterate rc cycle

# 3. Open PR targeting maintenance/v1.2.x (NOT main)
gh pr create --base maintenance/v1.2.x --head hotfix/v1.2.1 \
  --title "Hotfix v1.2.1"
```

After merge:
- Tag `v1.2.1` is pushed on the maintenance branch's merge commit.
- The fix is **not** auto-back-merged anywhere. If the bug also exists on `main`, cherry-pick the fix forward via a separate PR.

### Common issues

| Problem | Likely cause | Fix |
|---|---|---|
| Merge button is greyed out | Branch is not up to date with target | `git merge <target>` and push |
| CI fails on "branch vs manifest" | Branch is `release/v1.2.0` but manifest says `1.3.0-rc1` | Fix the manifest or rename the branch |
| CI fails on "CHANGELOG missing section" | No `## [X.Y.Z]` heading in `CHANGELOG.md` | Add the section |
| CI fails on "monotonicity" | Manifest version is ≤ the latest tag on the target line | Bump the version |
| E2E suite fails | Plugin behavior regressed, or Grafana/ClickHouse fixture issue | Inspect `test-results/`; re-run locally via the `grafana-plugin-e2e` skill |
| `ci-required` is stuck "Pending" forever | Workflow didn't trigger, or job name doesn't match the ruleset | Check trigger config and job name |
| Post-merge job didn't tag | Manifest still had `-rcN` when merged | Bump to clean version before merging |

---

## 4. CI Pipeline

### Pre-merge validation

**Trigger:** `pull_request` to `main` or `maintenance/**`.

Every push to a release/hotfix branch (and every PR sync) runs:

1. **Branch ⇔ manifest version check** — `release/v1.2.0` requires manifest base to be `1.2.0`.
2. **CHANGELOG check** — `## [X.Y.Z]` heading must exist for the base version, with non-empty content.
3. **Ancestry check** — PR head must contain the target branch tip (defense in depth alongside the branch-protection rule).
4. **Monotonicity check** — manifest base version must be greater than the highest existing tag on the target line.
5. **Lint** — `npm run lint`, `golangci-lint run`.
6. **Typecheck** — `npm run typecheck`, `go vet ./...`.
7. **Tests** — `npm test -- --ci`, `go test -race ./...`.
8. **Build** — `npm run build` (frontend), `mage build` (backend) producing `dist/`.
9. **Package** — produces `<plugin-name>-<version>.zip`. The version carries the `-rcN` suffix during rc iteration.
10. **Plugin validator** — runs `@grafana/plugin-validator` against the packaged zip. Catches catalog-rejection issues at PR review time. Runs for **both rc and stable** PRs.
11. **E2E tests** — Playwright suite (`tests/*.spec.ts`) via the docker-compose `playwright` service. Exercises the packaged plugin in a real Grafana against ClickHouse fixtures. Runs against `dist/` from the same build the package step produced. Must pass before the rc S3 upload runs.
12. **S3 upload** — **only for rc** PRs (manifest has `-rcN`). Stable PRs do not upload to S3 pre-merge; that happens post-merge with the signed artifact. No prefix is used; the version in the filename distinguishes rc from stable. `needs:` the validator and e2e jobs — a failing e2e must not produce an uploadable artifact.

All steps above are dependencies of a single umbrella job:

```yaml
ci-required:
  needs: [ <every other CI job> ]
  if: always()
  runs-on: ubuntu-latest
  steps:
    - name: Assert all required jobs succeeded
      run: |
        results='${{ toJSON(needs) }}'
        if echo "$results" | grep -E '"result": *"(failure|cancelled|skipped)"' >/dev/null; then
          exit 1
        fi
```

`ci-required` is the **only** required check listed in the branch protection ruleset. Adding/renaming/removing other jobs in the workflow does not require ruleset changes — only updating `needs:`.

### Post-merge release

**Trigger:** `pull_request_target` on `closed` with `merged == true`, targeting `main` or `maintenance/**`.

Runs only when the manifest is clean (no `-rcN`). Steps in order (tag-last + idempotent):

1. **Assert manifest is clean** — defense-in-depth check that nothing slipped past pre-merge.
2. **Checkout merge commit** — `ref: ${{ github.event.pull_request.merge_commit_sha }}`. This is the actual commit on the target branch, not the PR head.
3. **Rebuild** — frontend + backend from the merge commit. Don't reuse pre-merge artifacts; main may have moved during the PR's life and the rebuilt artifact must match the tag commit's tree.
4. **Package** — produces `<plugin-name>-<version>.zip` (no rc suffix this time).
5. **Sign** — Grafana access policy token. Produces a signed `MANIFEST.txt` inside the zip.
6. **E2E against the signed artifact** — extract the signed zip into a clean directory, mount it into Grafana via docker-compose, run the Playwright suite. Smoke-tests that signing did not perturb runtime behavior (the signed zip carries `MANIFEST.txt` and is the exact artifact users will install). Must pass before any publishing step runs.
7. **Upload to S3** — signed zip lands on the configured S3 path (no prefix).
8. **Attestation** — `actions/attest-build-provenance@v2` creates a sigstore attestation referencing the merge commit SHA. Verifiable later via `gh attestation verify` or the GitHub attestations API.
9. **Push tag** — `vX.Y.Z` on the merge commit. Push *last* so retries before this step are safe (no orphan tags).
10. **Draft GH release** — created against the new tag with body parsed from `CHANGELOG.md`. Manual promotion to "published" is required (intentional human gate).

Each step is idempotent: re-running the job after a transient failure skips work already done.

### Back-merge automation

After a release lands on `main`, a workflow opens a PR `chore/back-merge-vX.Y.Z` from `main` into `develop` containing:

- The release branch's CHANGELOG additions.
- Any rc-iteration fixes.
- Any hotfix changes that happened on main during the release window.

The back-merge PR does **not** modify the manifest version on develop — that bump already happened when the release branch was cut.

For maintenance hotfixes, no automatic back-merge is generated. The team manually cherry-picks fixes forward when applicable.

### Workflow permissions

The post-merge release job needs these permissions:

```yaml
permissions:
  contents: write        # push the tag, create the GH release
  id-token: write        # OIDC for sigstore attestation
  attestations: write    # write the attestation record
```

If `package.yml` is invoked as a reusable workflow, **both** the caller and the callee must declare `id-token: write` and `attestations: write` — the callee's effective permissions are bounded by the caller's.

The signing step requires the `GRAFANA_ACCESS_POLICY_TOKEN` secret. Tag push and release creation use the default `GITHUB_TOKEN` with `contents: write`.

---

## 5. Quick Reference

### Version state per branch

| Branch | Version at any given time |
|---|---|
| `develop` | `(next-release).0-dev` |
| `release/vX.Y.Z` | `X.Y.Z-rcN` while iterating, `X.Y.Z` when promoting |
| `hotfix/vX.Y.Z` | `X.Y.Z-rcN` while iterating, `X.Y.Z` when promoting |
| `maintenance/vX.Y.x` | Carries whatever was last released on this line (no manifest dev state) |
| `main` | Last released stable version |

### Where each version lives

| Artifact | Format | Where |
|---|---|---|
| Manifest version | `X.Y.Z` or `X.Y.Z-rcN` | `package.json` + `src/plugin.json` |
| rc zip | `<plugin>-X.Y.Z-rcN.zip` | S3 (overwrites on resync) |
| Stable zip (signed) | `<plugin>-X.Y.Z.zip` | S3 |
| Tag | `vX.Y.Z` | `main` (stable from release) or `maintenance/vX.Y.x` (stable from hotfix) |
| GitHub release | Draft, body from CHANGELOG | Promoted manually after review |
| Attestation | Sigstore record | GitHub attestations API, queryable per commit SHA |

### CHANGELOG conventions

- One heading per released version: `## [X.Y.Z]` (or `## vX.Y.Z`, depending on `parse-changelog.sh` expectations).
- No per-rc headings — the same `## [X.Y.Z]` section accumulates notes throughout the rc cycle.
- An optional `## [Unreleased]` section at the top is fine.

### Useful commands

```bash
# Verify a stable release artifact's provenance
gh attestation verify <plugin>-X.Y.Z.zip --owner <org>

# List rc artifacts on S3
aws s3 ls s3://<bucket>/ | grep -- '-rc'

# See what version a branch is at
jq -r .version package.json

# Check whether a release branch is up to date with main
git fetch origin main
git merge-base --is-ancestor origin/main HEAD && echo "up to date" || echo "behind"
```

---

## Summary diagram

```
develop (1.3.0-dev)
   │
   │  (cut release; bump develop to 1.4.0-dev immediately)
   ├─────────────────► release/v1.3.0 (1.3.0-rc1 → rc2 → ... → 1.3.0)
   │                       │
   │                       │ pre-merge CI: validate, build, test, package, validator, S3 (rc only)
   │                       │
   │                       ▼ (PR merged via merge commit)
   │                   main ──[tag v1.3.0]── post-merge: rebuild, sign, S3, attest, draft release
   │                       │
   │ ◄─── back-merge PR ───┘
   │ (chore/back-merge-v1.3.0: brings CHANGELOG + rc fixes into develop)
   │

main (released as v1.3.0)
   │
   │  (need to patch older v1.2.0?)
   │  cut maintenance/v1.2.x from v1.2.0 tag (lazy, only if needed)
   │
   └── maintenance/v1.2.x
            │
            └── hotfix/v1.2.1 (1.2.1-rc1 → ... → 1.2.1)
                    │
                    ▼ (PR merged via merge commit, target = maintenance/v1.2.x)
                maintenance/v1.2.x ──[tag v1.2.1]── post-merge: same as stable
                    │
                    └── (no auto back-merge; cherry-pick forward to main if applicable)
```