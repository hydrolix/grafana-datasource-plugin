<!-- Canonical skill body, hand-maintained and owned by this repo. Harness stubs
     (.claude/skills/validate-plugin/SKILL.md, .codex/skills/validate-plugin/SKILL.md)
     carry the frontmatter and @-import this file. Edit here, not in the stubs.
     This skill fully supersedes the generic `@grafana/create-plugin` validate
     recipe; the original unforked recipe is preserved in git history as the
     first committed version of this file (commit ffc071b) — diff against that
     after a scaffold update, no snapshot file needed.
     WARNING: this file lives under .config/, which `create-plugin update`
     regenerates in place. Keep it committed and re-check it after any update. -->

# Validate Grafana Plugin (Hydrolix)

Invoked as `/validate-plugin`. **Run from the repo root.** The two rules
below are the difference between a real gate and a green report that verified
nothing.

## TL;DR

```sh
npm ci                                      # only if package-lock.json changed
npm run build                               # frontend → dist/module.js
mage -v buildAll && chmod 0755 dist/gpx_*   # ALL platforms, then fix modes

PLUGIN_ID=hydrolix-hydrolix-datasource
ZIP="${PLUGIN_ID}-$(date +%Y%m%d-%H%M%S).zip"
cp -r dist "$PLUGIN_ID" && zip -qr "$ZIP" "$PLUGIN_ID" && rm -rf "$PLUGIN_ID"

npx --cache .cache/npm -y @grafana/plugin-validator@latest \
    -jsonOutput -sourceCodeUri "file://$PWD" "./$ZIP"
```

No npx? The validator also ships as `grafana/plugin-validator-cli` on Docker
Hub (`docker run --pull=always -v "$PWD/$ZIP:/archive.zip:ro" … -jsonOutput
/archive.zip`) — but then the source must be mounted too and `-sourceCodeUri`
adjusted, or the `-sourceCodeUri` rule below bites.

Exit code `0` = no errors; warnings don't fail it. Pre-zip verification
(typecheck / lint / tests) is `build-plugin`'s job — see its verification
chain, including its warnings about trusting exit codes.

## `-sourceCodeUri` is NOT optional

The generic create-plugin recipe omits it. Without it, the source-side analyzers
(`osv-scanner`, `codediff`, `llmreview`) have no source to scan and — verified
empirically by running a clean zip both ways — **all three vanish from the JSON
silently**: no error, no `skipped` entry, exit code still `0`. The no-flag
report can be near-identical to a validated one (observed difference: only
govulncheck's *source-scan* entry, which needs source access). A
`suspected: skipped` notice appears only when a *different* analyzer errors —
never because this flag is missing. This is the single easiest way to declare
the plugin release-ready without having scanned a single dependency.

Two accepted forms — and since `src/plugin.json` has no `sourceCodeUri` field,
the flag must be passed on the command line every time:

```sh
-sourceCodeUri "file://$PWD"     # local working tree, while iterating
-sourceCodeUri https://github.com/hydrolix/grafana-datasource-plugin/tree/<tag>   # reproduce a release verdict
```

**Proof it ran lives in stderr, not the JSON.** Capture stderr and require
both lines, each with a non-zero package count:

```
INFO Scanned /…/go.mod file and found N packages
INFO Scanned /…/package-lock.json file and found N packages
```

No such lines → the scan never happened and the result means nothing,
regardless of exit code. Corollary for reading the JSON: **an absent analyzer
is ambiguous** — it passed clean OR it never ran; only the stderr lines
distinguish the two.

## `mage -v buildAll`, never a single platform

A partial `dist/` fails with `Missing linux/amd64 backend binary` (Grafana
requires linux/amd64), and any error in an early analyzer **blocks `codediff`
and `llmreview`** — they report `suspected: skipped` and never look at the
code. A short report is a red flag, not a win: fix every error and re-run
until zero.

Two silent traps around the binaries:

- `chmod 0755 dist/gpx_*` after building — the SDK targets don't guarantee the
  mode, and a non-executable binary is a validator error.
- `buildAll` emits **production** binaries. The dev container's `mage-watcher`
  writes noticeably larger **debug** binaries into the same `dist/` — those
  pass validation but must not ship. If binary sizes look inconsistent,
  rebuild before packaging.

## Known-benign findings — do not chase these

| Finding | Why it's expected |
| ------- | ----------------- |
| `manifest` — **unsigned plugin**, no `MANIFEST.txt` | Any local build. Only `npm run sign` (release path) produces it. |
| `sponsorshiplink` recommendation | Deliberate; we don't ship a sponsor link. |
| `govulncheck` — `golang.org/x/crypto` **GO-2026-5932** | The `openpgp` "unmaintained by design" advisory. **No fix exists** and it's not reachable from our code — confirm with `go list -deps ./pkg/... \| grep -c openpgp` (expect `0`). |

Most other `govulncheck` `GO-*` IDs track the **Go toolchain that built the
binary**, not a dependency. Clearing them means bumping three places: the `go`
directive in `go.mod`, `go_version:` in `.github/workflows/ci.yml` (appears
twice), and the toolchain that actually runs `mage`. Warnings only — never
gate-failing, but the top real finding once errors are gone.

## osv-scanner findings: verify, then fix

**Findings are often stale.** A validator report can describe an older tree
than the one you're holding (e.g. a zip built before the last dependency
commit). Confirm each finding against the OSV API before changing anything —
and mind its trap: requests **must** send `Content-Type: application/json`,
otherwise the API returns `{}` for everything, which reads identically to
"not vulnerable". Empirically, only HIGH/CRITICAL advisories error the gate;
moderates may not appear in the report at all (they still show in `npm audit`).

When a finding is real:

- osv-scanner reads `go.mod` / `package-lock.json` as lockfiles, so **dev-only
  and test-only deps fail the gate too** — they must be bumped even though they
  never ship.
- **Go transitive:** bumping the parent module often doesn't help (it may still
  pin the vulnerable version). `go get <module>@<fixed> && go mod tidy`, then
  check whether it ships: `go list -deps ./pkg/... | grep -c <module>`.
- **npm transitive:** use `overrides` in `package.json` — but **inspect the
  existing overrides block first**: a stale pin there can hold a package *at*
  a vulnerable version, or even below what `@grafana/*` packages themselves
  declare. Then `npm install` to refresh the lockfile.
- After any dependency change: re-run `build-plugin`'s verification chain
  before repackaging.

## Cleanup

The timestamped zip is **not** deleted automatically — report the filename and
let the user remove it. A failed run often leaves a half-built zip behind
(e.g. missing platforms); say explicitly which zip is the validated one so the
wrong artifact isn't shipped.

## Cross-links

- **Building & verification chain** → `build-plugin` skill.
- **Exercising a zip** in the stack / e2e → `e2e-run-zip` skill (that skill
  *runs* a package; this one *validates* it).
- **Signing** (produces the `MANIFEST.txt` this gate warns about) →
  `build-plugin`'s signing section.
- **Known flake:** ClickHouse testcontainer readiness timeout in
  `go test ./pkg/plugin/` — see
  `.claude/findings/2026-08-21-testcontainers-clickhouse-timeout.md`.
