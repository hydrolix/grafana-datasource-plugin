<!-- Canonical skill body, hand-maintained and owned by this repo. Harness stubs
     (.claude/skills/stack-run/SKILL.md, .codex/skills/stack-run/SKILL.md)
     carry the frontmatter and @-import this file. Edit here, not in the stubs.
     WARNING: this file lives under .config/, which `create-plugin update`
     regenerates in place. Keep it committed and re-check it after any update. -->

# Grafana Plugin Run (dev stack)

## What gets started

`docker-compose.yaml` defines four services:

| Service             | Container name              | Ports          | Role                                                                                      |
| ------------------- | --------------------------- | -------------- | ----------------------------------------------------------------------------------------- |
| `grafana`           | `grafana-dashboards`        | `3000`, `2345` | Dev Grafana under supervisord. Plugin mounted from host `dist/` by default — overridable, see "Running an alternate plugin directory". Delve on 2345. |
| `clickhouse-server` | `clickhouse-server`         | `8123`         | Backing DB. Initialises `e2e.*` fixtures from `./testdata/containers/initdb.sql`.         |
| `keycloak`          | `keycloak`                  | `8080`         | OAuth IdP. Realm imported from `provisioning/keycloak/grafana-realm.json`.                |
| `playwright`        | `playwright-e2e`            | (network namespace shares grafana) | On-demand e2e runner. **Will also try to start on bare `docker compose up`** — accepted tradeoff (see e2e skill). |

## Bring the stack up

```sh
docker compose up --build              # or: npm run server (alias)
docker compose up -d --build           # detached, return shell immediately
docker compose up grafana clickhouse-server keycloak    # skip the playwright runner
```

- `--build` rebuilds the grafana image when the Dockerfile or build args change (cheap, layer-cached). Drop it for the second invocation onward.
- `npm run server` is just `docker compose up --build`.

**Prerequisite — `dist/` must contain the frontend bundle before grafana starts.** Supervisord's grafana program waits forever for `dist/gpx_plugin*` (which the container itself builds), but if `dist/module.js` is missing Grafana logs a plugin-load error and you'll see a broken plugin in the UI. On a fresh clone:

```sh
npm ci
npm run build        # creates dist/module.js, plugin.json, etc.
docker compose up --build
```

See the `build-plugin` skill for the full artifact story. To serve artifacts
from somewhere other than `dist/`, see "Running an alternate plugin
directory" below — and note that the supervisord guard above still checks
repo-root `dist/` even when a different folder is mounted.

## Running an alternate plugin directory

By default Grafana serves the plugin from the working-tree `dist/`. That mount is
declared in the **scaffold-owned** base file, which must not be edited (it is
regenerated in place by `create-plugin update`):

```yaml
# .config/docker-compose-base.yaml — DO NOT EDIT
volumes:
  - ../dist:/var/lib/grafana/plugins/hydrolix-hydrolix-datasource
```

(`../dist` is relative to `.config/`, i.e. repo-root `dist/`, because `extends:`
resolves a relative path against the *extended* file's own directory.)

To serve a **different** folder — a second build, another branch's `dist/`, an
unzipped release artifact, a CI download — override the mount from an *ephemeral
override compose file*. Compose merges a service's `volumes` list **deduplicated
by container target**, so an entry with the same target **replaces** the base one
rather than colliding (Docker rejects duplicate mount targets). The base
`docker-compose.yaml` and `.config/` stay untouched, so there is nothing to
`git restore` afterwards.

### Recipe

```sh
cd <repo-root>

# The folder must be the one CONTAINING plugin.json — not its parent.
PLUGIN_DIR=$(cd /path/to/plugin-folder && pwd)          # absolutise
test -f "$PLUGIN_DIR/plugin.json" || echo "!! not a plugin dir"

cat > /tmp/docker-compose.plugin-dir.yaml <<YAML
services:
  grafana:
    volumes:
      - ${PLUGIN_DIR}:/var/lib/grafana/plugins/hydrolix-hydrolix-datasource
YAML

# Point every later compose command at BOTH files. Override goes LAST so its
# volume wins the merge. COMPOSE_FILE is colon-separated.
export COMPOSE_FILE=docker-compose.yaml:/tmp/docker-compose.plugin-dir.yaml

docker compose up -d --build grafana clickhouse-server keycloak
until curl -sf http://localhost:3000/api/health >/dev/null; do sleep 2; done
```

- **Use an absolute host path.** A relative path in the override resolves
  against the *project directory* — the repo root, i.e. the directory of the
  first compose file — **not** the override's own directory in `/tmp`. So
  `./my-plugin` silently becomes `<repo-root>/my-plugin`. Absolutising with
  `$(cd … && pwd)` removes the trap.
- **Every later compose command must see both files** — `logs`, `ps`, `restart`,
  `run --rm playwright`, `down`. `export COMPOSE_FILE=…` is the least
  forgettable way; exporting it once makes plain `docker compose …` correct for
  the rest of the shell session. This matters most for the e2e runner: the
  `playwright` service uses `network_mode: "service:grafana"`, so with only the
  base file it resolves a different grafana container and fails to join it.
- **Don't use the `DC="docker compose -f … -f …"` + `$DC up` idiom in zsh.**
  zsh does not word-split unquoted parameter expansions, so `$DC` is treated as
  one command name and the whole thing fails. If you prefer an alias over the
  env var, use a function — portable across bash and zsh:
  `dc() { docker compose -f docker-compose.yaml -f /tmp/docker-compose.plugin-dir.yaml "$@"; }`
- **Changing `$PLUGIN_DIR` needs a re-create, not a restart.** Compose only
  re-reads mounts when the container is created:
  `docker compose up -d --force-recreate --no-deps grafana`.
- Unsigned folders load fine — the compose already sets
  `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS: hydrolix-hydrolix-datasource`.

### Live vs frozen

|                        | **Live** (`DEV=true`, the default)                        | **Frozen** (`DEV=false`)                                  |
| ---------------------- | --------------------------------------------------------- | --------------------------------------------------------- |
| Override contains      | volume only                                               | volume (add `:ro`) **+** `environment: DEV: "false"`      |
| Container runs         | supervisord: `mage-watcher`, `delve`, `build-watcher`      | plain `/run.sh` — no supervisord, no watchers             |
| Go hot-reload          | recompiles into **`dist/`**, *not* `$PLUGIN_DIR`           | none — the mounted binary is what runs                    |
| Delve on `:2345`       | yes                                                       | no                                                        |
| Use for                | a second dist-like folder you rebuild into by hand         | release/RC artifacts, anything that must not be mutated    |

```sh
# Frozen variant of the override:
cat > /tmp/docker-compose.plugin-dir.yaml <<YAML
services:
  grafana:
    environment:
      DEV: "false"
    volumes:
      - ${PLUGIN_DIR}:/var/lib/grafana/plugins/hydrolix-hydrolix-datasource:ro
YAML
```

`DEV` is baked from the `development` build arg but read **at runtime** by
`.config/entrypoint.sh` (`[ "$DEV" = "false" ] && exec /run.sh`), so overriding
the env var is enough — **no image rebuild needed**, on any `grafana_version`.

### Two live-mode traps (both from `dist/` hardcoded in supervisord)

1. **Grafana still waits for `dist/gpx_plugin*`.** `[program:grafana]` is
   `bash -c 'while [ ! -f /root/hydrolix-hydrolix-datasource/dist/gpx_plugin*; do sleep 1; done; /run.sh'`
   — repo-root `dist/`, regardless of what is mounted. If your working-tree
   `dist/` has no Go binary, Grafana never starts even though `$PLUGIN_DIR` has
   one. Normally `mage-watcher` satisfies this by itself; if you have emptied
   `dist/`, either let the watcher run or switch to frozen mode, which skips
   supervisord entirely.
2. **`mage -v watch` writes to `dist/`, not `$PLUGIN_DIR`.** Go edits recompile
   into a folder Grafana is no longer serving, so backend hot-reload silently
   stops working and your change looks like it had no effect. Frozen mode makes
   this explicit instead of silent.

`[program:build-watcher]` is unaffected — it `inotifywait`s the *container* path,
which is the same either way, so delve still re-attaches when `$PLUGIN_DIR`
changes on the host.

### Verify the right folder is being served

```sh
# 1. Merged config resolves to your dir (source of truth for the mount).
#    Expect exactly ONE mount on this target, sourced from $PLUGIN_DIR.
docker compose config | grep -B2 -A2 'plugins/hydrolix-hydrolix-datasource'

# 2. Grafana reports the version/signature from that folder.
#    Plain curl works — anonymous Admin is enabled on this stack.
curl -s http://localhost:3000/api/plugins/hydrolix-hydrolix-datasource/settings \
  | python3 -c 'import sys,json;d=json.load(sys.stdin);i=d.get("info",{});print("version:",i.get("version"),"| signature:",d.get("signature"))'

# 3. Frozen mode only: confirm nothing can rebuild the binary.
docker exec grafana-dashboards sh -c \
  'ps -o args | grep -iE "mage|supervisor" | grep -v grep || echo "frozen OK (no mage/supervisor)"'
```

If the version isn't the one in `$PLUGIN_DIR/plugin.json`, or you see a `mage`
process in frozen mode, **the folder you think you're testing is not what's
running** — fix that before trusting any result.

### Cleanup

```sh
docker compose down                  # still sees both files via COMPOSE_FILE
unset COMPOSE_FILE
rm -f /tmp/docker-compose.plugin-dir.yaml
# docker-compose.yaml, .config/, and dist/ were never modified.
```

### Common cases

- **A release/RC zip** → don't hand-roll it. `e2e-run-zip` wraps this exact
  mechanism with unzip-in-place, frozen-mode defaults, and signature/version
  verification.
- **Two builds side by side** → build into `dist/` and, say, `/tmp/dist-baseline`,
  then flip `$PLUGIN_DIR` and `docker compose up -d --force-recreate --no-deps grafana` to
  A/B them against the same dashboards.
- **Another branch's build** →
  `git worktree add ../wt-x <branch> && (cd ../wt-x && npm ci && npm run build)`,
  then point `$PLUGIN_DIR` at the absolutised `../wt-x/dist`. Note the base also
  mounts *this* checkout at `/root/hydrolix-hydrolix-datasource`, so in live mode
  `mage-watcher` compiles **this** branch's Go source — prefer frozen mode so the
  backend and frontend come from the same commit.

## Where to point your browser

- **Grafana**: <http://localhost:3000>. Anonymous Admin role is enabled (`GF_AUTH_ANONYMOUS_ENABLED=true`, `GF_AUTH_ANONYMOUS_ORG_ROLE=Admin`), so you don't need to log in to develop. Basic auth is **disabled** (`GF_AUTH_BASIC_ENABLED=false`).
- **Grafana via OAuth / Keycloak**: same URL, click "Sign in with Keycloak". Keycloak admin console: <http://localhost:8080> (`admin`/`admin`). The grafana realm is auto-imported on Keycloak start.
- **ClickHouse**: HTTP on `:8123`. Credentials `testuser` / `testpass` (set in `docker-compose.yaml`). Connect a SQL client to confirm fixtures: `docker exec clickhouse-server clickhouse-client -u testuser --password testpass --query "SHOW DATABASES"`.

## Hot reload — what auto-refreshes and what doesn't

| Edit                    | Picked up by                                                                                                       | Action needed                                  |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------- |
| `pkg/**` (Go)           | `[program:mage-watcher]` rebuilds `dist/gpx_plugin_*`; `[program:build-watcher]` sees the change via `inotifywait` and restarts `[program:delve]` | None — reload the Grafana page to see effects. |
| `src/**` (TS/React)     | Webpack on the host (only if `npm run dev` is running). Grafana picks up new `dist/module.js` on page reload.       | `npm run build` if not in watch mode.          |
| `docker-compose.yaml` / `.config/Dockerfile` / build args | Compose only re-evaluates on `up --build` or `build`.                                          | `docker compose up --build`.                   |
| `provisioning/**`       | Grafana reads on startup.                                                                                          | `docker compose restart grafana`.              |
| `testdata/containers/initdb.sql` | ClickHouse runs initdb only on a fresh data volume.                                                       | `docker compose down clickhouse-server` (drops anon volume), then `up`. |

**With an alternate plugin directory mounted, the first two rows change:** the
mage-watcher writes to `dist/`, and webpack writes wherever you point it — not
necessarily the mounted folder. See "Two live-mode traps" above.

## Switching Grafana versions

`docker-compose.yaml` toggles `grafana_version` via commented lines under the `grafana` service's `build.args:`:

```yaml
build:
  args:
    grafana_version: 13.0.1
#    grafana_version: 12.3.1
#    grafana_version: 12.0.2
#    grafana_version: 11.5.4
#    grafana_version: 10.4.16
```

Swap the active line, then:

```sh
docker compose stop grafana
docker compose build grafana
docker compose up -d --no-deps grafana    # --no-deps skips keycloak (already up)
until curl -sf http://localhost:3000/api/health >/dev/null; do sleep 2; done
```

Verified supported: 10.4.16, 11.5.4, 12.0.2, 12.3.1, 13.0.1. CI matrix uses 10.4.18, 11.6.1, 12.0.2, 13.0.1 (latest patch within each minor).

## Inspecting / debugging the stack

```sh
docker compose ps                                    # which services are up
docker compose logs -f grafana                       # tail grafana + supervisord output
docker compose logs grafana | grep -E 'mage|delve'   # spot-check the watchers
docker exec grafana-dashboards supervisorctl status  # per-program status inside the container
docker exec grafana-dashboards supervisorctl restart mage-watcher   # if it's FATAL
docker exec grafana-dashboards supervisorctl restart delve          # force delve re-attach
```

- Delve is on host port `2345`. Attach from your IDE (VS Code: "Connect to server"; GoLand: Go Remote) once the plugin process exists.
- The plugin process is `gpx_plugin_<arch>` — find it with `docker exec grafana-dashboards pgrep -af gpx_plugin`.

### "Grafana won't come up"

Check supervisord first. If `[program:grafana]` is FATAL, the most common causes:

1. `dist/` is missing the Go binary for the container's arch → the `while [ ! -f /root/.../dist/gpx_plugin* ]` guard never exits. Either build the Go side (the container does this itself if `mage-watcher` is RUNNING) or wait for the watcher. **This guard reads repo-root `dist/` even when an alternate plugin directory is mounted** — an empty `dist/` hangs startup regardless of what `$PLUGIN_DIR` contains.
2. `directory=` mismatch on Grafana ≤12.0.x → the `/run.sh` race documented in the e2e skill's "Dev container's `/run.sh` race" section. Confirm `directory=/usr/share/grafana` in `.config/supervisord/supervisord.conf`.
3. Port 3000 already in use on the host → `lsof -iTCP:3000 -sTCP:LISTEN`. Free it or change the published port.

### Resetting state

```sh
docker compose down              # stop + remove containers (data volumes preserved)
docker compose down -v           # ALSO drop named/anonymous volumes — re-runs ClickHouse initdb
docker compose down --rmi local  # also drop locally-built images (forces full rebuild next up)
```

Use `down -v` when ClickHouse fixtures look stale or after editing `initdb.sql`.

## What this skill does NOT cover

- Building plugin artifacts → `build-plugin`.
- Running playwright e2e against this stack → `e2e-dev`. The playwright service in this same `docker-compose.yaml` is invoked via `docker compose run --rm playwright …`, not `up`.
- Testing a packaged release/RC **zip** → `e2e-run-zip`. It specialises this skill's "alternate plugin directory" mechanism: unzip in place, frozen mount, signature/version checks.
