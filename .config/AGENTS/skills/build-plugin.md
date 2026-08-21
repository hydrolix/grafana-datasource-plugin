<!-- Canonical skill body, hand-maintained and owned by this repo. Harness stubs
     (.claude/skills/build-plugin/SKILL.md, .codex/skills/build-plugin/SKILL.md)
     carry the frontmatter and @-import this file. Edit here, not in the stubs.
     WARNING: this file lives under .config/, which `create-plugin update`
     regenerates in place. Keep it committed and re-check it after any update. -->

# Grafana Plugin Build

Invoked as `/build-plugin`. **Run from the repo root** — every path below is
root-relative and webpack resolves its config relative to CWD.

## Default procedure

Preconditions: Node ≥22 (`package.json` engines); run `npm ci` first if
`package-lock.json` changed since your last install.

```sh
npm run build                      # 1. frontend → dist/module.js (+ map, plugin.json)
                                   # 2. backend: normally NOTHING — see below
npm run typecheck && npm run lint && npm run test:ci   # 3. verify
```

**Step 2 is deliberately empty in the normal case.** The dev container's
`mage-watcher` owns the Go binary; building it on the host is only for release
artifacts, CI replication, or a cross-compile check. When you do need it:

```sh
mage -v build:linux && chmod 0755 dist/gpx_*
```

### Stop conditions — do not push past these

| Condition | Action |
| --------- | ------ |
| `npm run build` fails | **Stop.** Report the webpack error verbatim. Do not run the verification chain against a stale `dist/`. |
| `mage` not on PATH | **Stop.** Tell the user: mage is required to build the backend — install from https://magefile.org or `go install github.com/magefile/mage@latest` (`brew install mage` works too). |
| Backend build fails | **Stop.** Report it. Never `chmod`, zip, or sign a partially built `dist/`. |
| Verification chain not run | Do not claim the build is done. See "Verification chain" below. |

Authoritative build/packaging references: `.config/AGENTS/instructions.md` and
https://grafana.com/developers/plugin-tools/publish-a-plugin/package-a-plugin.md

## The artifact split — the thing that bites everyone

Grafana loads the plugin from `dist/`, mounted into the dev container at `/var/lib/grafana/plugins/hydrolix-hydrolix-datasource` (see `.config/docker-compose-base.yaml`). What's in `dist/` is what's running. **There are two artifact families with two different rebuild paths**:

| Artifact                                       | Built by                            | Where it runs               | Auto-rebuild?                                                  |
| ---------------------------------------------- | ----------------------------------- | --------------------------- | -------------------------------------------------------------- |
| `dist/module.js`, `module.js.map`, `plugin.json` | `webpack` via `npm run build`/`dev` | **Host**                    | **No** — you must run `npm run build` (or have `npm run dev` watching) |
| `dist/gpx_plugin_<os>_<arch>`                  | `mage build:debug`                  | **Inside the dev grafana container** | **Yes** — supervisord's `[program:mage-watcher]` runs `mage -v watch` and rebuilds on every change |

**Implication.** Edits under `src/` (TypeScript/React) are *not* live until you build on the host. Edits under `pkg/` (Go) are live the next time the container picks them up — the watcher rebuilds and `[program:build-watcher]` restarts delve via `inotifywait`.

Quick sanity check when behaviour doesn't match the source you're reading:

```sh
stat -f '%Sm %N' dist/module.js src/components/QueryEditor.tsx
# module.js older than the .tsx you edited → dev container is serving stale frontend.
```

This is the #1 way to get a misleading "fix verified" on a frontend change. See also the e2e skill's "Plugin artifacts the e2e suite consumes" section.

## Environment detection (for portable scripts)

This repo is npm-only and always has a backend, so you can hardcode both. Use
these when writing a script that must also work in a vanilla create-plugin repo,
or when verifying an assumption rather than trusting it:

```sh
# Package manager: packageManager field wins, then lockfile. Here: always npm.
PKG_MANAGER=$(
  if grep -q '"packageManager"' package.json 2>/dev/null; then
    grep '"packageManager"' package.json | sed -E 's/.*"packageManager" *: *"([^@]+).*/\1/'
  elif [ -f "pnpm-lock.yaml" ]; then echo "pnpm"
  elif [ -f "yarn.lock" ];      then echo "yarn"
  else echo "npm"
  fi
)

# Backend present? Non-zero means build the Go binary too. Here: always 1.
HAS_BACKEND=$(grep -c '"backend" *: *true' src/plugin.json || true)
```

Here the first branch always wins: `package.json` pins
`"packageManager": "npm@11.12.1"`, and `package-lock.json` is the only lockfile
committed. If `pnpm-lock.yaml` or `yarn.lock` ever appears in a diff, that is the
bug, not a new supported path — and if the pin and the lockfile ever disagree,
the pin is authoritative (Corepack reads it).

## Frontend (webpack)

```sh
npm run build         # one-shot production bundle → dist/module.js (+ map, plugin.json copy)
npm run dev           # webpack -w, development mode, rebuilds on save
```

- Both are defined in `package.json` and resolve to `webpack -c ./.config/webpack/webpack.config.ts --env {production|development}` — note the config lives under `.config/`, so CWD must be the repo root.
- `npm run dev` is the right default while iterating: leave it running in a side terminal; saved `.tsx`/`.ts` edits hit `dist/` in ~1–2 s and Grafana picks them up on the next page reload (no container restart).
- Node ≥22 is required (`package.json` engines). Check with `node -v` if a build blows up unexpectedly.
- Install/refresh deps with `npm ci` (not `npm install`) when `package-lock.json` changes — keeps the host's lockfile in lockstep with the playwright image.

## Backend (Go via mage)

You almost never need to build Go on the host. The dev container's `mage -v watch` does it for you on every save under `pkg/`. Host builds are for cross-compile sanity checks, release artifacts, and CI replication:

```sh
# One-shot host build (e.g. for cross-compile sanity checks, signed releases, CI replication)
mage build:debug                # → dist/gpx_plugin_<host_os>_<host_arch>
mage -v build:linux             # cross-compile for the dev container's target
mage -v                         # Default target = build.BuildAll — every platform (release path)
mage -l                         # list all mage targets the SDK exposes
```

**Always fix permissions after a host build.** Nothing in the SDK targets
guarantees the mode, and a non-executable binary fails silently at plugin load
and is flagged by `plugin-validator`:

```sh
chmod 0755 dist/gpx_*
```

- Requires `mage` on PATH (`brew install mage` or `go install github.com/magefile/mage@latest`).
- The Magefile wires Grafana's plugin-sdk `build` package as `mage:import`. `mage -v watch`, `mage build:debug`, `mage build:linuxARM64`, etc. are all SDK-provided.
- Build info (timestamp, plugin id, version) is injected via `SetBeforeBuildCallback` in `Magefile.go:25`. If the version string in About is stale, this is the path.

## Verification chain — run before claiming a build is "done"

```sh
npm run typecheck     # tsc --noEmit
npm run lint          # eslint --cache .
npm run lint:fix      # eslint --fix + prettier --write
npm run test:ci       # jest --ci with coverage, junit reporter, 4 workers
```

- `npm run test` (no `:ci`) starts Jest in **watch mode on changed files only** — fine for local TDD, but use `test:ci` for "did I break anything" sweeps.
- Lint warnings still produce exit 0; `--max-warnings 0` is *not* set, so check the output, don't just trust the exit code.
- `test:ci` passes `--passWithNoTests`, so an empty/mis-globbed run is a *pass*. Check that the test count is non-zero before trusting a green result.

## When `dist/` is suspicious

| Symptom                                                       | Likely cause                                                                                         | Fix                                                                                  |
| ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| UI behaviour matches an *old* commit, source on disk is newer | Stale `dist/module.js` (forgot `npm run build` after editing `src/`)                                 | `npm run build`, then reload the Grafana page                                        |
| Plugin won't load, Grafana logs "plugin not found" loop       | `dist/` is empty (fresh clone, never built) — supervisord blocks on `gpx_plugin*` and never starts Grafana | `npm run build` + bring the container up; the container rebuilds the Go binary itself |
| `dist/gpx_plugin_*` missing for your arch                     | First container start hasn't finished, or `mage-watcher` died                                        | `docker exec grafana-dashboards supervisorctl status mage-watcher`; restart it if FATAL |
| Go change isn't reflected after save                          | `mage-watcher` not running, or delve hasn't restarted                                                | `docker compose logs grafana \| grep -E 'mage\|delve'`; restart the grafana service if stuck |

## Cleaning

```sh
rm -rf dist/ node_modules/      # nuclear
npm ci                          # restore deps from lockfile
npm run build                   # rebuild frontend
# Go side rebuilds itself once the container is up — no action.
```

## Signing (release path only)

```sh
npm run build       # production frontend bundle
mage -v             # BuildAll — every platform's gpx_plugin_* binary
chmod 0755 dist/gpx_*
npm run sign        # invokes @grafana/sign-plugin against the current dist/
```

Run this only against a production-built `dist/`. Local dev plugins are allowed unsigned via `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS` in the dev compose (`.config/docker-compose-base.yaml`).

## Cross-links

- **Running the built artifacts** → `stack-run` skill.
- **Validating a packaged build before release** → `validate-plugin` skill
  (zips `dist/` and runs `@grafana/plugin-validator`). Run it after the signing
  sequence above.
- **Running e2e against the built artifacts** → `e2e-dev` skill. Its "Plugin artifacts the e2e suite consumes" section is the e2e-flavoured version of this skill's artifact-split table.
