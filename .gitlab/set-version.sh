#!/bin/bash
set -euo pipefail

CI_PIPELINE_SOURCE="${CI_PIPELINE_SOURCE:-}"
CI_MERGE_REQUEST_SOURCE_BRANCH_NAME="${CI_MERGE_REQUEST_SOURCE_BRANCH_NAME:-}"
CI_MERGE_REQUEST_TARGET_BRANCH_NAME="${CI_MERGE_REQUEST_TARGET_BRANCH_NAME:-}"

PACKAGE_FILE="package.json"
VERSION=$(grep '"version"' "$PACKAGE_FILE" | head -1 | sed -E 's/.*"version": *"([^"]+)".*/\1/')

IS_MERGE_REQUEST=false
IS_RELEASE_SOURCE_BRANCH=false
IS_MAIN_TARGET_BRANCH=false

[[ "$CI_PIPELINE_SOURCE" == "merge_request_event" ]] && IS_MERGE_REQUEST=true
[[ "$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME" =~ ^release[/_-].+ ]] && IS_RELEASE_SOURCE_BRANCH=true
[[ "$CI_MERGE_REQUEST_TARGET_BRANCH_NAME" == "main" ]] && IS_MAIN_TARGET_BRANCH=true

if $IS_MERGE_REQUEST && $IS_RELEASE_SOURCE_BRANCH && $IS_MAIN_TARGET_BRANCH; then
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
