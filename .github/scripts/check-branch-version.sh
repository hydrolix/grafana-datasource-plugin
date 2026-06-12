#!/usr/bin/env bash
# Verify the PR head branch name encodes the same version as the manifest,
# and the PR target (when on a maintenance line) covers the same X.Y line.
#
# Expects (set by GitHub Actions on pull_request / pull_request_target):
#   GITHUB_HEAD_REF  — source branch name (e.g. release/v1.2.3, hotfix/v1.2.1)
#   GITHUB_BASE_REF  — target branch name (e.g. main, maintenance/v1.2.x)

set -euo pipefail

HEAD_REF="${GITHUB_HEAD_REF:-}"
BASE_REF="${GITHUB_BASE_REF:-}"

if [[ -z "$HEAD_REF" || -z "$BASE_REF" ]]; then
  echo "GITHUB_HEAD_REF and GITHUB_BASE_REF must be set (run inside a pull_request event)."
  exit 1
fi

VERSION=$(jq -r .version package.json)
BASE_VERSION="${VERSION%-rc*}"  # strip -rcN suffix if present

case "$HEAD_REF" in
  release/v*|hotfix/v*)
    BRANCH_VERSION="${HEAD_REF#*/v}"
    ;;
  *)
    echo "Unexpected source branch \"$HEAD_REF\": expected release/vX.Y.Z or hotfix/vX.Y.Z"
    exit 1
    ;;
esac

if ! [[ "$BRANCH_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Source branch \"$HEAD_REF\" must encode a base version vX.Y.Z (got \"v$BRANCH_VERSION\")"
  exit 1
fi

if [[ "$BASE_VERSION" != "$BRANCH_VERSION" ]]; then
  echo "Manifest base version ($BASE_VERSION) does not match source branch version ($BRANCH_VERSION)"
  echo "  manifest:     $VERSION"
  echo "  source branch: $HEAD_REF"
  exit 1
fi

# Maintenance-line guard: hotfix/vA.B.C → maintenance/vX.Y.x requires A.B == X.Y.
if [[ "$BASE_REF" =~ ^maintenance/v([0-9]+)\.([0-9]+)\.x$ ]]; then
  MAINT_MAJOR="${BASH_REMATCH[1]}"
  MAINT_MINOR="${BASH_REMATCH[2]}"
  BRANCH_MAJOR="${BRANCH_VERSION%%.*}"
  BRANCH_REST="${BRANCH_VERSION#*.}"
  BRANCH_MINOR="${BRANCH_REST%%.*}"

  if [[ "$MAINT_MAJOR" != "$BRANCH_MAJOR" || "$MAINT_MINOR" != "$BRANCH_MINOR" ]]; then
    echo "Source branch $HEAD_REF (v$BRANCH_VERSION) targets $BASE_REF, but $MAINT_MAJOR.$MAINT_MINOR != $BRANCH_MAJOR.$BRANCH_MINOR"
    exit 1
  fi
fi

echo "OK: $HEAD_REF carries v$BASE_VERSION (manifest $VERSION) targeting $BASE_REF"
