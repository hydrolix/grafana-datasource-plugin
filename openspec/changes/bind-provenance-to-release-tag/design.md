## Context

`ci.yml` runs on `pull_request` and does everything in one run: build frontend and backend, `package.yml` sets the version, signs with `@grafana/sign-plugin`, zips, and conditionally attests; then `e2e`, `plugin-validator`, `publish-aws`, and `create-github-release` follow, the last of which creates the `v<version>` tag from the PR head SHA.

Because the attesting run is triggered by `pull_request`, its OIDC token carries `ref = refs/pull/<N>/merge`, and `actions/attest-build-provenance` copies that into the provenance statement. Verified on the current release:

```
$ gh attestation verify hydrolix-hydrolix-datasource-0.11.0.zip --repo hydrolix/grafana-datasource-plugin
EXIT=0
subject:        hydrolix-hydrolix-datasource-0.11.0.zip
                sha256:7febcc57f1eb93f90497b07cd48923624137cccf1e58c8717b1507bbc6e6fb3a
sourceRef:      refs/pull/164/merge
buildSignerURI: .../package.yml@refs/pull/164/merge
```

The digest matches the published asset, so the artifact genuinely is attested — but `refs/pull/164/merge` is ephemeral and no longer resolves, so a tag-scoped verifier reports no provenance. The tag is created *after* packaging, and nothing is ever built while a tag ref is checked out.

Distribution is a side-effect of the same PR run: `publish-aws` is gated on `base_ref == 'main'`, so the only way to get any build to S3 is to open a PR into `main`. RCs used exactly that — `rc/v0.11.0` carried a `-dev` version and shipped as a draft release tagged `v0.11.0-dev+5a33906c`. Moving publication to tags removes that path, so pre-release distribution must be redesigned rather than merely relocated.

Two constraints shape the design. The provenance ref is taken from the run's OIDC claims and cannot be supplied as an action input, so binding to a tag requires the build to *run* on the tag. And GitHub suppresses workflow triggers for refs created with `GITHUB_TOKEN`, so an automatically created tag does not by itself start a tag-triggered workflow — while a manually pushed tag does.

## Goals / Non-Goals

**Goals:**

- The ZIP published to the GitHub release and S3 is built, signed, and attested in a run whose source ref is `refs/tags/v<version>`.
- Tag, `package.json` version, and attestation subject filename agree, enforced in CI rather than by convention.
- Provenance is verifiable after the fact from the published asset alone.
- The tree CI exercised is the tree that gets tagged, built, and attested.
- The shipped ZIP is exercised end to end against every supported Grafana version before it is published.
- Releasing stays a single maintainer action (merge the release PR); no manual tag pushing required for releases.
- Dev and RC builds are distributable from S3 through explicit, correctly versioned tags — no PR side-effects, no mislabeled `-dev` drafts.
- PR runs keep packaging so `e2e` and `plugin-validator` are unaffected, and publish nothing.

**Non-Goals:**

- Reproducible builds. `Magefile.go` embeds `time.Now().Unix()` in `buildInfoJSON`, so two builds of the same commit differ; this change does not pursue bit-for-bit reproducibility.
- Migrating to `grafana/plugin-actions/build-plugin` wholesale. We align with its trigger model (tag-driven) while keeping our own signing, S3, and changelog steps.
- Re-attesting or back-filling provenance for already-published releases.
- Changing what `-dev` PR builds produce for e2e and the validator.
- Automatic dev publication on merges to `develop` — dev distribution is a deliberate per-commit act via a tag.

## Decisions

### D1: Build on the tag rather than promote the PR-built artifact

The release workflow checks out the tag and rebuilds, rather than downloading the PR run's `plugin-package` artifact and attesting it.

Rationale: an attestation's source ref comes from the OIDC token of the run that produces it. There is no way to re-bind an existing attestation, and attesting a downloaded artifact in a tag-triggered run would still be honest but would attest bytes this run did not build — exactly the property provenance exists to rule out.

*Alternative considered*: keep the PR build and pass tag metadata to `actions/attest-build-provenance`. Rejected — the action has no such input; `sourceRepositoryRef` is derived from the token, not parameters.

### D2: `package.yml` gains an `attest` input instead of an event-based gate

The `github.event_name == 'pull_request' && github.base_ref == 'main'` condition on the attestation step is replaced by a boolean `attest` workflow input, defaulted to `false` and set to `true` only by the release (`vX.Y.Z`) channel.

Rationale: the reusable workflow should not infer intent from the calling event. PR, dev, and RC runs all need the packaged ZIP but must not emit attestations — an input says that directly, and the condition stops being a subtle correctness dependency on trigger shape.

### D3: Tag on merge to `main`, then dispatch the release workflow at the tag ref — but only for a newly created tag

A new workflow on `push: branches: [main]` reads the version from `package.json`, creates `v<version>`, and then dispatches the release workflow with `--ref refs/tags/v<version>`. The release workflow declares both `workflow_dispatch` and `push: tags: ['v*']`. If the version is not a plain `X.Y.Z`, or the tag already exists, the job stops without creating or dispatching anything.

Rationale: GitHub does not fire workflow triggers for tags created with `GITHUB_TOKEN`, so `on: push: tags` alone would silently never run for releases. An explicit dispatch at the tag ref makes `github.ref` the tag, which is all the provenance binding needs, and requires only `actions: write` — no new secret. The guards exist because the workflow fires on *every* push to `main`: without the created-guard, a version-less push (docs fix, revert) would re-dispatch the release for an existing tag pointing at an older commit and re-publish a released version; without the shape guard, a `-dev` version accidentally landing on `main` would mint a permanent junk tag that matches no channel. Keeping `push: tags` as a second trigger means a manually pushed tag also releases, which is both the recovery path and the normal path for dev and RC tags — those are pushed by a human, so their push events fire naturally and need no dispatch.

*Alternative considered*: create the tag with a PAT or GitHub App token so `on: push: tags` fires naturally. Rejected — it introduces a long-lived, broadly scoped org secret for no functional gain over dispatch.

### D4: `set-version.sh` recognizes the three tag shapes, and every tag must agree with `package.json`

The script gains a tag branch for `GITHUB_REF_NAME` matching one of three shapes, each with an agreement check that fails the run before anything is signed:

- `vX.Y.Z` (release): assert `package.json` version equals `X.Y.Z`; keep it as-is.
- `vX.Y.Z-rc.N` (RC): assert `package.json` version equals `X.Y.Z` (the release branch carries the plain version); rewrite it to `X.Y.Z-rc.N`.
- `vX.Y.Z-dev+<sha8>` (dev): assert `package.json` version equals `X.Y.Z-dev` **and** `<sha8>` is a prefix of the commit's full SHA (`git rev-parse HEAD`); rewrite it to `X.Y.Z-dev+<sha8>`.

Rationale: today a tag ref matches neither `^release[/_-]` nor `^hotfix[/_-]`, so the script falls through to the `-dev` branch and rejects a valid release version — the tag path cannot work without this. The agreement checks exist because the ZIP filename (which carries the `package.json` version) is what S3 consumers and the attestation subject see; if it disagrees with the tag the artifact is misleadingly named. The dev tag's embedded SHA is redundant with the commit the tag points at and can therefore lie — asserting it against `HEAD` makes filename, tag, and commit unable to drift. The comparison is a prefix match against the full SHA rather than equality with `git rev-parse --short=8`, which returns *at least* eight characters and silently lengthens when a prefix becomes ambiguous — an equality check would then reject an honest tag.

### D5: Publication channels are selected by tag shape; dev and RC are S3-only and unattested

The tag-triggered workflow branches on the tag's shape. Dev tags publish the signed ZIP to S3 under `dev/`; RC tags publish under `rc/`; release tags publish the GitHub release plus S3's existing flat path (unchanged for current consumers, selected via a new path-prefix input on `s3_publish.yml`). Neither dev nor RC creates a GitHub release or an attestation; all three channels sign, since Grafana will not load an unsigned plugin.

The release channel additionally fails before creating the GitHub release when the parsed changelog entry for the version is empty, and all publication workflows declare a top-level `permissions: contents: read` block with jobs elevating explicitly.

Channel selection also requires branch containment, checked in the classify job by ancestry against freshly fetched branch refs before anything builds or signs: a release tag's commit must be contained in `origin/main`, an RC tag's in at least one `origin/release[/_-]*` or `origin/hotfix[/_-]*` branch, and a dev tag's in `origin/develop`. Shape alone would let anyone able to push a `v*` tag publish from any commit; containment makes each channel a statement about process — released code reached `main`, RC code sits on a release or hotfix branch, dev builds are merged work — and turns D6's premise that a dev-tagged commit already passed PR e2e on `develop` from convention into fact. For auto-created release tags the check is redundant by construction; it exists for the manually pushed recovery path. Hotfix branches are admitted for RCs so a hotfix build can be verified from S3 before merging to `main`.

Rationale: attestation exists to satisfy Grafana's release verifier, which only ever sees release ZIPs; keeping it release-only preserves the invariant that the attestation store contains exactly the shipped releases, so nothing a verifier finds there can point at bytes that were never released. The plugin signature already covers authenticity for anyone handed a dev or RC build. GitHub prereleases for RCs would have no consumer — the rendered changelog surface matters only for public releases. The changelog gate exists because the tag path is now the only publish path, so an empty entry would otherwise ship blank release notes; the permissions default is cheap hardening for workflows holding signing and AWS secrets.

*Alternatives considered*: publish dev builds on every push to `develop` — rejected: it publishes every merge whether wanted or not, and dev distribution should be a deliberate act with an S3 footprint to match. Publish RCs as GitHub prereleases — rejected: no consumer, and it would re-introduce release objects for artifacts Grafana never sees.

### D6: Validate and e2e the tag-built package on the RC and release channels; dev skips both

RC and release runs execute `plugin-validator` and the full Playwright matrix — Grafana 10.4.18, 11.6.1, 12.0.2, and 13.0.1 — against the ZIP they just built, and publish only if both pass. Dev runs publish after signing and the version agreement check alone. The PR run keeps its own e2e for pre-merge feedback.

Rationale: `e2e.yml` is already a `workflow_call` workflow that downloads the `plugin-package` artifact by name, unzips it, and drives a real Grafana against it, so reusing it on the tag path costs one `uses:` block. Because D1 makes the tag-built ZIP the shipped artifact, this is the only place its exact bytes are exercised — including whether the Grafana signature is accepted at plugin *load* time, which `plugin-validator` inspects statically but only a running Grafana proves. RCs get the full gate because they are the release dress rehearsal. Dev builds skip it because the tagged commit already passed PR e2e on `develop` and the channel exists for fast internal hand-off.

*Alternative considered*: skip e2e on the release tag build because the PR run already exercised the same commit. Rejected — the PR run exercises `refs/pull/<N>/merge` rather than the merge commit that gets tagged, and it exercises a different ZIP (D1); with the reusable workflow already in place, the argument from cost does not hold.

### D7: Post-publish verification round-trips the published copies, not the workspace ZIP

After publishing, the release channel downloads the GitHub release asset back and runs `gh attestation verify` on it, asserting `sourceRepositoryRef == refs/tags/v<version>`, then asserts the S3 copy's sha256 equals the verified asset's digest. RC and dev channels download their S3 object back over the same public HTTPS URL consumers are given and assert its sha256 equals the built ZIP's. Any mismatch fails the run.

Rationale: the whole change exists to satisfy an external verifier, and that verifier fetches the GitHub release asset — encoding its exact question as a job means the pipeline cannot regress into a PR-ref attestation without turning red. Verifying the *downloaded* copies rather than the local file is the point: an attestation binds bytes, not a location, and each upload can independently truncate, misfile, or overwrite; the round-trip proves what each endpoint actually serves. Fetching over the public URL rather than an authenticated `aws s3 cp` also proves the object is actually reachable by consumers — bucket policy, region, and URL encoding included.

The verification logic lives in a script under `.github/scripts/` rather than inline workflow YAML, so it is shellcheck-covered and can be smoke-tested locally against an already-published release — proving the attestation-JSON field path before the first tag-driven release depends on it. Run against the 0.11.0 asset, the expected outcome is a failure *at the source-ref assertion* that prints `refs/pull/164/merge`: extraction works, and the ref genuinely is not the tag.

### D8: `ci.yml` drops all publication

`create-github-release` and `publish-aws` are removed from `ci.yml` along with their `base_ref == 'main'` conditions, `release.yml` loses its tag-creation step, and the `draft: contains(version, 'dev')` conditional goes with it — no event other than a `v*` tag publishes anything.

Rationale: with publication tag-driven, those jobs would either double-publish or sit permanently dead. Leaving conditional-but-unreachable release jobs in the PR pipeline is how the current confusion arose. The same reasoning removes `package.yml`'s `version` workflow output: its only consumer was `ci.yml`'s release job, and an unused output on a reusable workflow reads as load-bearing API and fossilizes.

### D9: A pull request into `main` must already contain `main`'s tip

A job running on `pull_request` when the base branch is `main` fetches `origin/main` fresh and fails unless `git merge-base --is-ancestor <origin/main> <pull_request.head.sha>` succeeds.

Rationale: the tag is cut from `main`'s tip after the merge, and the `release target branches` ruleset restricts `main` to `allowed_merge_methods: ["merge"]`, so every release lands as a merge commit. If the release branch is behind `main`, that merge commit carries a tree nobody built: the PR run exercised `merge(head, main@T1)`, and a `pull_request` run is not re-triggered when the *base* advances, so a merge at T2 ships `merge(head, main@T2)`. Requiring `main ⊆ head` makes the merge commit's tree equal the head's tree, so the tree CI exercised is the tree that gets tagged, built, and attested. It also catches a hotfix that landed on `main` but was never back-merged before the release branch was cut.

Two details are load-bearing. The comparison must use a freshly fetched `origin/main`, not `github.event.pull_request.base.sha`, which is only the base tip as of the last PR event. And it must use `pull_request.head.sha`, not `HEAD` — on a `pull_request` event `actions/checkout` lands on the merge ref, which contains the base tip by construction and would make the assertion vacuously true.

*Alternative considered*: rely solely on branch protection's "require branches to be up to date" (`required_status_checks.strict`). Rejected as the *only* mechanism — it is a repo setting rather than reviewable code, and it cannot currently apply here at all: `main`'s protection reports `strict: false` with empty `contexts` and `checks`, so there is no required check for strict mode to gate on. It remains the correct enforcement layer, which is why this decision pairs the in-repo check with registering it as a required context.

## Risks / Trade-offs

- **[The dispatched release run never starts, leaving a tag with no release]** → Tag creation and dispatch live in the same job, so a dispatch failure fails that job visibly; the release workflow is re-runnable on an existing tag and a maintainer can also push the tag by hand. *Proving signal*: the tag-creation job fails red on dispatch error, and the release run's summary prints `github.ref`.
- **[A version-less or non-release-version push to `main` re-dispatches a release or mints a junk tag]** → D3's guards: the job creates and dispatches only for a plain `X.Y.Z` version whose tag it created in this run. *Proving signal*: a docs-only push (or a push carrying a `-dev` version) to `main` leaves the Actions log with a tag-creation run that skipped, no new tag, and no release run.
- **[The shipped ZIP is not the bytes any test ran against]** → Resolved by D6: the tag-built ZIP is the one `plugin-validator` and the Playwright matrix run against, and the one the attestation covers. *Proving signal*: the `e2e` and `plugin-validator` jobs in the release run, plus the attestation digest matching the published asset.
- **[e2e or validation fails on the tag build, leaving a tag with no release]** → An unreleased tag is inert; the release workflow is idempotent and re-runnable on the same tag once the fix lands, or the tag can be deleted and re-pushed. *Proving signal*: a re-run on the existing tag reaches the publish steps and produces the same version.
- **[A dev tag's embedded SHA lies about the commit it points at]** → D4 asserts the tag's `<sha8>` is a prefix of the tagged commit's full SHA and fails the run. *Proving signal*: the script test case tagging one commit with another commit's SHA exits non-zero.
- **[The ancestry check goes stale — it passes, then `main` advances before the merge button is pressed]** → The check re-runs on `synchronize` but not when the base moves, so in-repo it is fast feedback rather than a hard gate; enforcement comes from registering it as a required status check on `main` with `strict: true`, which GitHub re-evaluates at merge time. *Proving signal*: with strict mode on, a PR whose base advanced shows "This branch is out-of-date" and the merge button is disabled.
- **[Requiring `main ⊆ head` forces a re-sync and a full CI re-run on release PRs whenever `main` moves]** → Accepted. `main` advances only on releases and hotfixes, so the collision window is narrow. *Proving signal*: none needed — it surfaces as one extra CI run on the release PR.
- **[`set-version.sh` changes break `-dev` PR builds]** → The new tag branch is additive and the existing branch conditions are untouched. *Proving signal*: a script-level test job exercising the matrix of `GITHUB_REF_NAME` / `GITHUB_HEAD_REF` values (release branch, hotfix branch, dev PR, each tag shape, each mismatch) and asserting the resulting version or non-zero exit.
- **[Both `workflow_dispatch` and `push: tags` fire and publish twice]** → A `concurrency` group keyed on the tag with `cancel-in-progress: false` serializes them, and the release step is idempotent for a given tag. *Proving signal*: a duplicate run appears as skipped or no-op in the Actions log rather than as a second set of release assets.
- **[Tag/version drift produces a misleadingly named artifact]** → D4's agreement assertions fail the run before signing on every channel. *Proving signal*: the script test cases with mismatched tag/version pairs exit non-zero.
- **[RCs have no rendered changelog surface without a GitHub release]** → Accepted. RCs are internal; the S3 URL is the hand-off, and the changelog ships with the eventual release.
- **[An RC publish cannot be re-run after its release branch is deleted]** → Accepted: the containment check fails once the branch is gone; the rc tag stays inert and the eventual release supersedes it. Documented in `docs/releasing.md`. *Proving signal*: the classify job fails naming the missing containing branch.
- **[Release wall-clock grows because the artifact is built twice]** → Accepted. The PR build stays for e2e; the tag build is the shipped one. No mitigation beyond D6's decision not to re-run e2e on dev tags.

## Migration Plan

1. Land the workflow and script changes on `develop`; PR runs exercise `package.yml` with `attest: false` and the unchanged `-dev` version path.
2. Verify the script test job covers the tag matrix, and that the ancestry check fails a deliberately stale test PR into `main` and passes once `main` is merged in.
3. Rehearse the dev channel: push a `vX.Y.Z-dev+<sha8>` tag on a `develop` commit, confirm the run signs, skips validator/e2e/attestation, publishes to S3 `dev/`, and the digest round-trip passes; then delete the tag and object.
4. Rehearse the RC channel from the next release branch: push `vX.Y.Z-rc.1`, confirm validator plus the full e2e matrix gate the `rc/` upload and no GitHub release or attestation is created.
5. Register the ancestry check (and the PR `e2e` job) as required status checks on `main` and set `required_status_checks.strict = true`.
6. On the first release PR to `main`, confirm the tag-creation job creates `v<version>` and dispatches the release workflow at that ref.
7. Confirm the release run's e2e, validation, and verification gates pass, then re-run `gh attestation verify` locally against the published asset as an independent check.
8. Resubmit to Grafana and confirm `no-provenance-attestation` is gone. Retire the `rc/*` branch pattern in the team's release docs.

Rollback: revert the workflow commit and re-add the release jobs to `ci.yml`. Already-created tags are harmless — a tag with no release is inert, and re-running the release workflow on it is safe. No plugin artifact already in Grafana's catalog is affected.

## Open Questions

- Enabling `required_status_checks.strict` and registering required contexts on `main` is a repository-settings change, not a file in this repo; the `release target branches` ruleset also grants repository admins `always` bypass. Confirm who makes that change and whether admin bypass should be narrowed for release PRs.
