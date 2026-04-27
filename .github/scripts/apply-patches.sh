#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PATCH_DIR="$PROJECT_DIR/patches"

apply_patch() {
    local module="$1"
    local patch_file="$2"

    local mod_dir
    mod_dir=$(go list -m -f '{{.Dir}}' "$module")

    # Make module cache writable so we can patch it
    chmod -R u+w "$mod_dir"

    # Extract the unified diff portion (skip IntelliJ IDEA headers).
    local diff
    diff=$(sed -n '/^diff --git/,$p' "$patch_file")

    # Check if the patch is already applied (reverse applies cleanly).
    if echo "$diff" | patch -d "$mod_dir" -p1 -R --dry-run --silent >/dev/null 2>&1; then
        echo "Already applied: $(basename "$patch_file") -> $module"
        return 0
    fi

    # Apply the patch.
    if echo "$diff" | patch -d "$mod_dir" -p1 --silent; then
        echo "Applied: $(basename "$patch_file") -> $module"
    else
        echo "ERROR: Failed to apply patch: $(basename "$patch_file") -> $module" >&2
        exit 1
    fi
}

echo "Downloading Go dependencies..."
go mod download

echo "Applying patches to Go dependencies..."
apply_patch "github.com/ClickHouse/clickhouse-go/v2" \
    "$PATCH_DIR/clickhouse-disable-revision.patch"

echo "Done."
