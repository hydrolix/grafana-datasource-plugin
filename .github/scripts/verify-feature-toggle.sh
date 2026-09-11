#!/bin/bash
set -euo pipefail

# Assert that a Grafana feature toggle is in the state the caller asked for.
#
# Used by .github/workflows/nightly-e2e.yml before the suite runs. Without
# this check the toggle axis of that matrix is unfalsifiable: if
# GF_FEATURE_TOGGLES_ENABLE never reached the container, the luxon=true jobs
# would silently re-run the luxon=false configuration and the matrix would
# report green while testing half of what it claims to.
#
# An unset toggle is *absent* from /api/frontend/settings rather than
# present-and-false (verified against 13.3.0: with the toggle off the key is
# missing and zero of the ~68 reported toggles have the value false). That
# makes `.featureToggles[$t] // false` the right read for a real response --
# but it also means a response with NO featureToggles at all would read as
# "toggle is off" and pass vacuously. The shape checks below exist so that
# "off" has to be proven rather than defaulted.
#
# Exit codes are load-bearing and must stay distinct:
#   0  toggle matches EXPECT
#   1  genuine mismatch, or Grafana unreachable / unusable response
#   2  usage error (bad or missing EXPECT) -- a workflow typo, not a finding
#
# Env:
#   EXPECT                 required, "true" or "false"
#   TOGGLE                 toggle name (default: datetime.useLuxon)
#   GRAFANA_URL            default: http://localhost:3000
#   GRAFANA_AUTH           curl -u credentials (default: admin:admin)
#   GRAFANA_SETTINGS_FILE  read settings JSON from this file instead of
#                          querying Grafana. Test seam only.

# Deliberately not `${EXPECT:?}`: that exits 1, colliding with a genuine
# mismatch. An empty EXPECT is the likeliest misconfiguration (a renamed or
# dropped matrix axis renders as ""), so route it through the validation
# below and exit 2 like any other bad input.
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
  # Capture status and body separately. Plain `curl -sf` collapses every
  # distinct cause -- 401 after a password change, 503 from a Grafana that is
  # still starting, a dead container -- into a bare exit code with no output,
  # which on a 14-job matrix is an expensive thing to debug. Worse, it sits
  # next to a real mismatch (exit 1), so an infra blip can be misread as a
  # compatibility regression.
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

# Refuse to certify a toggle state from a response that cannot carry one.
# jq happily indexes null, so without this an auth redirect, a proxy error
# page returning 200, or a future API reshape would all read as "off" and
# pass every luxon=false job -- the exact vacuous green this script exists
# to prevent.
if ! echo "$SETTINGS" | jq -e 'type == "object" and (.featureToggles | type) == "object"' >/dev/null 2>&1; then
  echo "::error::$GRAFANA_URL/api/frontend/settings returned no usable .featureToggles object."
  echo "Cannot verify '$TOGGLE'; the toggle axis for this job would be vacuous. Response head:"
  echo "$SETTINGS" | head -c 2000
  exit 1
fi

# Grafana always reports a non-empty toggle map (~68 entries on 13.3). An
# empty one means this is not a real settings response.
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
  # `// {}` and `|| true` keep this diagnostic from pre-empting the exit code
  # below: under `set -euo pipefail` a jq error here would abort with its own
  # status, replacing the intended 1 on the one path that only runs when
  # something is already wrong.
  echo "$SETTINGS" | jq -r '(.featureToggles // {}) | to_entries[] | select(.value == true) | .key' | sort || true
  exit 1
fi
