#!/bin/bash
set -euo pipefail

# Assert a Grafana feature toggle is in the state the caller asked for, so the
# toggle axis of nightly-e2e.yml cannot silently collapse into one dimension.
#
# An unset toggle is ABSENT from /api/frontend/settings, not present-and-false.
# That makes `// false` the right read for a real response, but it also means a
# response with no featureToggles at all would read as "off" and pass
# vacuously — hence the shape checks.
#
# Exit codes are load-bearing: 0 match, 1 mismatch or unusable response,
# 2 usage error (a workflow typo, not a finding).
#
# Env: EXPECT (required, true|false), TOGGLE, GRAFANA_URL, GRAFANA_AUTH,
#      GRAFANA_SETTINGS_FILE (test seam — read JSON from a file).

# Not `${EXPECT:?}`: that exits 1, colliding with a genuine mismatch.
EXPECT="${EXPECT:-}"
TOGGLE="${TOGGLE:-datetime.useLuxon}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_AUTH="${GRAFANA_AUTH:-admin:admin}"
GRAFANA_SETTINGS_FILE="${GRAFANA_SETTINGS_FILE:-}"

if [[ "$EXPECT" != "true" && "$EXPECT" != "false" ]]; then
  echo "::error::EXPECT must be \"true\" or \"false\", got \"$EXPECT\"." >&2
  echo "This is a workflow configuration error, not a compatibility finding." >&2
  exit 2
fi

if [[ -n "$GRAFANA_SETTINGS_FILE" ]]; then
  SETTINGS=$(cat "$GRAFANA_SETTINGS_FILE")
else
  # Plain `curl -sf` collapses 401/503/dead-container into a bare exit code
  # with no output, sitting right next to a real mismatch (exit 1).
  BODY_FILE=$(mktemp)
  trap 'rm -f "$BODY_FILE"' EXIT
  HTTP_CODE=$(curl -s -o "$BODY_FILE" -w '%{http_code}' \
    -u "$GRAFANA_AUTH" "$GRAFANA_URL/api/frontend/settings") || {
      echo "::error::Could not reach $GRAFANA_URL/api/frontend/settings (curl exit $?)."
      echo "The toggle axis for this job was NOT verified."
      exit 1
    }
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "::error::$GRAFANA_URL/api/frontend/settings returned HTTP $HTTP_CODE."
    echo "The toggle axis for this job was NOT verified. Response head:"
    head -c 2000 "$BODY_FILE"
    exit 1
  fi
  SETTINGS=$(cat "$BODY_FILE")
fi

# jq indexes null happily, so without this an auth redirect or a 200 error
# page reads as "off" and passes every luxon=false job.
if ! echo "$SETTINGS" | jq -e 'type == "object" and (.featureToggles | type) == "object"' >/dev/null 2>&1; then
  echo "::error::$GRAFANA_URL/api/frontend/settings returned no usable .featureToggles object."
  echo "Cannot verify '$TOGGLE'; the toggle axis for this job would be vacuous. Response head:"
  echo "$SETTINGS" | head -c 2000
  exit 1
fi

# Grafana always reports a non-empty toggle map; an empty one is not a real
# settings response.
if [[ "$(echo "$SETTINGS" | jq -r '.featureToggles | length')" -eq 0 ]]; then
  echo "::error::.featureToggles is empty -- refusing to certify '$TOGGLE=$EXPECT' from a degenerate response."
  exit 1
fi

ACTUAL=$(echo "$SETTINGS" | jq -r --arg t "$TOGGLE" '.featureToggles[$t] // false')
echo "$TOGGLE: expected=$EXPECT actual=$ACTUAL"

if [[ "$ACTUAL" != "$EXPECT" ]]; then
  echo "::error::$TOGGLE is '$ACTUAL' but the matrix asked for '$EXPECT'."
  echo "GF_FEATURE_TOGGLES_ENABLE did not reach the Grafana container as expected."
  echo "Enabled toggles reported by Grafana:"
  # `// {}` and `|| true` stop a jq error here from replacing the exit 1 below.
  echo "$SETTINGS" | jq -r '(.featureToggles // {}) | to_entries[] | select(.value == true) | .key' | sort || true
  exit 1
fi
