#!/bin/bash
set -uo pipefail

# Tests for record-grafana-image.sh, via a `docker` PATH shim so no daemon
# and no images are needed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RECORD="$SCRIPT_DIR/record-grafana-image.sh"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/bin"

FAILURES=0

# $1 = image the container reports ("" = inspect fails)
# $2 = digest for that image     ("" = no RepoDigests)
make_docker_shim() {
  cat > "$TMP/bin/docker" <<SHIM
#!/bin/bash
# Args: inspect --format='<tpl>' <target>
target="\${!#}"
case "\$2" in
  *Config.Image*)
    [[ -z '$1' ]] && { echo >&2 "error: no such object: \$target"; echo; exit 1; }
    echo '$1' ;;
  *RepoDigests*)
    [[ -z '$2' ]] && { echo; exit 1; }
    echo '$2' ;;
esac
SHIM
  chmod +x "$TMP/bin/docker"
}

run_case() {  # desc, image, digest, then assertions as KEY=VALUE or !SUBSTRING
  local desc=$1 image=$2 digest=$3; shift 3
  make_docker_shim "$image" "$digest"
  local out summary
  summary="$TMP/summary.md"; : > "$summary"
  out=$(PATH="$TMP/bin:$PATH" GRAFANA_VERSION=13.2.1 LUXON=true CONTAINER=grafana \
        GF_FEATURE_TOGGLES_ENABLE=datetime.useLuxon \
        OUTPUT_FILE="$TMP/out.txt" GITHUB_STEP_SUMMARY="$summary" "$RECORD" 2>&1)

  # The file must always be exactly three key=value lines — a stray newline
  # from docker corrupts it, which is what the CR/LF strips guard.
  local lines; lines=$(wc -l < "$TMP/out.txt" | tr -d ' ')
  if [[ "$lines" != "3" ]]; then
    echo "FAIL - $desc (out.txt has $lines lines, want 3)"; FAILURES=$((FAILURES+1)); return
  fi

  local a
  for a in "$@"; do
    if [[ "$a" == '!'* ]]; then
      # Negative: must appear in neither the annotations nor the file.
      if grep -qF -- "${a:1}" <<<"$out" || grep -qF -- "${a:1}" "$TMP/out.txt"; then
        echo "FAIL - $desc (unexpected output \"${a:1}\")"; FAILURES=$((FAILURES+1)); return
      fi
    elif [[ "$a" == '::'* ]]; then
      # Annotation: substring of stdout.
      if ! grep -qF -- "$a" <<<"$out"; then
        echo "FAIL - $desc (missing annotation \"$a\")"; FAILURES=$((FAILURES+1)); return
      fi
    # key=value: match the WHOLE line in the file. Anchoring is the point --
    # an unanchored match would accept a trailing \r and let the CR/LF strip
    # be deleted without any test noticing.
    elif ! grep -qxF -- "$a" "$TMP/out.txt"; then
      echo "FAIL - $desc (no exact line \"$a\")"; FAILURES=$((FAILURES+1)); return
    fi
  done
  echo "ok   - $desc"
}

run_case "records the image the container reports, not the tag" \
  "grafana/grafana-enterprise:nightly" "grafana/grafana-enterprise@sha256:abc" \
  "grafana_image=grafana/grafana-enterprise:nightly" \
  "grafana_digest=grafana/grafana-enterprise@sha256:abc" \
  '!::warning'

# The recorded image must track the container even when it disagrees with
# GRAFANA_VERSION — that is the whole point of inspecting it.
run_case "a compose image change is followed, not overridden by the tag" \
  "some-mirror.internal/grafana:custom" "some-mirror.internal/grafana@sha256:def" \
  "grafana_image=some-mirror.internal/grafana:custom" \
  '!::warning'

run_case "unresolvable digest warns but does not fail" \
  "grafana/grafana-enterprise:nightly" "" \
  "grafana_digest=unresolved" \
  "::warning title=Grafana digest unresolved"

run_case "missing container records unknown, never a reconstructed guess" \
  "" "" \
  "grafana_image=unknown" \
  "grafana_digest=unresolved" \
  "::warning title=Grafana image unknown" \
  '!grafana/grafana-enterprise:13.2.1'

# Two warnings for one root cause is noise, not information.
run_case "missing container does not also warn about the digest" \
  "" "" \
  '!::warning title=Grafana digest unresolved'

# docker on a CRLF host would otherwise embed \r in the key=value line.
run_case "CRLF from docker is stripped (image and digest)" \
  $'grafana/grafana-enterprise:nightly\r' $'grafana/grafana-enterprise@sha256:abc\r' \
  "grafana_image=grafana/grafana-enterprise:nightly" \
  "grafana_digest=grafana/grafana-enterprise@sha256:abc"

make_docker_shim "grafana/grafana-enterprise:nightly" "grafana/grafana-enterprise@sha256:abc"
if PATH="$TMP/bin:$PATH" LUXON=true OUTPUT_FILE="$TMP/out.txt" "$RECORD" >/dev/null 2>&1; then
  echo "FAIL - missing GRAFANA_VERSION should be an error"
  FAILURES=$((FAILURES + 1))
else
  echo "ok   - missing GRAFANA_VERSION is an error"
fi

if [[ "$FAILURES" -gt 0 ]]; then
  echo "$FAILURES test(s) failed"
  exit 1
fi
echo "all record-grafana-image tests passed"
