## Why

Grafana's plugin review reports `no-provenance-attestation` for our released ZIP even though we do publish one. The attestation verifies (`gh attestation verify` exits 0) and its subject digest matches the release asset byte-for-byte, but it is bound to `refs/pull/164/merge` — an ephemeral PR merge ref that no longer resolves — because the job that signs, packages, and attests runs on the `pull_request` event, and the `v<version>` tag is created only afterwards. Nothing is ever built on the tag ref, so a tag-scoped verifier concludes the release has no provenance. Every submission carries this finding until the build that produces the shipped ZIP runs on the tag.

Moving publication off the PR run also removes the only pre-release distribution path: today S3 publication happens on pull requests into `main`, which is how RCs shipped — an `rc/*` branch carrying a `-dev` version, producing a mislabeled draft release. Dev and RC distribution therefore need explicit channels of their own rather than a relocated side-effect.

## What Changes

- `ci.yml` keeps build, test, e2e, and validation on `pull_request`; publishing moves entirely to a tag-triggered path — PR runs publish nothing.
- Add a workflow that creates the `v<version>` tag when a release branch merges to `main`, replacing the tag-creation step currently embedded in `release.yml`.
- Rework `release.yml` into a workflow triggered on `v*` tags that selects a publication channel from the tag's shape:
  - `vX.Y.Z-dev+<sha8>` (pushed manually): signed dev build published to S3 under `dev/` — no GitHub release, no attestation.
  - `vX.Y.Z-rc.N` (pushed manually from a release branch): the built ZIP is validated and e2e-tested against the supported Grafana matrix, then published to S3 under `rc/` — no GitHub release, no attestation.
  - `vX.Y.Z` (created automatically on merge to `main`): built, signed, attested, validated, e2e-tested against the matrix, published as a GitHub release and to S3's existing flat path, then provenance-verified — attestation ref `refs/tags/vX.Y.Z`.
- Replace the `github.event_name == 'pull_request' && github.base_ref == 'main'` gate on the attestation step in `package.yml` with an explicit input, set only by the release channel.
- Teach `.github/scripts/set-version.sh` the three tag shapes; fail when the tag disagrees with `package.json` or, for dev tags, with the tagged commit's SHA.
- Add a path-prefix input to `s3_publish.yml`; releases keep the current flat S3 path.
- Verify after publishing: re-download the GitHub release asset and `gh attestation verify` it; digest-check the S3 copy over its public URL on every channel.
- Guard tag creation on `main` so only plain `X.Y.Z` versions are tagged, and fail a release whose changelog entry is empty.
- Add a pull-request gate for base `main` that fails unless the source branch already contains `main`'s tip, so the merge commit that gets tagged has the tree CI actually exercised.
- Remove the release and S3-publish jobs from `ci.yml`; drop draft `-dev` releases; retire the `rc/*` branch pattern.
- Verification coverage: shell-level tests for version resolution, e2e against rc- and release-tag builds, channel rehearsals, and a post-publish gate asserting the published ZIP's attestation names the release tag.
- **BREAKING (release process only)**: the shipped ZIP is rebuilt on the tag rather than promoted from the PR run, so its bytes differ from the PR-tested artifact; dev and RC distribution become deliberate manual tag pushes. No plugin-runtime, API, or dashboard impact — Grafana 10.x compatibility is untouched.

## Capabilities

### New Capabilities

- `release-provenance`: The contract that the plugin ZIP distributed to Grafana and S3 is built, signed, tested, and attested in a workflow run whose source ref is the release tag; that the tag, `package.json` version, and attestation subject agree; that the commit being released is the one CI exercised; and that provenance is verifiable after the fact with `gh attestation verify`.
- `distribution-channels`: The contract that publication channels are selected by tag shape — manually pushed dev and RC tags publish signed builds to prefixed S3 paths without GitHub releases or attestations, only the release tag publishes a GitHub release, and pull-request runs and branch pushes publish nothing.

### Modified Capabilities

<!-- None — the release pipeline is not codified in any current spec. -->

## Impact

- **Frontend / Backend**: no source changes. `src/` and `pkg/` are untouched.
- **CI**: `.github/workflows/ci.yml` loses its release and S3-publish jobs and gains the base-`main` ancestry gate and script tests; `.github/workflows/package.yml` gains an attestation input; `.github/workflows/release.yml` becomes the tag-triggered channel workflow; new tag-creation workflow added; `s3_publish.yml` gains a prefix input. `build-frontend.yml`, `build-backend.yml`, `e2e.yml`, and `validate.yml` are reused as-is.
- **Scripts**: `.github/scripts/set-version.sh` gains tag-shape handling and agreement checks; `.github/scripts/publish-aws.sh` gains a path-prefix parameter.
- **Release process**: releases happen by merging the release branch to `main` (auto-tag); dev and RC builds happen by pushing a tag; `rc/*` branches are retired; maintainers merge `main` into the release branch before merging to `main`.
- **Repository settings**: the ancestry gate is only advisory until it is registered as a required status check on `main` with strict mode enabled — `main` currently has no required checks and `strict: false`.
- **Secrets / permissions**: the tag-triggered workflow needs `contents: write`, `id-token: write`, `attestations: write`, plus the Grafana signing token and AWS credentials that `ci.yml` holds today.
- **User-visible**: none. This change makes an existing artifact property verifiable; it does not alter plugin behaviour.
- **Tracking**: HDX-12163.
