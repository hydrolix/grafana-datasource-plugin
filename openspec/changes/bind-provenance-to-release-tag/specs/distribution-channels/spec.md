# distribution-channels

## ADDED Requirements

### Requirement: The tag-triggered workflow selects its publication channel from the tag's shape

The workflow triggered by `v*` tags SHALL classify the tag as exactly one of: dev (`v<X.Y.Z>-dev+<sha8>`), RC (`v<X.Y.Z>-rc.<N>`), or release (`v<X.Y.Z>`), and SHALL run only that channel's publication steps. A tag matching none of the three shapes SHALL fail the run without publishing. All channels SHALL build the frontend and backend within the run and sign the plugin.

#### Scenario: A dev tag runs the dev channel only

- **GIVEN** a pushed tag `v0.12.0-dev+<sha8>`
- **WHEN** the workflow run completes
- **THEN** the ZIP SHALL have been uploaded to S3 under the `dev/` prefix
- **AND** no GitHub release SHALL be created
- **AND** no attestation SHALL be created

#### Scenario: An RC tag runs the RC channel only

- **GIVEN** a pushed tag `v0.12.0-rc.1`
- **WHEN** the workflow run completes successfully
- **THEN** the ZIP SHALL have been uploaded to S3 under the `rc/` prefix
- **AND** no GitHub release SHALL be created
- **AND** no attestation SHALL be created

#### Scenario: A release tag runs the release channel

- **GIVEN** the tag `v0.12.0`
- **WHEN** the workflow run completes successfully
- **THEN** a GitHub release SHALL exist with the ZIP attached
- **AND** the ZIP SHALL have been uploaded to S3 at the existing unprefixed path

#### Scenario: An unrecognized tag shape publishes nothing

- **GIVEN** a pushed tag `v0.12.0-beta.1`
- **WHEN** the workflow runs
- **THEN** it SHALL fail before signing or publishing
- **AND** no S3 object and no GitHub release SHALL be created

### Requirement: A channel tag SHALL be contained in its source branch

The classify job SHALL, after shape classification and before any build or signing, resolve the tagged commit and fail the run unless it is contained in the channel's source branch, evaluated by ancestry against freshly fetched branch refs: `origin/main` for release tags, at least one `origin/release[/_-]*` or `origin/hotfix[/_-]*` branch for RC tags, and `origin/develop` for dev tags. The failure message SHALL name the tag, the commit, and the required branch.

#### Scenario: A release tag outside main is rejected

- **GIVEN** a pushed tag `v0.12.0` at a commit not contained in `origin/main`
- **WHEN** the classify job runs
- **THEN** it SHALL fail before any build, signing, or publishing
- **AND** the message SHALL name `main` as the required branch

#### Scenario: An RC tag on a release branch passes containment

- **GIVEN** a pushed tag `v0.12.0-rc.1` at a commit contained in `origin/release/v0.12.0`
- **WHEN** the classify job runs
- **THEN** the containment check SHALL pass

#### Scenario: An RC tag on a feature branch is rejected

- **GIVEN** a pushed tag `v0.12.0-rc.1` at a commit contained in no `release[/_-]*` or `hotfix[/_-]*` branch
- **WHEN** the classify job runs
- **THEN** it SHALL fail before any build, signing, or publishing

#### Scenario: A dev tag outside develop is rejected

- **GIVEN** a pushed tag `v0.12.0-dev+<sha8>` at a feature-branch commit not contained in `origin/develop`
- **WHEN** the classify job runs
- **THEN** it SHALL fail before any build, signing, or publishing
- **AND** the message SHALL name `develop` as the required branch

#### Scenario: A dev tag on develop passes containment

- **GIVEN** a pushed tag `v0.12.0-dev+<sha8>` at a commit contained in `origin/develop`
- **WHEN** the classify job runs
- **THEN** the containment check SHALL pass

### Requirement: The dev channel publishes a signed build without release gates

The dev channel SHALL publish the signed ZIP to S3 under the `dev/` prefix after version and SHA agreement checks pass. It SHALL NOT run the plugin validator or the e2e suite, SHALL NOT create a GitHub release, and SHALL NOT attest.

#### Scenario: A dev build reaches S3 after signing alone

- **GIVEN** a pushed tag `v0.12.0-dev+<sha8>` whose agreement checks pass
- **WHEN** the workflow's job graph for that run is inspected
- **THEN** the S3 upload SHALL depend on packaging (signing) and version agreement
- **AND** no validator or e2e job SHALL have run

#### Scenario: A dev build failing agreement never publishes

- **GIVEN** a pushed dev tag whose embedded SHA or version disagrees
- **WHEN** the workflow runs
- **THEN** the run SHALL fail before the S3 upload
- **AND** no object SHALL appear under `dev/`

### Requirement: The RC channel is gated by the validator and the full e2e matrix

The RC channel SHALL run the Grafana plugin validator and the Playwright e2e suite, covering every Grafana version in the pull-request pipeline's matrix, against the ZIP built in the same run, and SHALL upload to S3 under the `rc/` prefix only if both succeed.

#### Scenario: RC publication waits on validation and e2e

- **GIVEN** a pushed tag `v0.12.0-rc.1`
- **WHEN** the plugin validator or any e2e matrix job fails
- **THEN** the S3 upload SHALL NOT occur

#### Scenario: The RC e2e matrix matches the pull-request matrix

- **GIVEN** the RC channel's e2e invocation
- **WHEN** its Grafana version matrix is read
- **THEN** it SHALL equal the matrix used by the pull-request pipeline

### Requirement: S3 publication accepts a path prefix and keeps the release path unchanged

`.github/workflows/s3_publish.yml` SHALL accept a path-prefix input, defaulting to none. The dev channel SHALL pass `dev/`, the RC channel SHALL pass `rc/`, and the release channel SHALL pass no prefix, so released ZIPs keep the current `grafana-datasource-plugin/<zip>` layout consumed by existing users.

#### Scenario: Prefixed uploads land under their channel directory

- **GIVEN** the dev channel publishing `hydrolix-hydrolix-datasource-0.12.0-dev+<sha8>.zip`
- **WHEN** the upload completes
- **THEN** the object key SHALL begin with `grafana-datasource-plugin/dev/`

#### Scenario: Release uploads keep the existing flat path

- **GIVEN** the release channel publishing `hydrolix-hydrolix-datasource-0.12.0.zip`
- **WHEN** the upload completes
- **THEN** the object key SHALL be `grafana-datasource-plugin/hydrolix-hydrolix-datasource-0.12.0.zip`

### Requirement: Dev and RC channels verify their S3 copy by digest round-trip

After uploading, the dev and RC channels SHALL download the object back over the public HTTPS URL published to consumers and fail the run unless its `sha256` digest equals that of the ZIP built in the run.

#### Scenario: A corrupted upload fails the run

- **GIVEN** a dev or RC run whose S3 object differs from the built ZIP
- **WHEN** the digest round-trip step runs
- **THEN** it SHALL exit non-zero
- **AND** the workflow run SHALL be marked failed

#### Scenario: A faithful upload passes

- **GIVEN** a dev or RC run whose S3 object matches the built ZIP
- **WHEN** the digest round-trip step runs
- **THEN** it SHALL exit `0`
- **AND** the run summary SHALL report the object URL and digest

### Requirement: The release channel SHALL fail when its changelog entry is empty

Before creating the GitHub release, the release channel SHALL parse the changelog entry for the version being released and SHALL fail the run when the entry is missing or empty.

#### Scenario: A missing changelog entry blocks the release

- **GIVEN** a release run for tag `v<version>` where `CHANGELOG.md` has no entry for `<version>`
- **WHEN** the changelog parsing step runs
- **THEN** it SHALL exit non-zero
- **AND** the GitHub release SHALL NOT be created

#### Scenario: A present changelog entry ships as the release body

- **GIVEN** a release run for tag `v<version>` where `CHANGELOG.md` carries an entry for `<version>`
- **WHEN** the GitHub release is created
- **THEN** its body SHALL be the parsed entry

### Requirement: Publication workflows declare least-privilege permissions

`release.yml`, `tag-release.yml`, and `ci.yml` SHALL declare a workflow-level `permissions` block of `contents: read`, and jobs needing more SHALL elevate explicitly in their own `permissions` block.

#### Scenario: Workflow-level permissions are read-only

- **GIVEN** the publication workflow files
- **WHEN** their top-level `permissions` blocks are read
- **THEN** each SHALL declare `contents: read` and nothing broader
- **AND** jobs that write (tagging, releasing, attesting) SHALL carry their own explicit `permissions` block

### Requirement: Nothing publishes outside the tag-triggered workflow

Pull-request runs and branch pushes SHALL NOT upload to S3, create GitHub releases, or create tags. Dev and RC tags are pushed by maintainers; only the release tag is created automatically, on merge to `main`.

#### Scenario: A merge to develop publishes nothing

- **GIVEN** a pull request merged into `develop`
- **WHEN** all workflows triggered by that merge complete
- **THEN** no S3 object, GitHub release, or tag SHALL have been created

#### Scenario: A pull-request run publishes nothing

- **GIVEN** a `pull_request` run of `ci.yml` targeting any base branch
- **WHEN** the run completes
- **THEN** no S3 object, GitHub release, or tag SHALL have been created
