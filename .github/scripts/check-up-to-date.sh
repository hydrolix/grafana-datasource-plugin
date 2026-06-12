#!/usr/bin/env bash
# Verify the PR head contains the tip of the target branch (the same property
# GitHub's "Require branches up to date before merging" enforces, replicated in
# the workflow so failures are visible in CI logs).
#
# Expects (set on pull_request / pull_request_target):
#   GITHUB_BASE_REF       — target branch name
#   GITHUB_HEAD_REF       — source branch name (for the error message)
#   GITHUB_PR_HEAD_SHA    — explicit PR head SHA (optional; falls back to HEAD)

set -euo pipefail

BASE_REF="${GITHUB_BASE_REF:-}"
HEAD_REF="${GITHUB_HEAD_REF:-}"
HEAD_SHA="${GITHUB_PR_HEAD_SHA:-$(git rev-parse HEAD)}"

if [[ -z "$BASE_REF" ]]; then
  echo "GITHUB_BASE_REF must be set (run inside a pull_request event)."
  exit 1
fi

git fetch --no-tags origin "$BASE_REF" >/dev/null 2>&1
BASE_SHA=$(git rev-parse "origin/$BASE_REF")

if git merge-base --is-ancestor "$BASE_SHA" "$HEAD_SHA"; then
  echo "OK: $HEAD_REF contains $BASE_REF tip ($BASE_SHA)"
  exit 0
fi

echo "Branch $HEAD_REF is not up to date with $BASE_REF"
echo "  $BASE_REF tip: $BASE_SHA"
echo "  PR head:      $HEAD_SHA"
echo
echo "Run locally:"
echo "  git fetch origin $BASE_REF"
echo "  git merge origin/$BASE_REF"
echo "  git push"
exit 1
