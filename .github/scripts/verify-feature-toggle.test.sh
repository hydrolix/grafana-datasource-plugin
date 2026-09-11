#!/bin/bash
set -uo pipefail

# Tests for verify-feature-toggle.sh. Uses the GRAFANA_SETTINGS_FILE seam so
# no Grafana is needed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERIFY="$SCRIPT_DIR/verify-feature-toggle.sh"

FIXTURE_DIR=$(mktemp -d)
trap 'rm -rf "$FIXTURE_DIR"' EXIT

FAILURES=0

# The two shapes Grafana returns: enabled is present-and-true, disabled is
# absent entirely.
cat > "$FIXTURE_DIR/on.json" <<'JSON'
{"featureToggles": {"datetime.useLuxon": true, "someOther": true}}
JSON
cat > "$FIXTURE_DIR/off.json" <<'JSON'
{"featureToggles": {"someOther": true, "third": true}}
JSON
# Defensive: some builds may report it explicitly false.
cat > "$FIXTURE_DIR/explicit-false.json" <<'JSON'
{"featureToggles": {"datetime.useLuxon": false, "someOther": true}}
JSON
# Degenerate shapes — each once read as "off" and passed vacuously.
echo '{}'                                  > "$FIXTURE_DIR/no-toggles.json"
echo '{"featureToggles": {}}'              > "$FIXTURE_DIR/empty-toggles.json"
echo '{"message": "Unauthorized"}'         > "$FIXTURE_DIR/error-body.json"
echo '{"featureToggles": "not-an-object"}' > "$FIXTURE_DIR/wrong-type.json"
echo 'not json at all'                     > "$FIXTURE_DIR/malformed.json"

# expect_exit <desc> <want-exit> <fixture> <EXPECT> [TOGGLE] [substring...]
expect_exit() {
  local desc=$1 want=$2 settings=$3 expect=$4 toggle=${5:-datetime.useLuxon}
  shift 5 2>/dev/null || shift 4
  local out got
  out=$(EXPECT="$expect" TOGGLE="$toggle" GRAFANA_SETTINGS_FILE="$FIXTURE_DIR/$settings" \
    "$VERIFY" 2>&1)
  got=$?

  if [[ "$got" -ne "$want" ]]; then
    echo "FAIL - $desc (wanted exit $want, got $got)"
    FAILURES=$((FAILURES + 1))
    return
  fi
  local needle
  for needle in "$@"; do
    if ! grep -qF -- "$needle" <<<"$out"; then
      echo "FAIL - $desc (output missing \"$needle\")"
      FAILURES=$((FAILURES + 1))
      return
    fi
  done
  echo "ok   - $desc"
}

# --- matching / mismatching -------------------------------------------------
expect_exit "toggle on, expected on"           0 on.json             true
expect_exit "toggle absent, expected off"      0 off.json            false
expect_exit "explicit false, expected off"     0 explicit-false.json false
expect_exit "toggle on but expected off"       1 on.json             false
expect_exit "toggle absent but expected on"    1 off.json            true
expect_exit "explicit false but expected on"   1 explicit-false.json true

# --- usage errors must be exit 2, never 1 (a typo is not a finding) --------
expect_exit "invalid EXPECT value"             2 on.json             yes
expect_exit "empty EXPECT is a usage error"    2 on.json             ""

# --- degenerate responses must never certify "off" -------------------------
expect_exit "missing featureToggles is not 'off'" 1 no-toggles.json    false
expect_exit "empty featureToggles is not 'off'"   1 empty-toggles.json false
expect_exit "error body is not 'off'"             1 error-body.json    false
expect_exit "non-object featureToggles is not 'off'" 1 wrong-type.json false
expect_exit "malformed JSON is not 'off'"         1 malformed.json     false
# ...and must not certify "on" either.
expect_exit "missing featureToggles is not 'on'"  1 no-toggles.json    true

# --- TOGGLE is honoured, not hardcoded --------------------------------------
expect_exit "honours a non-default TOGGLE (on)"  0 on.json  true  someOther
expect_exit "honours a non-default TOGGLE (off)" 1 off.json false someOther

# --- diagnostics on the failure path ----------------------------------------
expect_exit "mismatch lists enabled toggles" 1 on.json false datetime.useLuxon "someOther"
expect_exit "mismatch emits a GitHub error annotation" 1 on.json false datetime.useLuxon "::error::"
# Listing disabled toggles as enabled would send a debugger the wrong way.
expect_exit "diagnostic excludes disabled toggles" 1 explicit-false.json true datetime.useLuxon "someOther"
OUT=$(EXPECT=true GRAFANA_SETTINGS_FILE="$FIXTURE_DIR/explicit-false.json" "$VERIFY" 2>&1)
if grep -q "^datetime.useLuxon$" <<<"$OUT"; then
  echo "FAIL - diagnostic listed a disabled toggle as enabled"
  FAILURES=$((FAILURES + 1))
else
  echo "ok   - diagnostic omits the disabled toggle"
fi

if [[ "$FAILURES" -gt 0 ]]; then
  echo "$FAILURES test(s) failed"
  exit 1
fi
echo "all verify-feature-toggle tests passed"
