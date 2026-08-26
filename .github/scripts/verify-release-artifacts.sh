#!/usr/bin/env bash
# Verifies a published release's provenance and S3 mirror end to end,
# against the downloaded copies rather than any local workspace ZIP:
#
#   1. Download the release's zip asset via `gh release download`.
#   2. `gh attestation verify` that downloaded asset.
#   3. Compute its sha256 and compare against the S3 copy fetched over the
#      public HTTPS URL (proves reachability, bucket policy, and URL
#      encoding, not just that GitHub served the right bytes).
#   4. Only then assert the attestation's sourceRepositoryRef equals
#      refs/tags/<tag> — deliberately last, so a PR-ref attestation still
#      exercises (and proves) every earlier step before failing here.
#
# Usage:
#   verify-release-artifacts.sh <tag> [repo]
#
#   <tag>   Release tag to verify, e.g. v0.11.0
#   [repo]  owner/repo to verify against; defaults to $GITHUB_REPOSITORY
#
# Requires `gh` to be authenticated (GH_TOKEN or prior `gh auth login`) with
# access to the target repo.
#
# Example (local smoke test):
#   GITHUB_REPOSITORY=hydrolix/grafana-datasource-plugin \
#     ./.github/scripts/verify-release-artifacts.sh v0.11.0
set -euo pipefail

TAG="${1:?Usage: verify-release-artifacts.sh <tag> [repo]}"
REPO="${2:-${GITHUB_REPOSITORY:-}}"

if [[ -z "$REPO" ]]; then
  echo "No repo given: pass it as \$2 or set GITHUB_REPOSITORY"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=s3-config.sh
source "$SCRIPT_DIR/s3-config.sh"

# Portable sha256: macOS has no sha256sum, only shasum -a 256.
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

WORKDIR=$(mktemp -d)
cleanup() {
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

# --- 1. Download the release asset -----------------------------------------
gh release download "$TAG" --repo "$REPO" --pattern '*.zip' --dir "$WORKDIR/release-asset"
ZIP=$(find "$WORKDIR/release-asset" -maxdepth 1 -name '*.zip' | head -n 1)
if [[ -z "$ZIP" ]]; then
  echo "No zip asset downloaded from release $TAG"
  exit 1
fi
ZIP_NAME=$(basename "$ZIP")
echo "Downloaded release asset: $ZIP_NAME"

# --- 2. Verify attestation ---------------------------------------------------
ATTESTATION_JSON="$WORKDIR/attestation.json"
gh attestation verify "$ZIP" --repo "$REPO" --format json > "$ATTESTATION_JSON"
echo "gh attestation verify succeeded"

# --- 3. S3 digest comparison -------------------------------------------------
DIGEST=$(sha256_of "$ZIP")
S3_COPY="$WORKDIR/s3-copy.zip"
curl -fsSL -o "$S3_COPY" "${S3_PUBLIC_BASE_URL}/grafana-datasource-plugin/${ZIP_NAME}"
S3_DIGEST=$(sha256_of "$S3_COPY")
if [[ "$S3_DIGEST" != "$DIGEST" ]]; then
  echo "S3 copy sha256 \"$S3_DIGEST\" does not equal release asset sha256 \"$DIGEST\""
  exit 1
fi
echo "S3 copy matches: sha256=$DIGEST"

# --- 4. Source-ref assertion (last, deliberately) ---------------------------
SOURCE_REF=$(jq -r '.[0].verificationResult.signature.certificate.sourceRepositoryRef' "$ATTESTATION_JSON")
EXPECTED_REF="refs/tags/$TAG"
if [[ "$SOURCE_REF" != "$EXPECTED_REF" ]]; then
  echo "Attestation sourceRepositoryRef \"$SOURCE_REF\" does not equal \"$EXPECTED_REF\""
  exit 1
fi
echo "Verified $ZIP_NAME: sourceRepositoryRef=$SOURCE_REF sha256=$DIGEST"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## Release verification"
    echo "- Verified source ref: \`$SOURCE_REF\`"
    echo "- Artifact digest: \`sha256:$DIGEST\`"
    echo "- S3 copy digest matches the verified release asset"
  } >> "$GITHUB_STEP_SUMMARY"
fi
