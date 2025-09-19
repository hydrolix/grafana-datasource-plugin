#!/bin/bash
set -euo pipefail

GITHUB_EVENT_NAME="${GITHUB_EVENT_NAME:-}"
GITHUB_HEAD_REF="${GITHUB_HEAD_REF:-}"
GITHUB_BASE_REF="${GITHUB_BASE_REF:-}"

PACKAGE_FILE="package.json"
VERSION=$(grep '"version"' "$PACKAGE_FILE" | head -1 | sed -E 's/.*"version": *"([^"]+)".*/\1/')

IS_PULL_REQUEST=false
IS_RELEASE_SOURCE_BRANCH=false
IS_HOTFIX_SOURCE_BRANCH=false
IS_MAIN_TARGET_BRANCH=false

[[ "$GITHUB_EVENT_NAME" == "pull_request" ]] && IS_PULL_REQUEST=true
[[ "$GITHUB_HEAD_REF" =~ ^release[/_-].+ ]] && IS_RELEASE_SOURCE_BRANCH=true
[[ "$GITHUB_HEAD_REF" =~ ^hotfix[/_-].+ ]] && IS_HOTFIX_SOURCE_BRANCH=true
[[ "$GITHUB_BASE_REF" == "main" ]] && IS_MAIN_TARGET_BRANCH=true

if $IS_PULL_REQUEST && { $IS_RELEASE_SOURCE_BRANCH || $IS_HOTFIX_SOURCE_BRANCH; } && $IS_MAIN_TARGET_BRANCH; then
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
  NEW_VERSION="${VERSION}+${GIT_SHA}"

  sed -i -E 's/("version": *")[^"]+(")/\1'"$NEW_VERSION"'\2/' "$PACKAGE_FILE"
fi