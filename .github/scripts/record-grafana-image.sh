#!/bin/bash
set -euo pipefail

# Resolve the Grafana image under test to an immutable digest and record it.
#
# Used by .github/workflows/nightly-e2e.yml. That matrix includes the
# `nightly` tag, which floats by design -- pinning it would defeat the point
# of a canary -- so the tag alone does not identify what a given run tested.
# Resolving it to a digest is what keeps a red job reproducible after the tag
# has moved on: re-run against grafana/grafana-enterprise@<digest>.
#
# Writes grafana-image.txt (collected into the job's artifact) and, when
# GITHUB_STEP_SUMMARY is set, a table into the run summary.
#
# Env:
#   GRAFANA_VERSION            required, the image tag
#   GRAFANA_REPO               fallback repo if the container cannot be
#                              inspected (default: grafana/grafana-enterprise)
#   CONTAINER                  container to read the real image from
#                              (default: grafana, per e2e-docker-compose.yml)
#   GF_FEATURE_TOGGLES_ENABLE  recorded as-is; empty means none
#   LUXON                      matrix axis value, for the summary heading
#   OUTPUT_FILE                default: grafana-image.txt
#   GITHUB_STEP_SUMMARY        appended to when set

GRAFANA_VERSION="${GRAFANA_VERSION:?GRAFANA_VERSION must be set}"
GRAFANA_REPO="${GRAFANA_REPO:-grafana/grafana-enterprise}"
CONTAINER="${CONTAINER:-grafana}"
TOGGLES="${GF_FEATURE_TOGGLES_ENABLE:-}"
LUXON="${LUXON:-}"
OUTPUT_FILE="${OUTPUT_FILE:-grafana-image.txt}"

# Prefer the image the running container actually reports over one rebuilt
# from GRAFANA_REPO + GRAFANA_VERSION. The reconstructed name duplicates the
# `image:` line in .github/e2e-docker-compose.yml with nothing tying the two
# together, so any change there (a mirror, a pull-through cache, grafana/grafana)
# would silently start recording the wrong image -- or none.
IMAGE=$(docker inspect --format='{{.Config.Image}}' "$CONTAINER" 2>/dev/null | head -n1 || true)
IMAGE="${IMAGE//[$'\n\r']/}"
if [[ -z "$IMAGE" ]]; then
  IMAGE="$GRAFANA_REPO:$GRAFANA_VERSION"
fi

# An image built locally or pulled without a digest has no RepoDigests entry,
# and a missing image makes `docker inspect` emit a blank line to stdout
# before failing. Normalise both to the single token "unresolved" rather than
# `|| echo`, which would concatenate that blank line with the fallback and
# produce a two-line value that corrupts the key=value file and the summary
# table. The digest is diagnostic, so a failure here must not fail the job.
DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' "$IMAGE" 2>/dev/null | head -n1 || true)
DIGEST="${DIGEST//[$'\n\r']/}"
if [[ -z "$DIGEST" ]]; then
  DIGEST="unresolved"
  # Do not fail the job -- but do not degrade silently either. Without the
  # digest, a red run on the floating `nightly` tag is unreproducible once
  # the tag moves, which is the guarantee this script exists to provide.
  # A ::warning:: surfaces on the run page even when the job is green.
  echo "::warning title=Grafana digest unresolved::Could not resolve a RepoDigest for ${IMAGE}. This run is NOT reproducible once the tag moves. Check the image name against .github/e2e-docker-compose.yml."
fi

{
  echo "grafana_image=$IMAGE"
  echo "grafana_digest=$DIGEST"
  echo "feature_toggles=${TOGGLES:-(none)}"
} | tee "$OUTPUT_FILE"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### Grafana ${GRAFANA_VERSION} (luxon=${LUXON})"
    echo ""
    echo "| Field | Value |"
    echo "| --- | --- |"
    echo "| Image | \`$IMAGE\` |"
    echo "| Digest | \`$DIGEST\` |"
    echo "| Feature toggles | \`${TOGGLES:-(none)}\` |"
  } >> "$GITHUB_STEP_SUMMARY"
fi
