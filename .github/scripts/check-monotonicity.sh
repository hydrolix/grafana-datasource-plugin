#!/usr/bin/env bash
# Verify the manifest base version is strictly greater than the highest
# existing tag on the target line.
#   - PR target main             → compare against the highest vX.Y.Z tag overall
#   - PR target maintenance/vX.Y.x → compare against the highest vX.Y.* tag
#
# Expects (set on pull_request / pull_request_target):
#   GITHUB_BASE_REF — target branch name

set -euo pipefail

BASE_REF="${GITHUB_BASE_REF:-}"

if [[ -z "$BASE_REF" ]]; then
  echo "GITHUB_BASE_REF must be set (run inside a pull_request event)."
  exit 1
fi

VERSION=$(jq -r .version package.json)
BASE_VERSION="${VERSION%-rc*}"

# Make sure we have tags from the remote.
git fetch --no-tags --tags origin >/dev/null 2>&1 || true

if [[ "$BASE_REF" =~ ^maintenance/v([0-9]+)\.([0-9]+)\.x$ ]]; then
  MAJOR="${BASH_REMATCH[1]}"
  MINOR="${BASH_REMATCH[2]}"
  PATTERN="v${MAJOR}.${MINOR}.*"
  LINE_DESC="maintenance line $MAJOR.$MINOR"
else
  PATTERN='v[0-9]*.[0-9]*.[0-9]*'
  LINE_DESC="main"
fi

# Highest matching tag (semver-aware sort), trimmed of the leading 'v'.
HIGHEST_TAG=$(git tag --list "$PATTERN" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1 || true)
HIGHEST_VERSION="${HIGHEST_TAG#v}"

if [[ -z "$HIGHEST_VERSION" ]]; then
  echo "OK: no prior tags on $LINE_DESC; $BASE_VERSION is the first release"
  exit 0
fi

if [[ "$BASE_VERSION" == "$HIGHEST_VERSION" ]]; then
  echo "Manifest version $BASE_VERSION matches an existing tag v$HIGHEST_VERSION on $LINE_DESC"
  exit 1
fi

# Use sort -V: if HIGHEST then BASE is already sorted, BASE >= HIGHEST.
# Combined with the equality check above, that means BASE > HIGHEST.
if printf "%s\n%s\n" "$HIGHEST_VERSION" "$BASE_VERSION" | sort -V -C; then
  echo "OK: $BASE_VERSION > $HIGHEST_VERSION (highest on $LINE_DESC)"
  exit 0
fi

echo "Manifest version $BASE_VERSION is not greater than the highest tag on $LINE_DESC ($HIGHEST_VERSION)"
echo "Cannot release a lower version on this line — semver monotonicity violated."
exit 1
