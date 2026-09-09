#!/bin/bash
set -euo pipefail

GITHUB_HEAD_REF="${GITHUB_HEAD_REF:-}"
GITHUB_REF_NAME="${GITHUB_REF_NAME:-}"

PACKAGE_FILE="package.json"
VERSION=$(jq -r .version "$PACKAGE_FILE")

# Portable in-place rewrite: BSD sed's -i takes a mandatory suffix, so a bare
# `sed -i` breaks when the script (and its test harness) runs on macOS.
write_version() {
  local tmp
  tmp=$(mktemp)
  sed -E 's/("version": *")[^"]+(")/\1'"$1"'\2/' "$PACKAGE_FILE" > "$tmp"
  mv "$tmp" "$PACKAGE_FILE"
}

# Tag refs are publication sources: each shape must agree with package.json
# (and, for dev tags, with the tagged commit) before anything is signed.
if [[ "$GITHUB_REF_NAME" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
  TAG_VERSION="${BASH_REMATCH[1]}"
  if [[ "$VERSION" != "$TAG_VERSION" ]]; then
    echo "Release tag \"$GITHUB_REF_NAME\" disagrees with package.json version \"$VERSION\""
    exit 1
  fi
  exit 0
fi

if [[ "$GITHUB_REF_NAME" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)-rc\.[0-9]+$ ]]; then
  TAG_BASE_VERSION="${BASH_REMATCH[1]}"
  if [[ "$VERSION" != "$TAG_BASE_VERSION" ]]; then
    echo "RC tag \"$GITHUB_REF_NAME\" (base \"$TAG_BASE_VERSION\") disagrees with package.json version \"$VERSION\""
    exit 1
  fi
  write_version "${GITHUB_REF_NAME#v}"
  exit 0
fi

if [[ "$GITHUB_REF_NAME" =~ ^v([0-9]+\.[0-9]+\.[0-9]+-dev)\+([0-9a-f]{8})$ ]]; then
  TAG_BASE_VERSION="${BASH_REMATCH[1]}"
  TAG_SHA="${BASH_REMATCH[2]}"
  if [[ "$VERSION" != "$TAG_BASE_VERSION" ]]; then
    echo "Dev tag \"$GITHUB_REF_NAME\" (base \"$TAG_BASE_VERSION\") disagrees with package.json version \"$VERSION\""
    exit 1
  fi
  # A prefix check against the full SHA, not equality against the 8-char
  # short SHA: `git rev-parse --short=8` returns *at least* eight characters
  # and silently lengthens on ambiguity, which would make an equality check
  # reject an honest tag.
  FULL_SHA=$(git rev-parse HEAD)
  if [[ "$FULL_SHA" != "$TAG_SHA"* ]]; then
    echo "Dev tag SHA \"$TAG_SHA\" disagrees with the tagged commit's SHA \"$FULL_SHA\""
    exit 1
  fi
  write_version "${TAG_BASE_VERSION}+${TAG_SHA}"
  exit 0
fi

IS_RELEASE_SOURCE_BRANCH=false
IS_HOTFIX_SOURCE_BRANCH=false

[[ "$GITHUB_HEAD_REF" =~ ^release[/_-].+ ]] && IS_RELEASE_SOURCE_BRANCH=true
[[ "$GITHUB_HEAD_REF" =~ ^hotfix[/_-].+ ]] && IS_HOTFIX_SOURCE_BRANCH=true

[[ "$GITHUB_REF_NAME" =~ ^release[/_-].+ ]] && IS_RELEASE_SOURCE_BRANCH=true
[[ "$GITHUB_REF_NAME" =~ ^hotfix[/_-].+ ]] && IS_HOTFIX_SOURCE_BRANCH=true

if $IS_RELEASE_SOURCE_BRANCH || $IS_HOTFIX_SOURCE_BRANCH; then
  if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Invalid version \"$VERSION\": expected format X.X.X"
    exit 1
  fi
else
  if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+-dev$ ]]; then
    echo "Invalid version \"$VERSION\": expected format X.X.X-dev"
    exit 1
  fi

  GIT_SHA=$(git rev-parse --short=8 HEAD)
  write_version "${VERSION}+${GIT_SHA}"
fi
