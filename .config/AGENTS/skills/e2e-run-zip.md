<!-- Canonical skill body, hand-maintained and owned by this repo. Harness stubs
     (.claude/skills/e2e-run-zip/SKILL.md, .codex/skills/e2e-run-zip/SKILL.md)
     carry the frontmatter and @-import this file. Edit here, not in the stubs.
     WARNING: this file lives under .config/, which `create-plugin update`
     regenerates in place. Keep it committed and re-check it after any update. -->

# Grafana Plugin E2E against a packaged zip

Run the dev stack and Playwright suite against a **pre-built, signed plugin
package** (`hydrolix-hydrolix-datasource-<version>.zip`) — the exact artifact
that ships — instead of the working-tree `dist/`.

The package is unpacked into a **timestamped sandbox directory**,
`dist_<timestamp>/`, together with copies of the compose artifacts. Everything
happens inside that folder: the repo's own `dist/`, `docker-compose.yaml`, and
`.config/` are never touched, and the sandbox survives the test run so it can be
poked at afterwards.

**Non-negotiables:**

- The package is unzipped **as-is**. Never copy package contents into the repo's
  `dist/`, and never let anything rebuild the packaged binaries — the whole point
  is to exercise the bytes that will ship.
- **The sandbox is not deleted without the user's approval.** See "Cleanup".

**Division of labour with the sibling skills:**

| Concern | Skill |
| --- | --- |
| Bringing the stack up against the sandbox, health checks, version switching | **`stack-run`** — its "Running an alternate plugin directory" section is the general mechanism this skill parameterises |
| Locators, debugging failed specs, the `playwright` service internals, cross-version quirks | **`e2e-dev`** |
| This skill | sandbox layout, unzip-in-place, package verification, frozen-mode defaults |

## What a package zip contains

```
hydrolix-hydrolix-datasource/          ← single top-level folder
├── plugin.json                        ← id + version
├── MANIFEST.txt                       ← signed manifest (SHA256 per file)
├── module.js, module.js.map           ← frontend bundle
├── gpx_plugin_linux_arm64             ← backend, one binary per arch
├── gpx_plugin_linux_amd64
├── gpx_plugin_darwin_arm64 / _amd64
├── gpx_plugin_linux_arm
├── gpx_plugin_windows_amd64.exe
├── go_plugin_build_manifest
├── img/, README.md, CHANGELOG.md, LICENSE
```

Grafana selects the backend binary by the container's OS/arch. The dev grafana
container is `linux`, so on **Apple Silicon it needs `gpx_plugin_linux_arm64`**
(present in the package). The signed `MANIFEST.txt` makes Grafana report
`signature: valid` — testing a package also validates its signature, not just
its behaviour.

## Why a sandbox directory

- **`dist/` substitution** — explicitly avoided. It conflates "what I built
  locally" with "what shipped", destroys your working build, and is easy to
  forget to undo.
- **Unzipping next to the zip** (e.g. in `~/Downloads`) works for a one-shot run
  but leaves nothing to experiment with: no compose file, no provisioning, no
  record of which zip produced which result.
- **A `dist_<timestamp>/` sandbox** keeps the artifact, the compose stack that
  loaded it, and the provenance note in one disposable folder. Two or three of
  them can coexist, so you can A/B an RC against the previous release. It also
  happens to place the package at `<sandbox>/dist`, which is what the container's
  hardcoded `dist/` paths expect (see "The mage-watcher trap").
- **`file:///…zip` via `GF_INSTALL_PLUGINS`** — *does not work*. `/run.sh` turns
  `url;folder` into `grafana cli --pluginUrl <url> plugins install <folder>`, but
  grafana-cli fetches over Go's `net/http`, which rejects `file://`
  (`unsupported protocol scheme "file"`). You'd need to serve the zip over HTTP
  from inside the compose network — more moving parts than a bind mount.

`dist_*/` is gitignored, so a sandbox never shows up as untracked noise.

## Step 1 — build the sandbox

```sh
cd <repo-root>
ZIP=~/Downloads/hydrolix-hydrolix-datasource-<version>.zip   # the package to test

TS=$(date +%Y%m%d-%H%M%S)
SANDBOX="$PWD/dist_$TS"                    # absolute — needed by the mount later
mkdir -p "$SANDBOX"

# 1a. Unzip into a staging dir, then promote the plugin folder to $SANDBOX/dist.
#     The zip's top-level folder becomes `dist` so the container's hardcoded
#     dist/ paths line up and no volume override is needed if you later run
#     compose from inside the sandbox.
unzip -q "$ZIP" -d "$SANDBOX/.unzip"
PKG=$(dirname "$(find "$SANDBOX/.unzip" -maxdepth 2 -name plugin.json | head -1)")
test -n "$PKG" || { echo "!! no plugin.json in $ZIP"; return 2>/dev/null || exit 1; }
mv "$PKG" "$SANDBOX/dist"
rm -rf "$SANDBOX/.unzip"

# 1b. Copy the compose artifacts so the sandbox is a self-contained playground.
#     All small (~150K total). node_modules and the repo's dist/ are NOT copied.
cp docker-compose.yaml "$SANDBOX/"
cp -R .config provisioning testdata "$SANDBOX/"
cp -R tests playwright.config.ts package.json package-lock.json "$SANDBOX/"  # for reference/editing

# 1c. Frozen-mode override, kept inside the sandbox (no volume entry needed —
#     the plugin dir is passed separately in step 3).
cat > "$SANDBOX/docker-compose.package.yaml" <<YAML
services:
  grafana:
    environment:
      DEV: "false"        # prod entrypoint -> no mage-watcher rebuild
    volumes:
      - ${SANDBOX}/dist:/var/lib/grafana/plugins/hydrolix-hydrolix-datasource:ro
YAML

# 1d. Provenance note — which zip, when, what version.
cat > "$SANDBOX/README.md" <<NOTE
# Package test sandbox $TS

- Source zip: $ZIP
- Unpacked:   ./dist  (plugin folder, exactly as shipped — do not edit)
- Version:    $(python3 -c 'import json;print(json.load(open("'"$SANDBOX"'/dist/plugin.json"))["info"]["version"])' 2>/dev/null)
- Created:    $TS

Compose artifacts here are copies; the repo's originals were not modified.
Bring the stack up from the repo root with:

    export COMPOSE_FILE=docker-compose.yaml:$SANDBOX/docker-compose.package.yaml
    docker compose up -d grafana clickhouse-server

Safe to delete this whole directory when done.
NOTE

echo "sandbox ready: $SANDBOX"
ls "$SANDBOX"
```

Everything in `$SANDBOX` except `dist/` is yours to edit — that's the point.
`dist/` is the artifact under test; editing it invalidates the signature.

## Step 2 — verify the package before running it

```sh
# Backend binary for the container's arch must be present.
ls "$SANDBOX/dist" | grep -E 'gpx_plugin_linux_(arm64|amd64)' || echo "!! no linux backend"

# Signed manifest present, and the declared version.
test -f "$SANDBOX/dist/MANIFEST.txt" && echo "MANIFEST.txt present (expect signature: valid)"
python3 -c 'import json;d=json.load(open("'"$SANDBOX"'/dist/plugin.json"));print("id:",d["id"],"version:",d["info"]["version"])'
```

## Step 3 — bring the stack up (via `stack-run`)

This is `stack-run`'s **frozen** alternate-plugin-directory flow, with
`PLUGIN_DIR = $SANDBOX/dist`. Read that skill for the mechanism, the live-vs-frozen
table, and the traps; the two lines specific to this skill are:

```sh
cd <repo-root>
export COMPOSE_FILE=docker-compose.yaml:$SANDBOX/docker-compose.package.yaml
docker compose up -d grafana clickhouse-server
until curl -sf http://localhost:3000/api/health >/dev/null; do sleep 2; done
```

Points that carry over from `stack-run` and matter here:

- **Override last.** `COMPOSE_FILE` is colon-separated and order-sensitive — the
  sandbox override must come second so its `DEV` and volume win the merge.
- **Absolute path in the override.** `$SANDBOX` is absolute by construction
  (step 1). A relative path would resolve against the repo root, not the
  override's location.
- **Never use the `DC="docker compose -f … -f …"` + `$DC up` idiom in zsh** — zsh
  does not word-split unquoted expansions, so `$DC` fails as a single command
  name. Exporting `COMPOSE_FILE` avoids the problem entirely and also covers the
  `run --rm playwright` invocation in step 5.
- **Run from the repo root, not the sandbox.** The compose project name is
  derived from the working directory; running from `dist_<ts>/` creates a new
  project, which means new image tags and a full rebuild of both the grafana and
  playwright images. See "Fully isolated variant" if you want that anyway.
- Unsigned loading is not a concern — the Dockerfile bakes
  `GF_DEFAULT_APP_MODE=development` and the compose sets
  `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS` — but a real package is signed and
  validates as `valid` regardless.

## Step 4 — verify the package (not something else) is loaded

```sh
# Version + signature come from the mounted package.
curl -s http://localhost:3000/api/plugins/hydrolix-hydrolix-datasource/settings \
  | python3 -c 'import sys,json;d=json.load(sys.stdin);i=d.get("info",{});print("version:",i.get("version"));print("signature:",d.get("signature"),d.get("signatureType"),d.get("signatureOrg"))'
# Expect: version: <the package version>   signature: valid commercial hydrolix.io

# Prod entrypoint = no watcher rebuilding the binary.
docker exec grafana-dashboards sh -c \
  'ps -o args | grep -iE "mage|supervisor" | grep -v grep || echo "frozen OK (no mage/supervisor)"'

# Exactly one mount on the plugin target, sourced from the sandbox.
docker compose config | grep -B2 -A2 'plugins/hydrolix-hydrolix-datasource'
```

If `version:` shows something other than the package version, or you see a
`mage` process, the package is **not** what's running — stop and fix before
trusting any test result.

## Step 5 — run the e2e suite

`COMPOSE_FILE` from step 3 is still in effect, so the `playwright` service
resolves the same merged config — required, because it joins the grafana
container's network namespace via `network_mode: "service:grafana"`.

```sh
docker compose run --rm playwright                                # full suite (~2–3 min)
docker compose run --rm playwright tests/configEditor.spec.ts     # one spec
docker compose run --rm playwright --grep "interpolated query"    # ad-hoc grep
```

The specs come from the **repo's** `tests/` (the runner bind-mounts the repo at
`/work`), while Grafana serves the **package**. That's the intended split: current
tests against the shipping artifact. The `tests/` copy in the sandbox is a
reference snapshot — edit it only if you want a frozen record of what was run.

Everything else about the runner — result artifacts under `test-results/`,
`junit_report.xml` at repo root, image-rebuild conditions, timeouts — is
identical to the `e2e-dev` skill.

## Cleanup — optional, and only with the user's approval

The sandbox is deliberately left in place after the run so the user can keep
poking at it. **Do not delete it on your own initiative.** Tearing the stack down
is safe and unprompted; removing the directory is not.

```sh
# Always safe: stop the stack, restore the shell.
docker compose down
unset COMPOSE_FILE
```

That alone leaves the repo exactly as it was — `docker-compose.yaml`, `.config/`,
and `dist/` were never modified, so there is nothing to `git restore`.

Then **ask** before running:

```sh
rm -rf "$SANDBOX"        # ONLY after the user confirms
```

Reasons to keep it: re-running the suite without re-unzipping, diffing two RCs,
inspecting `MANIFEST.txt` / `go_plugin_build_manifest`, or reproducing a failure
later.

**A sandbox is big — ~215 MB measured for 0.10.2** — because the package ships
six architecture binaries (`dist/` is essentially all of it; the copied compose
artifacts total under 1 MB). They are gitignored, but don't accumulate them
silently:

```sh
ls -d dist_*/ 2>/dev/null && du -sh dist_*/ 2>/dev/null   # what's lying around, and how big
```

## Playing around in the sandbox

Things that work well once the stack is up:

- **Edit `$SANDBOX/docker-compose.package.yaml`** to add env vars, flip
  `DEV` back to `"true"`, or drop `:ro` — then
  `docker compose up -d --force-recreate --no-deps grafana`. Compose only
  re-reads mounts and env on container re-creation, so `restart` is not enough.
- **Edit `$SANDBOX/provisioning/`** and `docker compose restart grafana` — but
  note the *repo's* `provisioning/` is what the base file mounts. To use the
  sandbox copy, add it to the override:
  `- ${SANDBOX}/provisioning:/etc/grafana/provisioning`.
- **Compare two packages** — build a second sandbox from the other zip, then
  point `COMPOSE_FILE` at its override and `--force-recreate` grafana. Same
  dashboards, different artifact.
- **Don't edit `$SANDBOX/dist`.** Any change there breaks the signature and
  Grafana will report `signature: modified`, which is a useful test in itself but
  not what you want by accident.

### Fully isolated variant

To run the stack entirely out of the sandbox — no repo paths in the mounts at
all — run compose *from* `$SANDBOX`. The copied `.config/docker-compose-base.yaml`
mounts `../dist`, which resolves to `$SANDBOX/dist`, so **no volume override is
needed**; only `DEV: "false"` is. Two costs:

```sh
cd "$SANDBOX"
export COMPOSE_PROJECT_NAME=grafana-datasource-plugin   # reuse existing image tags
export COMPOSE_FILE=docker-compose.yaml:docker-compose.package.yaml
docker compose up -d grafana clickhouse-server
```

1. **Set `COMPOSE_PROJECT_NAME`**, or the project defaults to `dist_<ts>` and
   compose rebuilds `…-grafana` and `…-playwright` from scratch (~5 min for the
   playwright image).
2. **Container names and ports still collide** with a running dev stack —
   `container_name:` is explicit (`grafana-dashboards`, `clickhouse-server`,
   `keycloak`, `playwright-e2e`) and ports 3000/2345/8123/8080 are fixed. Bring
   the other stack down first, whichever direction you're switching.

The e2e runner in this mode uses the sandbox's `tests/` and needs its
`node_modules`; the repo-root invocation in step 5 avoids that entirely, which is
why it's the default.

## The mage-watcher trap (why `DEV=false` is load-bearing)

In the default dev build the grafana container runs supervisord, whose
`[program:mage-watcher]` executes `mage -v watch` in `/root/hydrolix-hydrolix-datasource`
(the repo root, bind-mounted). That watcher compiles `pkg/` into
`dist/gpx_plugin_linux_<arch>` — the *same* file the default `../dist` mount
exposes to Grafana. So in dev mode the running backend is always a fresh local
build, regardless of what a package contains. `DEV=false` takes the `/run.sh`
branch of `entrypoint.sh`, which never starts supervisord, so the packaged
binary is used untouched. (The `..:/root/hydrolix-hydrolix-datasource` mount is
still present but inert in prod mode.)

Frontend (`module.js`) has no equivalent trap — nothing in any container
rebuilds it — but with the package mount you're serving the package's
`module.js` anyway, not `dist/`.

Related, and the reason the sandbox names its folder `dist`:
`[program:grafana]` waits on `/root/hydrolix-hydrolix-datasource/dist/gpx_plugin*`
before starting Grafana. In frozen mode supervisord never runs so the guard is
irrelevant; in the fully-isolated variant the container's repo root *is* the
sandbox, so `$SANDBOX/dist/gpx_plugin_linux_*` from the package satisfies it
directly.

## Testing a specific Grafana version

The running Grafana version is whatever the local `…-grafana:latest` image was
last built with (the `grafana_version` build arg in `docker-compose.yaml`,
default line = 13.0.1; commented toggles for 12.3.1 / 12.0.2 / 11.5.4 / 10.4.16).
The `DEV=false` runtime override works on **any** of these images — you do
**not** need to rebuild with `development=false`. To switch versions, follow the
"Switching Grafana versions" section of the `stack-run` skill
(`docker compose build grafana` after changing the arg), then apply this skill's
override on top. To sweep several versions, rebuild + re-run per version, keeping
one sandbox and re-creating only the grafana container.

## Gotchas

- **Wrong-arch backend "plugin unavailable".** If Grafana logs the plugin as
  registered but queries fail with the backend not starting, confirm the package
  actually contains `gpx_plugin_linux_<container-arch>` (step 2). A frontend-only
  or single-arch package will register but have no working backend on your arch.
- **Read-only mount is intentional.** Grafana never needs to write into the
  plugin dir; `:ro` guarantees the test run can't mutate the artifact under test.
- **`$SANDBOX` must be absolute.** Step 1 builds it from `$PWD`. If you set it by
  hand as a relative path, the override's mount silently resolves against the
  repo root.
- **`$SANDBOX` and `$TS` are shell variables, not persisted.** A new terminal
  loses them; recover with `ls -d dist_*/` and re-export, or read
  `$SANDBOX/README.md`, which records the zip and version.
- **Don't `git checkout docker-compose.yaml` expecting to undo this.** This skill
  never edits the base compose or `.config/`; there's nothing to revert there.
  (If a previous session hand-edited them, that's a separate cleanup.)
- **Step 5 fails with `Executable doesn't exist at …/chromium_headless_shell-*`**
  — that's a **stale playwright image**, not a package problem: the image's baked
  browser no longer matches the installed `@playwright/test`. Fix with
  `docker compose build playwright` (see `e2e-dev`'s rebuild conditions). The
  grafana side is unaffected, so steps 3–4 still prove the package loads.
- **Signature says `invalid`/`modified`/`unsigned`?** Either files were changed
  after signing (re-run step 1 for a clean extraction) or the mount points at the
  wrong folder — it must be the dir *containing* `plugin.json` and
  `MANIFEST.txt`, i.e. `$SANDBOX/dist`, not `$SANDBOX`.
