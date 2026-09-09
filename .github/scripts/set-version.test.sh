#!/bin/bash
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SET_VERSION="$SCRIPT_DIR/set-version.sh"

FIXTURE_DIR=$(mktemp -d)
trap 'rm -rf "$FIXTURE_DIR"' EXIT

FAILURES=0
HEAD_SHA8=""

setup_fixture() {
  local version=$1
  rm -rf "$FIXTURE_DIR"
  mkdir -p "$FIXTURE_DIR"
  cat > "$FIXTURE_DIR/package.json" <<EOF
{
  "name": "hydrolix-hydrolix-datasource",
  "version": "$version"
}
EOF
  git -C "$FIXTURE_DIR" init -q
  git -C "$FIXTURE_DIR" -c user.name=test -c user.email=test@test add package.json
  git -C "$FIXTURE_DIR" -c user.name=test -c user.email=test@test commit -qm fixture
  HEAD_SHA8=$(git -C "$FIXTURE_DIR" rev-parse --short=8 HEAD)
}

fixture_version() {
  sed -En 's/.*"version": *"([^"]+)".*/\1/p' "$FIXTURE_DIR/package.json" | head -1
}

# run_case <name> <pkg-version> <ref-name> <head-ref> <expected-exit> <expected-version> [expected-substr...]
# "__SHA8__" in <ref-name> / <expected-version> / <expected-substr> is replaced
# with the fixture HEAD's short SHA; <expected-version> is checked only when
# <expected-exit> is 0; "nonzero" means any non-zero exit.
run_case() {
  local name=$1 version=$2 ref_name=$3 head_ref=$4 expected_exit=$5 expected_version=$6
  shift 6

  setup_fixture "$version"
  ref_name="${ref_name//__SHA8__/$HEAD_SHA8}"
  expected_version="${expected_version//__SHA8__/$HEAD_SHA8}"

  local output exit_code=0
  output=$(cd "$FIXTURE_DIR" && GITHUB_REF_NAME="$ref_name" GITHUB_HEAD_REF="$head_ref" "$SET_VERSION" 2>&1) || exit_code=$?

  local failed=false
  if [[ "$expected_exit" == "nonzero" ]]; then
    if [[ "$exit_code" -eq 0 ]]; then
      echo "FAIL: $name — expected non-zero exit, got 0"
      failed=true
    fi
  elif [[ "$exit_code" -ne "$expected_exit" ]]; then
    echo "FAIL: $name — expected exit $expected_exit, got $exit_code"
    failed=true
  fi

  if [[ "$expected_exit" == "0" && "$failed" == false ]]; then
    local actual_version
    actual_version=$(fixture_version)
    if [[ "$actual_version" != "$expected_version" ]]; then
      echo "FAIL: $name — expected version \"$expected_version\", got \"$actual_version\""
      failed=true
    fi
  fi

  local substr
  for substr in "$@"; do
    substr="${substr//__SHA8__/$HEAD_SHA8}"
    if [[ "$output" != *"$substr"* ]]; then
      echo "FAIL: $name — output missing \"$substr\""
      echo "  output: $output"
      failed=true
    fi
  done

  if [[ "$failed" == true ]]; then
    FAILURES=$((FAILURES + 1))
  else
    echo "PASS: $name"
  fi
}

# Tag shapes
run_case "release tag matching package.json" \
  "0.12.0" "v0.12.0" "" 0 "0.12.0"
run_case "release tag mismatching package.json names both values" \
  "0.12.0" "v9.9.9" "" nonzero "" "v9.9.9" "0.12.0"
run_case "release tag against a -dev version" \
  "0.12.0-dev" "v0.12.0" "" nonzero "" "v0.12.0" "0.12.0-dev"
run_case "rc tag matching base version rewrites to rc form" \
  "0.12.0" "v0.12.0-rc.1" "" 0 "0.12.0-rc.1"
run_case "rc tag with mismatched base version" \
  "0.13.0" "v0.12.0-rc.1" "" nonzero "" "v0.12.0-rc.1" "0.13.0"
run_case "dev tag matching HEAD rewrites the version" \
  "0.12.0-dev" "v0.12.0-dev+__SHA8__" "" 0 "0.12.0-dev+__SHA8__"
run_case "dev tag with wrong SHA names both SHAs" \
  "0.12.0-dev" "v0.12.0-dev+deadbeef" "" nonzero "" "deadbeef" "__SHA8__"
run_case "dev tag against a non-dev version" \
  "0.12.0" "v0.12.0-dev+__SHA8__" "" nonzero "" "v0.12.0-dev+__SHA8__" "0.12.0"

# Branch refs (existing behaviour, unchanged)
run_case "release branch head ref keeps the plain version" \
  "0.12.0" "" "release/v0.12.0" 0 "0.12.0"
run_case "hotfix branch head ref keeps the plain version" \
  "0.12.1" "" "hotfix/v0.12.1" 0 "0.12.1"
run_case "feature branch dev build gets a SHA suffix" \
  "0.12.0-dev" "" "feature/HDX-12163-release-tag-provenance" 0 "0.12.0-dev+__SHA8__"

echo
if [[ "$FAILURES" -gt 0 ]]; then
  echo "$FAILURES case(s) failed"
  exit 1
fi
echo "All cases passed"
