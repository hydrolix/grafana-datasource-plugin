## 1. Version resolution on tag refs

- [x] 1.1 Add a tag branch to `.github/scripts/set-version.sh` for `GITHUB_REF_NAME` matching `^v[0-9]+\.[0-9]+\.[0-9]+$`, asserting the `package.json` version equals the tag minus its leading `v` and keeping it as-is; exit non-zero with a message naming both values on mismatch
- [x] 1.2 Add the RC shape `^v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+$`: assert `package.json` equals the tag's `X.Y.Z` base, then rewrite the version to `X.Y.Z-rc.N`
- [x] 1.3 Add the dev shape `^v[0-9]+\.[0-9]+\.[0-9]+-dev\+[0-9a-f]{8}$`: assert `package.json` equals `X.Y.Z-dev` and the tag's `<sha8>` equals `git rev-parse --short=8 HEAD` (message naming both SHAs on mismatch), then rewrite the version to `X.Y.Z-dev+<sha8>`
- [x] 1.4 Confirm the existing `release`/`hotfix` branch paths and the `X.Y.Z-dev` + SHA PR path are untouched by re-reading the script's branch conditions

## 2. Unit coverage for version resolution

- [x] 2.1 Add `.github/scripts/set-version.test.sh` running the script against a temporary `package.json` and git fixture for each case: `v0.12.0` tag with matching version, `v9.9.9` tag mismatched, `v0.12.0` tag against a `-dev` version, `v0.12.0-rc.1` with matching base, `v0.12.0-rc.1` with mismatched base, `v0.12.0-dev+<sha8>` matching HEAD, dev tag with a wrong SHA, dev tag against a non-dev version, `release/v0.12.0` head ref, `hotfix/v0.12.1` head ref, and a plain feature-branch dev build
- [x] 2.2 Assert the resulting version string (or non-zero exit) for each case, and make the test script exit non-zero if any case fails
- [x] 2.3 Wire the test into `ci.yml` as a job so it gates every PR

## 3. Attestation gating in `package.yml`

- [x] 3.1 Add a boolean `attest` input to `.github/workflows/package.yml`'s `workflow_call` block, defaulting to `false`
- [x] 3.2 Replace the `github.event_name == 'pull_request' && github.base_ref == 'main'` condition on the `actions/attest-build-provenance` step with a check on the `attest` input alone
- [x] 3.3 Verify the `package` job still declares `id-token: write` and `attestations: write` so a caller passing `attest: true` can sign

## 4. S3 publication prefix

- [x] 4.1 Add an optional `path_prefix` input (default empty) to `.github/workflows/s3_publish.yml`, passed through to the publish script
- [x] 4.2 Parameterize `.github/scripts/publish-aws.sh` with the prefix so the object key becomes `grafana-datasource-plugin/<prefix><zip>`, keeping the unprefixed release layout byte-identical to today
- [x] 4.3 Add a digest round-trip to the publish flow: download the uploaded object back and exit non-zero unless its `sha256` equals the local ZIP's, echoing the object URL and digest into the run summary

## 5. Tag creation on merge to `main`

- [x] 5.1 Add `.github/workflows/tag-release.yml` triggered on `push` to `main`, reading the version from `package.json`
- [x] 5.2 Create the tag `v<version>` at the pushed commit only when it does not already exist; when it exists, skip tag creation and dispatch so a version-less push to `main` cannot re-release
- [x] 5.3 Dispatch the release workflow at `refs/tags/v<version>` in the same job, only when this run created the tag, so a dispatch failure fails visibly
- [x] 5.4 Grant the job `contents: write` and `actions: write`, and nothing more

## 6. Base-`main` ancestry gate

- [x] 6.1 Add a job to `ci.yml` that runs only when `github.base_ref == 'main'`, checking out with `fetch-depth: 0` and `persist-credentials: false`
- [x] 6.2 Fetch `origin/main` during the run and fail unless `git merge-base --is-ancestor <origin/main tip> ${{ github.event.pull_request.head.sha }}` succeeds
- [x] 6.3 On failure, print the `main` commit missing from the source branch and the command to resolve it (`git merge origin/main`)
- [x] 6.4 Confirm the check is evaluated against `pull_request.head.sha`, not the checked-out merge-ref `HEAD`, and against a freshly fetched `main`, not `pull_request.base.sha`
- [x] 6.5 Verify the job is skipped for pull requests targeting `develop`

## 7. Tag-triggered channel workflow

- [x] 7.1 Convert `.github/workflows/release.yml` to trigger on `workflow_dispatch` and `push` of tags matching `v*`, remove its tag-creation step, and add a `concurrency` group keyed on the tag with `cancel-in-progress: false`
- [x] 7.2 Classify the tag as dev / rc / release by shape in a first job whose outputs gate all later jobs; fail the run on any other shape before signing or publishing
- [x] 7.3 Build the frontend and backend within the run by calling `build-frontend.yml` and `build-backend.yml`; do not download `pull_request` artifacts
- [x] 7.4 Call `package.yml` with the Grafana signing token, passing `attest: true` only for the release shape
- [x] 7.5 For rc and release shapes, call `validate.yml` and `e2e.yml` against the freshly built `plugin-package` artifact, keeping the Grafana 10.4.18 / 11.6.1 / 12.0.2 / 13.0.1 matrix, and gate publication on both; the dev shape skips both
- [x] 7.6 Publish per channel: dev → `s3_publish.yml` with `path_prefix: dev/`; rc → `s3_publish.yml` with `path_prefix: rc/`; release → GitHub release from the tag with the parsed changelog, then `s3_publish.yml` with no prefix
- [x] 7.7 On the release shape, after publishing, re-download the release asset with `gh release download`, run `gh attestation verify <zip> --repo hydrolix/grafana-datasource-plugin --format json`, assert `sourceRepositoryRef == refs/tags/v<version>`, and assert the S3 copy's `sha256` equals the verified asset's digest, failing the run otherwise
- [x] 7.8 Echo the channel, verified source ref (release only), and artifact digest into the run summary

## 8. Retire publication from the pull-request pipeline

- [x] 8.1 Remove the `create-github-release` and `publish-aws` jobs from `.github/workflows/ci.yml`
- [x] 8.2 Remove the now-unused `base_ref == 'main'` publication conditions and confirm no remaining `ci.yml` job creates tags, releases, or S3 objects
- [x] 8.3 Confirm `e2e` and `plugin-validator` still consume the PR `package` job's artifact unchanged
- [x] 8.4 Drop the `draft: contains(inputs.version, 'dev')` conditional from the release step, since the release shape admits only `X.Y.Z`

## 9. Review hardening

- [x] 9.1 Guard `tag-release.yml`: skip tag creation and dispatch (exit 0 with a message naming the version) unless the `package.json` version matches `^[0-9]+\.[0-9]+\.[0-9]+$`
- [x] 9.2 In `set-version.sh`'s dev tag path, compare the tag's `<sha8>` as a prefix of `git rev-parse HEAD` (full SHA) instead of equality with `git rev-parse --short=8 HEAD`, which lengthens on ambiguity; keep the failure message naming both values
- [x] 9.3 In `publish-aws.sh`, perform the digest round-trip by downloading the object over the public HTTPS URL (`curl`) instead of authenticated `aws s3 cp`, so the check also proves public reachability and URL encoding
- [x] 9.4 Add a workflow-level `permissions: contents: read` block to `release.yml`, `tag-release.yml`, and `ci.yml`; confirm every job needing more (tagging, releasing, attesting, compatibility-check comments) already elevates in its own `permissions` block
- [x] 9.5 Fail the release channel before creating the GitHub release when `parse-changelog.sh` returns an empty entry for the version
- [x] 9.6 Unify version reads on `jq -r .version` in `set-version.sh`, replacing the grep/sed parse
- [x] 9.7 Single-source the S3 bucket/public base URL shared by `publish-aws.sh` and `release.yml`'s `verify-release` job (e.g. a small config file under `.github/scripts/` both read), so a bucket change is one edit
- [x] 9.8 Add `docs/releasing.md` documenting the three channels, including the dev-tag one-liner (`git tag "v$(jq -r .version package.json)+$(git rev-parse --short=8 HEAD)"`), the RC flow from a release branch, and the automatic release flow
- [x] 9.9 Extend `.github/scripts/set-version.test.sh` if any case's setup depends on the changed SHA comparison or version parsing, and re-run the full harness plus `bash -n`/shellcheck/actionlint on every touched file
- [x] 9.10 Extract `verify-release`'s attestation and S3 verification logic into `.github/scripts/verify-release-artifacts.sh` (tag as argument, repo from `GITHUB_REPOSITORY` or a flag, sourcing `s3-config.sh`, portable sha256, summary written only when `GITHUB_STEP_SUMMARY` is set); the workflow job becomes checkout + one script invocation
- [x] 9.11 Smoke-test the script locally against the published `v0.11.0` release: expect it to fail at the source-ref assertion printing `refs/pull/164/merge` — proving the attestation-JSON field path extracts correctly — after the download, `gh attestation verify`, and S3 digest steps succeed
- [x] 9.12 Remove `package.yml`'s unused `version` workflow output and the job/step output plumbing that exists only to feed it, after confirming nothing else consumes it
- [x] 9.13 Add a branch-containment gate to `release.yml`'s classify job: checkout with `fetch-depth: 0` and `persist-credentials: false`, peel the tag to its commit (`git rev-parse "<tag>^{commit}"` — annotated tags make `GITHUB_SHA` a tag object), then require `git merge-base --is-ancestor` containment in `origin/main` (release), at least one `origin/release[/_-]*` or `origin/hotfix[/_-]*` branch (rc), or `origin/develop` (dev); fail before any build or signing with a message naming the tag, commit, and required branch
- [x] 9.14 Document the containment rules in `docs/releasing.md`, including the accepted edge that an RC publish cannot be re-run after its release branch is deleted
- [x] 9.15 Re-run `actionlint` on `release.yml` and `bash -n`/shellcheck on any script the gate adds or touches

## 10. Channel rehearsals

- [ ] 10.1 Dev: push a `v<X.Y.Z>-dev+<sha8>` tag on a `develop` commit; confirm the run signs, skips validator/e2e/attestation, uploads under `dev/`, and the digest round-trip passes; then delete the tag and object
- [ ] 10.2 Dev negative: push a dev tag with a wrong embedded SHA and confirm the run fails before any upload
- [ ] 10.3 RC: from a release branch, push `v<X.Y.Z>-rc.1`; confirm validator plus the full e2e matrix gate the `rc/` upload, and that no GitHub release and no attestation are created
- [ ] 10.4 Release: on a scratch setup, exercise the release shape end to end and confirm the attestation reports `sourceRepositoryRef == refs/tags/<tag>`, the verification gate passes, and an independent `gh attestation verify` from a clean checkout agrees; then delete the throwaway tag and release
- [ ] 10.5 Confirm a PR run in the same period produced no attestation, proving the `attest` input gates correctly
- [ ] 10.6 Confirm a forced e2e failure on an rc or release run blocks the publish steps
- [ ] 10.7 Open a deliberately stale test PR into `main` and confirm the ancestry gate fails it, then passes after merging `main` into the source branch
- [ ] 10.8 Push a version-less commit to `main` (or replay the event on a fork) and confirm `tag-release.yml` skips both tag creation and dispatch; repeat with a `-dev` version to confirm the shape guard

## 11. First real release and submission

- [ ] 11.1 Register the ancestry gate and the PR `e2e` job as required status checks on `main` and enable `required_status_checks.strict`, so the gate is enforced at merge time rather than advisory
- [ ] 11.2 Merge a release branch to `main` and confirm `tag-release.yml` creates `v<version>` and dispatches the release workflow
- [ ] 11.3 Verify the published asset's provenance names `refs/tags/v<version>` and its digest matches both the release asset and the S3 copy
- [ ] 11.4 Resubmit the plugin to Grafana and confirm `no-provenance-attestation` is no longer reported
- [ ] 11.5 Retire the `rc/*` branch pattern in the team's release docs, pointing at `docs/releasing.md` for the dev/rc/release tag channels
- [ ] 11.6 Record the outcome on HDX-12163
