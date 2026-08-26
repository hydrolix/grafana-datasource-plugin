# Releasing

Publication is entirely tag-driven. `.github/workflows/release.yml` triggers
on `push: tags: ['v*']` (and `workflow_dispatch`), classifies the tag into one
of three shapes, and runs only that channel's steps. Nothing else — no
`pull_request` run, no push to `develop` or `main` on its own — creates a
GitHub release, uploads to S3, or creates a tag other than the automatic
release tag described below.

| Channel | Tag shape             | Created by                        | Gates                          | Publishes                              |
|---------|------------------------|------------------------------------|---------------------------------|-----------------------------------------|
| dev     | `vX.Y.Z-dev+<sha8>`    | maintainer, manual push            | version + SHA agreement only    | S3 `dev/` only                          |
| rc      | `vX.Y.Z-rc.N`          | maintainer, manual push            | plugin-validator + full e2e matrix | S3 `rc/` only                        |
| release | `vX.Y.Z`               | automatic, on merge to `main`      | plugin-validator + full e2e matrix | GitHub release + S3 flat path + attestation + provenance verification |

All three channels build the frontend and backend within the run and sign
the plugin. Only the release channel attests.

## Branch containment

Tag shape alone only says what a tag *claims* to be — anyone able to push a
`v*` tag could otherwise point a release tag at an arbitrary commit. Before
anything builds or signs, the `classify` job in `release.yml` resolves the
tagged commit and checks it against freshly fetched branch refs:

| Channel | Tag must be contained in                                            |
| ------- | ------------------------------------------------------------------- |
| release | `origin/main`                                                       |
| rc      | at least one `origin/release[/_-]*` or `origin/hotfix[/_-]*` branch |
| dev     | `origin/develop`                                                    |

A tag whose commit is not reachable from the required branch fails the run
immediately, naming the tag, the commit, and the required branch. For an
annotated tag this check peels the tag object to the commit it points at
(`git rev-parse <tag>^{commit}`) rather than trusting the push event's SHA,
which for annotated tags is the tag object itself.

For auto-created release tags this check is redundant by construction — the
tag is cut from `main`'s tip. It matters for the manually pushed recovery
path and for rc/dev tags, which are always pushed by hand.

**Accepted edge**: if an rc tag's release branch is deleted before the tag
is re-run (for example after the release branch merges to `main` and is
cleaned up), the containment check fails naming the missing branch, and the
rc publish cannot be re-run. This is accepted rather than worked around: an
inert rc tag has no consumer once the eventual release supersedes it.

## Dev channel

Purpose: hand a specific `develop` commit to someone without waiting for a
release — fastest path to a real ZIP, no gates beyond agreement checks.

1. Check out the `develop` commit you want to ship (or just use your current
   `develop` HEAD).
2. `package.json`'s version must already be the `X.Y.Z-dev` form (this is the
   normal state of `develop`).
3. Tag and push:

   ```bash
   git tag "v$(jq -r .version package.json)+$(git rev-parse --short=8 HEAD)"
   git push origin "v$(jq -r .version package.json)+$(git rev-parse --short=8 HEAD)"
   ```

   (Run the `jq`/`git rev-parse` substitution once and reuse the value if you
   want to avoid re-evaluating it between the two commands.)

4. The tagged commit's full SHA must start with the 8 hex characters in the
   tag — since the tag is built from `git rev-parse --short=8 HEAD` on that
   same commit, this holds by construction as long as you don't hand-edit
   the tag.
5. The run signs the plugin, checks version/SHA agreement, uploads to S3
   under `dev/`, and round-trips the digest over the public URL. It does
   **not** run the plugin validator, e2e, or attestation.

To take a dev build back out, delete the tag (`git push origin
:refs/tags/<tag>`) and remove the S3 object if it should not remain
reachable.

## RC channel

Purpose: publish an installable, signed pre-release build to a stable public
URL, without creating a GitHub release or an attestation. This is the only
way to hand someone a release-branch or hotfix build before it merges — a
PR run's ZIP is a 7-day Actions artifact requiring repo access.

Not a quality gate: the PR into `main` already builds, signs, validates, and
runs the full e2e matrix, against the merge ref rather than the branch tip.
The gates below exist so the `rc/` prefix never carries ungated bytes, not
to produce a signal the PR run lacks.

1. On the release branch (`release/vX.Y.Z`), `package.json`'s version is the
   plain `X.Y.Z` (release) form.
2. Tag and push manually, incrementing `N` per attempt:

   ```bash
   git tag v<X.Y.Z>-rc.<N>
   git push origin v<X.Y.Z>-rc.<N>
   ```

3. The run asserts the tag's base version equals `package.json`, rewrites
   the in-run version to `X.Y.Z-rc.N`, signs, then runs the plugin validator
   and the full Playwright matrix (Grafana 10.4.18, 11.6.1, 12.0.2, 13.0.1)
   against the ZIP it just built. Only if both pass does it upload to S3
   under `rc/` and round-trip the digest.
4. No GitHub release and no attestation are created for RCs — the S3 URL is
   the hand-off, and Grafana's verifier never sees pre-release builds.

## Release channel

Purpose: the one channel Grafana's provenance verifier sees. Releasing is a
single maintainer action: merging the release branch to `main`.

1. Before opening the release PR, merge `main` into the release branch so
   the branch already contains `main`'s tip. A base-`main` PR is gated on
   this (the ancestry check) — a stale branch is rejected.
2. `package.json` on the release branch carries the plain `X.Y.Z` version.
3. Merge the release PR into `main` (repository rules require a merge
   commit; do not squash or rebase).
4. `.github/workflows/tag-release.yml` runs on that push, reads the version
   from `package.json`, creates the tag `v<version>` at the pushed commit
   (skipping cleanly if the version isn't a plain `X.Y.Z` or the tag already
   exists), and dispatches `release.yml` at `refs/tags/v<version>`.
5. The release run builds, signs, attests (`sourceRepositoryRef` becomes
   `refs/tags/v<version>`), then gates on the plugin validator and the full
   e2e matrix. On success it creates the GitHub release (body from
   `CHANGELOG.md`'s top entry — the run fails first if that entry is empty)
   and publishes to S3 at the existing flat path.
6. A final verification job downloads the release asset back, runs `gh
   attestation verify` against it, asserts the source ref is
   `refs/tags/v<version>`, and asserts the S3 copy's digest matches the
   verified asset. Any mismatch fails the run red.

### CHANGELOG requirement

Add the version's entry to `CHANGELOG.md` on the release branch before
merging — the release run fails before creating the GitHub release if the
topmost entry is empty.

### If a release run fails

An unreleased tag is inert — nothing has been published, so there is no
inconsistent state to clean up.

- **Fix forward on the same tag**: if the failure is transient (a flaky e2e
  job, a network blip) or the fix doesn't change `package.json`'s version,
  re-run the failed jobs (or the whole run) from the Actions UI at the
  existing tag. The workflow is idempotent for a given tag.
- **Delete and re-push**: if the fix requires new commits, delete the tag
  (`git push origin :refs/tags/v<version>`) after landing the fix, then
  either let a subsequent merge to `main` recreate it (if `package.json`'s
  version is unchanged, first delete the tag so `tag-release.yml`'s
  already-exists guard doesn't skip it) or push the tag manually
  (`git tag v<version> <fixed-commit> && git push origin v<version>`), which
  triggers `release.yml` the same way a maintainer-pushed dev or RC tag
  does.

## Manual dispatch and duplicate triggers

`release.yml` accepts `workflow_dispatch` as well as `push: tags`, so it can
be re-run directly against an existing tag from the Actions UI. Because both
triggers can fire for the same tag, runs are serialized by a
`concurrency: release-<ref>` group with `cancel-in-progress: false`, so a
duplicate run cannot produce a second release or a second S3 upload.
