#!/bin/bash
set -uo pipefail

# Tests for verify-feature-toggle.sh. Uses the GRAFANA_SETTINGS_FILE seam so
# no Grafana is needed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERIFY="$SCRIPT_DIR/verify-feature-toggle.sh"

FIXTURE_DIR=$(mktemp -d)
trap 'rm -rf "$FIXTURE_DIR"' EXIT

FAILURES=0

# The two shapes Grafana actually returns: an enabled toggle is present and
# true; a disabled one is absent entirely, never present-and-false.
cat > "$FIXTURE_DIR/on.json" <<'JSON'
{"featureToggles": {"datetime.useLuxon": true, "someOther": true}}
JSON
cat > "$FIXTURE_DIR/off.json" <<'JSON'
{"featureToggles": {"someOther": true}}
JSON
# Defensive: some builds may report it explicitly false.
cat > "$FIXTURE_DIR/explicit-false.json" <<'JSON'
{"featureToggles": {"datetime.useLuxon": false}}
JSON

expect_exit() {
  local desc=$1 want=$2 settings=$3 expect=$4
  EXPECT="$expect" GRAFANA_SETTINGS_FILE="$FIXTURE_DIR/$settings" \
    "$VERIFY" >/dev/null 2>&1
  local got=$?
  if [[ "$got" -eq "$want" ]]; then
    echo "ok   - $desc"
  else
    echo "FAIL - $desc (wanted exit $want, got $got)"
    FAILURES=$((FAILURES + 1))
  fi
}

expect_exit "toggle on, expected on"                0 on.json             true
expect_exit "toggle absent, expected off"           0 off.json            false
expect_exit "explicit false, expected off"          0 explicit-false.json false
expect_exit "toggle on but expected off"            1 on.json             false
expect_exit "toggle absent but expected on"         1 off.json            true
expect_exit "explicit false but expected on"        1 explicit-false.json true

# A malformed EXPECT must be a usage error (2), distinct from a genuine
# mismatch (1) -- otherwise a typo in the matrix would read as a real failure.
expect_exit "invalid EXPECT value"                  2 on.json             yes

# The failure path must name the enabled toggles, so a red job says what the
# container actually had rather than only what it lacked.
OUT=$(EXPECT=false GRAFANA_SETTINGS_FILE="$FIXTURE_DIR/on.json" "$VERIFY" 2>&1)
if echo "$OUT" | grep -q "someOther"; then
  echo "ok   - mismatch output lists enabled toggles"
else
  echo "FAIL - mismatch output should list enabled toggles"
  FAILURES=$((FAILURES + 1))
fi

if [[ "$FAILURES" -gt 0 ]]; then
  echo "$FAILURES test(s) failed"
  exit 1
fi
echo "all verify-feature-toggle tests passed"
