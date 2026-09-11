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
# Note an unset toggle is *absent* from /api/frontend/settings rather than
# present-and-false, hence the `// false` default below.
#
# Env:
#   EXPECT                 required, "true" or "false"
#   TOGGLE                 toggle name (default: datetime.useLuxon)
#   GRAFANA_URL            default: http://localhost:3000
#   GRAFANA_AUTH           curl -u credentials (default: admin:admin)
#   GRAFANA_SETTINGS_FILE  read settings JSON from this file instead of
#                          querying Grafana. Test seam only.

EXPECT="${EXPECT:?EXPECT must be set to \"true\" or \"false\"}"
TOGGLE="${TOGGLE:-datetime.useLuxon}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_AUTH="${GRAFANA_AUTH:-admin:admin}"
GRAFANA_SETTINGS_FILE="${GRAFANA_SETTINGS_FILE:-}"

if [[ "$EXPECT" != "true" && "$EXPECT" != "false" ]]; then
  echo "EXPECT must be \"true\" or \"false\", got \"$EXPECT\"" >&2
  exit 2
fi

if [[ -n "$GRAFANA_SETTINGS_FILE" ]]; then
  SETTINGS=$(cat "$GRAFANA_SETTINGS_FILE")
else
  SETTINGS=$(curl -sf -u "$GRAFANA_AUTH" "$GRAFANA_URL/api/frontend/settings")
fi

ACTUAL=$(echo "$SETTINGS" | jq -r --arg t "$TOGGLE" '.featureToggles[$t] // false')
echo "$TOGGLE: expected=$EXPECT actual=$ACTUAL"

if [[ "$ACTUAL" != "$EXPECT" ]]; then
  echo "::error::$TOGGLE is '$ACTUAL' but the matrix asked for '$EXPECT'."
  echo "GF_FEATURE_TOGGLES_ENABLE did not reach the Grafana container as expected."
  echo "Enabled toggles reported by Grafana:"
  echo "$SETTINGS" | jq -r '.featureToggles | to_entries[] | select(.value == true) | .key' | sort
  exit 1
fi
