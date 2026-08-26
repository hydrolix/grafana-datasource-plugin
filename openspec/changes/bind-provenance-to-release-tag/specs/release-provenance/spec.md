# release-provenance

## ADDED Requirements

### Requirement: The published release ZIP is attested in a run whose source ref is the release tag

The workflow that builds, signs, and packages the artifact distributed to the GitHub release and to S3 SHALL execute with `github.ref` equal to `refs/tags/v<version>`, and SHALL invoke `actions/attest-build-provenance` on that ZIP within the same run. The resulting attestation's `sourceRepositoryRef` SHALL therefore be `refs/tags/v<version>`, and its subject digest SHALL be the digest of the published asset.

#### Scenario: Provenance of a published release names the tag, not a pull-request ref

- **GIVEN** a release published for version `<version>` by the tag-triggered workflow
- **WHEN** `gh attestation verify <plugin>-<version>.zip --repo hydrolix/grafana-datasource-plugin --format json` is run against the asset downloaded from that release
- **THEN** the command SHALL exit `0`
- **AND** the attestation's `sourceRepositoryRef` SHALL equal `refs/tags/v<version>`
- **AND** the attestation's `buildSignerURI` SHALL end with `@refs/tags/v<version>`
- **AND** the attestation's subject digest SHALL equal the `sha256` of the published asset

#### Scenario: The attested ZIP is the one this run built

- **GIVEN** the tag-triggered release workflow
- **WHEN** its job graph is inspected
- **THEN** the ZIP passed to `actions/attest-build-provenance` SHALL be produced by build and packaging steps in the same workflow run
- **AND** the workflow SHALL NOT attest an artifact downloaded from another run

### Requirement: Only the release channel SHALL emit an attestation

`.github/workflows/package.yml` SHALL expose a boolean `attest` input defaulting to `false`, and SHALL run its attestation step only when that input is `true`. The attestation step SHALL NOT be conditioned on `github.event_name` or `github.base_ref`. Only the release (`vX.Y.Z`) channel of the tag-triggered workflow SHALL pass `attest: true`; pull-request runs and dev and RC tag runs SHALL pass or inherit `attest: false`.

#### Scenario: A pull-request run packages without attesting

- **GIVEN** a `pull_request` run of `ci.yml` targeting any base branch
- **WHEN** the `package` job completes
- **THEN** the `plugin-package` artifact SHALL be uploaded for the `e2e` and `plugin-validator` jobs
- **AND** no attestation SHALL be created for that ZIP

#### Scenario: Dev and RC tag runs package without attesting

- **GIVEN** a tag-triggered run for `vX.Y.Z-dev+<sha8>` or `vX.Y.Z-rc.N`
- **WHEN** the packaging job completes
- **THEN** no attestation SHALL be created for that ZIP

#### Scenario: The attestation step is gated on the input alone

- **GIVEN** `.github/workflows/package.yml`
- **WHEN** the condition on the `actions/attest-build-provenance` step is read
- **THEN** it SHALL reference only the `attest` input
- **AND** it SHALL NOT reference `github.event_name` or `github.base_ref`

### Requirement: Version resolution recognizes the three tag shapes and enforces agreement

`.github/scripts/set-version.sh` SHALL treat a `GITHUB_REF_NAME` matching one of three tag shapes as a publication source, each with an agreement check that fails the run before any signing occurs. For `v<X.Y.Z>` it SHALL assert the `package.json` version equals `X.Y.Z` and keep it. For `v<X.Y.Z>-rc.<N>` it SHALL assert the `package.json` version equals `X.Y.Z` and rewrite it to `X.Y.Z-rc.N`. For `v<X.Y.Z>-dev+<sha8>` it SHALL assert the `package.json` version equals `X.Y.Z-dev` and that `<sha8>` is a prefix of the full SHA of `HEAD`, and rewrite the version to `X.Y.Z-dev+<sha8>`. Failure messages SHALL name both disagreeing values. Existing handling of `release`/`hotfix` branch refs and of `X.Y.Z-dev` pull-request builds SHALL be unchanged.

#### Scenario: A release tag ref yields the plain release version

- **GIVEN** `package.json` declaring version `0.12.0`
- **WHEN** the script runs with `GITHUB_REF_NAME=v0.12.0` and an empty `GITHUB_HEAD_REF`
- **THEN** the script SHALL exit `0`
- **AND** the `package.json` version SHALL remain `0.12.0`

#### Scenario: A mismatched release tag fails before signing

- **GIVEN** `package.json` declaring version `0.12.0`
- **WHEN** the script runs with `GITHUB_REF_NAME=v9.9.9`
- **THEN** the script SHALL exit non-zero
- **AND** the emitted message SHALL name both the tag and the `package.json` version

#### Scenario: An RC tag rewrites the version to the RC form

- **GIVEN** `package.json` declaring version `0.12.0`
- **WHEN** the script runs with `GITHUB_REF_NAME=v0.12.0-rc.1`
- **THEN** the script SHALL exit `0`
- **AND** the `package.json` version SHALL become `0.12.0-rc.1`

#### Scenario: An RC tag whose base version disagrees is rejected

- **GIVEN** `package.json` declaring version `0.13.0`
- **WHEN** the script runs with `GITHUB_REF_NAME=v0.12.0-rc.1`
- **THEN** the script SHALL exit non-zero

#### Scenario: A dev tag matching the commit rewrites the version

- **GIVEN** `package.json` declaring version `0.12.0-dev` at a commit whose full SHA begins with `<sha8>`
- **WHEN** the script runs with `GITHUB_REF_NAME=v0.12.0-dev+<sha8>`
- **THEN** the script SHALL exit `0`
- **AND** the `package.json` version SHALL become `0.12.0-dev+<sha8>`

#### Scenario: A dev tag whose embedded SHA is not the tagged commit is rejected

- **GIVEN** `package.json` declaring version `0.12.0-dev` at a commit whose full SHA does not begin with `deadbeef`
- **WHEN** the script runs with `GITHUB_REF_NAME=v0.12.0-dev+deadbeef`
- **THEN** the script SHALL exit non-zero
- **AND** the emitted message SHALL name the tag's SHA and the commit's SHA

#### Scenario: A dev tag against a non-dev version is rejected

- **GIVEN** `package.json` declaring version `0.12.0`
- **WHEN** the script runs with `GITHUB_REF_NAME=v0.12.0-dev+<sha8>` for the current commit
- **THEN** the script SHALL exit non-zero

#### Scenario: A dev pull-request build still gets a SHA suffix

- **GIVEN** `package.json` declaring version `0.12.0-dev`
- **WHEN** the script runs with `GITHUB_HEAD_REF=feature/HDX-12163-release-tag-provenance` and no tag ref
- **THEN** the script SHALL exit `0`
- **AND** the `package.json` version SHALL become `0.12.0-dev+<short-sha>`

#### Scenario: A release branch ref keeps its existing behaviour

- **GIVEN** `package.json` declaring version `0.12.0`
- **WHEN** the script runs with `GITHUB_HEAD_REF=release/v0.12.0`
- **THEN** the script SHALL exit `0`
- **AND** the `package.json` version SHALL remain `0.12.0`

### Requirement: Release publication is tag-driven and dispatch happens only for a newly created tag

A workflow triggered by `push` to `main` SHALL read the version from `package.json`, create the tag `v<version>` when the version matches `X.Y.Z` and the tag does not already exist, and start the release workflow at `refs/tags/v<version>` only when this run created the tag. When the tag already exists, or the version does not match `X.Y.Z`, the workflow SHALL neither create a tag nor dispatch. The release workflow SHALL accept both `workflow_dispatch` and `push` of tags matching `v*`, and SHALL be serialized by a `concurrency` group keyed on the tag. `ci.yml` SHALL NOT create tags, publish GitHub releases, or publish to S3.

#### Scenario: Merging a release branch to main tags and releases

- **GIVEN** a release branch declaring version `<version>` merged into `main`
- **WHEN** the `push` to `main` workflow completes
- **THEN** the tag `v<version>` SHALL exist in the repository
- **AND** a release workflow run SHALL have started with `github.ref` equal to `refs/tags/v<version>`

#### Scenario: A version-less push to main does not re-release

- **GIVEN** the tag `v<version>` already exists for the version in `package.json`
- **WHEN** a commit is pushed to `main` without a version change
- **THEN** the workflow SHALL NOT create a tag
- **AND** it SHALL NOT dispatch the release workflow

#### Scenario: A non-release version on main is not tagged

- **GIVEN** a commit pushed to `main` whose `package.json` version is `0.12.0-dev`
- **WHEN** the `push` to `main` workflow runs
- **THEN** it SHALL NOT create a tag
- **AND** it SHALL NOT dispatch the release workflow

#### Scenario: A manually pushed tag releases the same way

- **GIVEN** an existing commit on `main` whose `package.json` version is `<version>`
- **WHEN** a maintainer pushes the tag `v<version>`
- **THEN** the release workflow SHALL run with `github.ref` equal to `refs/tags/v<version>`
- **AND** it SHALL produce a signed, attested ZIP for `<version>`

#### Scenario: Concurrent triggers for one tag do not publish twice

- **GIVEN** both a `workflow_dispatch` and a `push` event for the tag `v<version>`
- **WHEN** the second run reaches the publish step
- **THEN** the runs SHALL be serialized by the tag-keyed `concurrency` group
- **AND** the release SHALL carry exactly one ZIP asset for `<version>`

#### Scenario: The pull-request pipeline no longer publishes

- **GIVEN** `.github/workflows/ci.yml` after this change
- **WHEN** its job list is read
- **THEN** it SHALL NOT contain a job that creates a git tag, creates a GitHub release, or uploads to S3
- **AND** no job SHALL be conditioned on `github.base_ref == 'main'` except the ancestry gate

### Requirement: The release run verifies the published copies before finishing

The release channel SHALL, after publishing, download the ZIP asset back from the GitHub release, run `gh attestation verify` against that downloaded copy, and fail the run unless the returned attestation's source ref equals `refs/tags/v<version>`. It SHALL additionally assert that the S3 copy's `sha256` digest equals the verified asset's digest. Verification SHALL run against downloaded copies, not the local workspace ZIP. Verification failure SHALL be a red run, not a warning. The verification logic SHALL be a script under `.github/scripts/`, invocable outside CI against a published release, with the workflow step invoking that script rather than inlining the logic.

#### Scenario: Verification logic is a locally runnable script

- **GIVEN** the release verification implementation
- **WHEN** its location is read
- **THEN** it SHALL be a script under `.github/scripts/`
- **AND** the `verify-release` workflow step SHALL invoke that script rather than inline the logic

#### Scenario: A tag-bound attestation passes the gate

- **GIVEN** a release run for tag `v<version>` that attested the ZIP it built
- **WHEN** the verification step runs against the asset re-downloaded from the GitHub release
- **THEN** the step SHALL exit `0`
- **AND** the run SHALL report the verified source ref and digest in its summary

#### Scenario: A non-tag attestation fails the gate

- **GIVEN** a release run whose attestation reports a source ref other than `refs/tags/v<version>`
- **WHEN** the verification step runs
- **THEN** the step SHALL exit non-zero
- **AND** the workflow run SHALL be marked failed

#### Scenario: An S3 copy that differs from the release asset fails the gate

- **GIVEN** a release run whose S3 upload produced bytes differing from the GitHub release asset
- **WHEN** the S3 digest comparison runs
- **THEN** the step SHALL exit non-zero
- **AND** the workflow run SHALL be marked failed

### Requirement: The tag-built release package is validated and e2e-tested before publication

The release channel SHALL run both the Grafana plugin validator and the Playwright e2e suite against the ZIP it built, and SHALL publish only if both succeed. The e2e suite SHALL cover every Grafana version in the matrix the pull-request pipeline covers. The workflow SHALL NOT depend on artifacts from the pull-request run.

#### Scenario: Validation gates publication

- **GIVEN** a release run for tag `v<version>`
- **WHEN** the plugin validator reports a failure for the freshly built ZIP
- **THEN** the GitHub release SHALL NOT be created
- **AND** the S3 upload SHALL NOT occur

#### Scenario: E2E gates publication

- **GIVEN** a release run for tag `v<version>`
- **WHEN** the e2e suite fails for any Grafana version in the matrix
- **THEN** the GitHub release SHALL NOT be created
- **AND** the S3 upload SHALL NOT occur

#### Scenario: E2E exercises the artifact this run built

- **GIVEN** the tag-triggered release workflow
- **WHEN** the e2e job resolves its plugin under test
- **THEN** it SHALL consume the `plugin-package` artifact produced by the packaging job of the same run
- **AND** the Grafana version matrix SHALL match the one used by the pull-request pipeline

#### Scenario: The release does not reuse pull-request artifacts

- **GIVEN** the tag-triggered release workflow
- **WHEN** its steps are read
- **THEN** it SHALL build the frontend and backend within the run
- **AND** it SHALL NOT download the `plugin-package` artifact produced by a `pull_request` run

### Requirement: A pull request into `main` SHALL already contain `main`'s tip

The pull-request pipeline SHALL run a check, when the base branch is `main`, that fails unless the current tip of `origin/main` is an ancestor of the pull request's head commit. The check SHALL resolve `main` by fetching it during the run rather than reading `github.event.pull_request.base.sha`, and SHALL evaluate ancestry against `github.event.pull_request.head.sha` rather than the checked-out `HEAD`. The check SHALL NOT run for pull requests whose base is not `main`.

#### Scenario: A stale release branch is rejected

- **GIVEN** a pull request into `main` whose source branch does not contain the current tip of `main`
- **WHEN** the ancestry check runs
- **THEN** the check SHALL fail
- **AND** the message SHALL name the `main` commit that is missing from the source branch

#### Scenario: An up-to-date release branch passes

- **GIVEN** a pull request into `main` whose source branch contains the current tip of `main`
- **WHEN** the ancestry check runs
- **THEN** the check SHALL pass

#### Scenario: Ancestry is evaluated against the branch head, not the merge ref

- **GIVEN** the ancestry check implementation
- **WHEN** the commit it tests for descendancy is read
- **THEN** it SHALL be `github.event.pull_request.head.sha`
- **AND** it SHALL NOT be the checked-out `HEAD` of the `refs/pull/<N>/merge` ref

#### Scenario: `main` is resolved fresh rather than from the event payload

- **GIVEN** a pull request into `main` opened before `main` advanced
- **WHEN** the ancestry check runs after `main` advanced
- **THEN** it SHALL compare against the advanced tip of `origin/main`
- **AND** the check SHALL fail until the source branch contains that tip

#### Scenario: Pull requests into other branches are unaffected

- **GIVEN** a pull request into `develop`
- **WHEN** the pipeline runs
- **THEN** the ancestry check SHALL be skipped
- **AND** it SHALL NOT block the pull request
