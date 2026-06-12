#!/usr/bin/env bash
# Verify CHANGELOG.md has a non-empty section for the current manifest base
# version. Tolerates the headings `## X.Y.Z`, `## vX.Y.Z`, and `## [X.Y.Z]`.

set -euo pipefail

CHANGELOG="${1:-CHANGELOG.md}"

if [[ ! -f "$CHANGELOG" ]]; then
  echo "Changelog file not found: $CHANGELOG"
  exit 1
fi

VERSION=$(jq -r .version package.json)
BASE_VERSION="${VERSION%-rc*}"

# Pull the body of the first section whose heading matches the base version,
# stopping at the next ## heading.
SECTION=$(awk -v v="$BASE_VERSION" '
  BEGIN { inside = 0; body = "" }
  /^## / {
    if (inside) exit
    # Strip the "## " prefix and any surrounding [] or leading v
    h = $0; sub(/^## +/, "", h); gsub(/[][]/, "", h); sub(/^v/, "", h)
    if (h == v) { inside = 1; next }
  }
  inside { body = body $0 "\n" }
  END { printf "%s", body }
' "$CHANGELOG")

if [[ -z "$SECTION" ]]; then
  echo "CHANGELOG.md has no section for version $BASE_VERSION"
  echo "Expected one of: \"## $BASE_VERSION\", \"## v$BASE_VERSION\", or \"## [$BASE_VERSION]\""
  exit 1
fi

# Strip whitespace; require at least one non-empty line of content.
CONTENT=$(echo "$SECTION" | sed -E 's/^[[:space:]]+$//' | tr -d '[:space:]')
if [[ -z "$CONTENT" ]]; then
  echo "CHANGELOG.md section for $BASE_VERSION is empty"
  exit 1
fi

echo "OK: CHANGELOG.md has content for $BASE_VERSION"
