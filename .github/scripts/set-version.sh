#!/bin/bash
set -euo pipefail

GITHUB_HEAD_REF="${GITHUB_HEAD_REF:-}"
GITHUB_REF_NAME="${GITHUB_REF_NAME:-}"

PACKAGE_FILE="package.json"
VERSION=$(grep '"version"' "$PACKAGE_FILE" | head -1 | sed -E 's/.*"version": *"([^"]+)".*/\1/')

IS_RELEASE_SOURCE_BRANCH=false
IS_HOTFIX_SOURCE_BRANCH=false

[[ "$GITHUB_HEAD_REF" =~ ^release[/_-].+ ]] && IS_RELEASE_SOURCE_BRANCH=true
[[ "$GITHUB_HEAD_REF" =~ ^hotfix[/_-].+ ]] && IS_HOTFIX_SOURCE_BRANCH=true

[[ "$GITHUB_REF_NAME" =~ ^release[/_-].+ ]] && IS_RELEASE_SOURCE_BRANCH=true
[[ "$GITHUB_REF_NAME" =~ ^hotfix[/_-].+ ]] && IS_HOTFIX_SOURCE_BRANCH=true

if $IS_RELEASE_SOURCE_BRANCH || $IS_HOTFIX_SOURCE_BRANCH; then
  # Release/hotfix branches carry either a clean version (X.Y.Z) when
  # promoting to stable, or an rc version (X.Y.Z-rcN) while iterating.
  if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)?$ ]]; then
    echo "Invalid version \"$VERSION\": expected X.Y.Z or X.Y.Z-rcN on $GITHUB_HEAD_REF$GITHUB_REF_NAME"
    exit 1
  fi
else
  if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+-dev$ ]]; then
    echo "Invalid version \"$VERSION\": expected format X.Y.Z-dev"
    exit 1
  fi

  GIT_SHA=$(git rev-parse --short=8 HEAD)
  NEW_VERSION="${VERSION}+${GIT_SHA}"

  sed -i -E 's/("version": *")[^"]+(")/\1'"$NEW_VERSION"'\2/' "$PACKAGE_FILE"
fi