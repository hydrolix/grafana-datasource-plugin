#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=s3-config.sh
source "$SCRIPT_DIR/s3-config.sh"

PATH_PREFIX="${PATH_PREFIX:-}"

ZIP_PATH=$(find . -maxdepth 1 -name '*.zip' | head -n 1)

if [ -z "$ZIP_PATH" ]; then
    echo "No zip file found in the current directory"
    exit 1
fi

ZIP_NAME=$(basename "$ZIP_PATH")

PACKAGE_NAME="${ZIP_NAME%-*.*.zip}"
PACKAGE_VERSION=$(echo "$ZIP_NAME" | sed -E 's/^.*-([0-9]+\.[0-9]+\.[0-9]+[^/]*)\.zip$/\1/')

if [ -z "$PACKAGE_NAME" ] || [ -z "$PACKAGE_VERSION" ]; then
    echo "Failed to extract package name or version"
    exit 1
fi

PATH_SUFFIX="grafana-datasource-plugin/${PATH_PREFIX}${ZIP_NAME}"

S3_PATH="s3://${S3_BUCKET}/$PATH_SUFFIX"
PUBLIC_PATH="${S3_PUBLIC_BASE_URL}/${PATH_SUFFIX/+/%2B}"

echo "Uploading $ZIP_NAME to $S3_PATH ..."
aws s3 cp "$ZIP_NAME" "$S3_PATH"

# Digest round-trip: fetch over the public HTTPS URL rather than an
# authenticated `aws s3 cp` so the check also proves public reachability and
# URL encoding, not just that the bucket API returned the right bytes.
ROUNDTRIP_DIR=$(mktemp -d)
curl -fsSL -o "$ROUNDTRIP_DIR/$ZIP_NAME" "$PUBLIC_PATH"
LOCAL_DIGEST=$(sha256sum "$ZIP_NAME" | cut -d' ' -f1)
REMOTE_DIGEST=$(sha256sum "$ROUNDTRIP_DIR/$ZIP_NAME" | cut -d' ' -f1)
rm -rf "$ROUNDTRIP_DIR"

if [ "$LOCAL_DIGEST" != "$REMOTE_DIGEST" ]; then
    echo "S3 copy sha256 \"$REMOTE_DIGEST\" does not match local ZIP sha256 \"$LOCAL_DIGEST\""
    exit 1
fi

echo "Verified $PUBLIC_PATH (sha256:$LOCAL_DIGEST)"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    {
        echo "## S3 publish"
        echo "- Object: $PUBLIC_PATH"
        echo "- sha256: \`$LOCAL_DIGEST\`"
    } >> "$GITHUB_STEP_SUMMARY"
fi

echo "Run curl -O $PUBLIC_PATH to get it"
