#!/usr/bin/env bash
# Extract a section from CHANGELOG.md.
#   parse-changelog.sh <file>                 → topmost ## section
#   parse-changelog.sh <file> <version>       → the section whose heading
#                                                matches "## X.Y.Z",
#                                                "## vX.Y.Z", or "## [X.Y.Z]"

set -euo pipefail

if [[ "$#" -lt 1 || "$#" -gt 2 ]]; then
  echo "Usage: $0 <changelog-file> [version]"
  exit 1
fi

FILE="$1"
TARGET="${2:-}"

if [[ -z "$TARGET" ]]; then
  awk '
    /^## / { if (section) exit; section = 1 }
    section { print }
  ' "$FILE"
  exit 0
fi

awk -v v="$TARGET" '
  /^## / {
    if (inside) exit
    h = $0; sub(/^## +/, "", h); gsub(/[][]/, "", h); sub(/^v/, "", h)
    if (h == v) { inside = 1; print; next }
  }
  inside { print }
' "$FILE"
