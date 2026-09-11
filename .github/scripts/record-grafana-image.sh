#!/bin/bash
set -euo pipefail

# Resolve the Grafana image under test to an immutable digest and record it to
# grafana-image.txt and the step summary. The nightly tag floats, so the tag
# alone does not identify what ran; the digest is what makes a red job
# reproducible after the tag moves.
#
# Env: GRAFANA_VERSION (required), CONTAINER, LUXON,
#      GF_FEATURE_TOGGLES_ENABLE, OUTPUT_FILE, GITHUB_STEP_SUMMARY.

GRAFANA_VERSION="${GRAFANA_VERSION:?GRAFANA_VERSION must be set}"
CONTAINER="${CONTAINER:-grafana}"
TOGGLES="${GF_FEATURE_TOGGLES_ENABLE:-}"
LUXON="${LUXON:-}"
OUTPUT_FILE="${OUTPUT_FILE:-grafana-image.txt}"

# Read the image from the container rather than rebuilding it from the tag,
# which would silently drift from the `image:` line in e2e-docker-compose.yml.
IMAGE=$(docker inspect --format='{{.Config.Image}}' "$CONTAINER" 2>/dev/null | head -n1 || true)
IMAGE="${IMAGE//[$'\n\r']/}"
if [[ -z "$IMAGE" ]]; then
  # Deliberately not reconstructed from GRAFANA_VERSION: in the only case
  # where that would differ from the compose file, the guess is wrong by
  # construction, and a confident wrong record is worse than an absent one.
  IMAGE="unknown"
  echo "::warning title=Grafana image unknown::Could not inspect container '${CONTAINER}'; this run is not reproducible."
fi

# `docker inspect` emits a blank line before failing on a missing image, so
# `|| echo unresolved` would yield a two-line value and corrupt the output.
DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' "$IMAGE" 2>/dev/null | head -n1 || true)
DIGEST="${DIGEST//[$'\n\r']/}"
if [[ -z "$DIGEST" ]]; then
  DIGEST="unresolved"
  # Diagnostic only, so don't fail the job — but don't degrade silently.
  # Skipped when the image is already unknown: that warned above, and
  # "could not resolve a digest for unknown" adds noise, not information.
  if [[ "$IMAGE" != "unknown" ]]; then
    echo "::warning title=Grafana digest unresolved::Could not resolve a RepoDigest for ${IMAGE}. This run is NOT reproducible once the tag moves. Check the image name against .github/e2e-docker-compose.yml."
  fi
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
