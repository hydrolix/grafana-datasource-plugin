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
#   GRAFANA_REPO               default: grafana/grafana-enterprise
#   GF_FEATURE_TOGGLES_ENABLE  recorded as-is; empty means none
#   LUXON                      matrix axis value, for the summary heading
#   OUTPUT_FILE                default: grafana-image.txt
#   GITHUB_STEP_SUMMARY        appended to when set

GRAFANA_VERSION="${GRAFANA_VERSION:?GRAFANA_VERSION must be set}"
GRAFANA_REPO="${GRAFANA_REPO:-grafana/grafana-enterprise}"
TOGGLES="${GF_FEATURE_TOGGLES_ENABLE:-}"
LUXON="${LUXON:-}"
OUTPUT_FILE="${OUTPUT_FILE:-grafana-image.txt}"

IMAGE="$GRAFANA_REPO:$GRAFANA_VERSION"

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
