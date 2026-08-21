# Hydrolix Grafana Datasource Plugin — Coding Reference

Day-to-day reference for working in this repo. The high-level coding
principles (MUST/SHOULD/CAN tiers, datasource invariants, testing
philosophy, OpenSpec artifact rules) live in `openspec/config.yaml` so
OpenSpec artifacts also pick them up. This file is for specifics: paths,
commands, quality gates, project quirks, and pointers to the skills.

## Skills (use these for build / run / e2e)

Prefer invoking the skill over hand-rolling commands — they cover the
dist/ ↔ working-tree sync rules, the docker-compose service map, locator
patterns, and the host-vs-container build split.

**Layout:** skill *bodies* are canonical in `.config/AGENTS/skills/<name>.md`,
shared across harnesses. `.claude/skills/<name>/SKILL.md` and
`.codex/skills/<name>/SKILL.md` are frontmatter-only stubs that `@`-import the
body. **Edit the body in `.config/AGENTS/skills/`, never the stubs** — an edit to
one stub silently diverges from the other harness. This mirrors the scaffold's
own fan-in pattern (`CLAUDE.md` / `AGENTS.md` / `GEMINI.md` →
`.config/AGENTS/instructions.md`).

**⚠ The skill bodies live inside `.config/`, which `create-plugin update`
regenerates in place.** The scaffold's blanket "never edit `.config/`" rule does
not apply to `.config/AGENTS/skills/` — those files are ours — but the update
hazard is real: **keep them committed**, and after running `create-plugin update`
diff `.config/AGENTS/skills/` and restore anything it clobbered. There is no
copy outside `.config/` to fall back on.


- **`build-plugin`** — frontend webpack bundle (`dist/module.js*`)
  and Go backend binary (`dist/gpx_plugin_*`). Covers when each needs a
  rebuild, watch vs one-shot, and the typecheck/lint/unit verification
  chain. Invoke for any task touching `src/`, `pkg/`, `Magefile.go`, or
  `webpack.config.ts`.
- **`stack-run`** — docker-compose dev stack (Grafana with the
  plugin bind-mounted, ClickHouse, Keycloak, on-demand playwright). Covers
  `docker compose up` semantics, the `dist/` prerequisite, **serving plugin
  artifacts from an alternate folder** (any path, live or frozen, via an
  ephemeral compose override), switching Grafana versions, hot-reload, delve
  attach, OAuth/anonymous auth, ClickHouse fixture access.
- **`e2e-dev`** — Playwright tests via the docker-compose
  `playwright` service. Covers locator patterns (Grafana react-select,
  Monaco editor, panel chrome), state-dependent QueryEditor interactions,
  and timeout/retry expectations.
- **`validate-plugin`** — pre-release gate: zips `dist/` and runs Grafana's
  official `@grafana/plugin-validator`. Supersedes the generic scaffold recipe
  with this repo's deviations (`-sourceCodeUri` is mandatory, `buildAll` only).
  Invoke before publishing or submitting a release artifact.
- **`e2e-run-zip`** — run the stack / e2e against a *packaged*
  plugin zip (release/RC artifact) instead of `dist/`. Unpacks into a
  timestamped `dist_<ts>/dist` sandbox with copies of the compose artifacts,
  then delegates bring-up to `stack-run` in frozen mode (read-only mount +
  `DEV=false`) so the mage-watcher can't rebuild the packaged binary. No
  `dist/` substitution; sandbox cleanup requires user approval. Invoke for
  "test this plugin package / zip".

## Layout

| Path                              | What                                           |
|-----------------------------------|------------------------------------------------|
| `src/`                            | TypeScript + React frontend                    |
| `src/datasource.ts`               | `DataSource` extends `DataSourceWithBackend`   |
| `src/module.ts`                   | Plugin module — `setConfigEditor`, `setQueryEditor`, `setAnnotationSupport` |
| `src/editor/metadataProvider.ts`  | Schema/autocomplete; `ZERO_TIME_RANGE` sentinel |
| `src/plugin.json`                 | Capabilities manifest                          |
| `src/types.ts`                    | `HdxQuery`, `QueryType`, datasource options    |
| `pkg/`                            | Go backend (sqlds-based)                       |
| `pkg/plugin/driver.go`            | Backend driver; `panelId` / `panelName` attribution at `:421-467` |
| `tests/`                          | Playwright e2e                                  |
| `tests/helpers.ts`                | Shared e2e helpers                              |
| `dist/`                           | Built artifacts (`module.js*`, `gpx_plugin_*`)  |
| `.config/AGENTS/skills/`          | Canonical skill bodies (shared by `.claude` + `.codex` stubs) |
| `openspec/`                       | OpenSpec config and changes                     |
| `openspec/config.yaml`            | General coding principles + artifact rules     |
| `openspec/changes/`               | Active change proposals                        |
| `Magefile.go`                     | Go build targets                               |
| `docker-compose.yaml`             | Dev stack definition                           |

## Package management

- Package manager: **npm**. Lockfile is `package-lock.json`. No pnpm, no yarn.
- Frequently used scripts (full list in `package.json`):
  - `npm run typecheck` — `tsc --noEmit`
  - `npm run lint` — `eslint --cache .`
  - `npm test` — Jest in **watch mode on changed files only** (`jest --watch --onlyChanged`); use `npm run test:ci` for a one-shot full sweep
  - `npm run build` — webpack production bundle (`dist/module.js*`)
  - `npm run e2e` — Playwright (prefer `e2e-dev` skill)

Go is built via `mage` **inside the dev container**, not on the host.
Host Go picks up the wrong toolchain. See `build-plugin`.

## Quality gates (pre-PR)

| Gate                                | Level | Notes                       |
|-------------------------------------|-------|------------------------------|
| `npm run typecheck`                 | MUST  |                              |
| `npm run lint`                      | MUST  |                              |
| `npm run test:ci`                   | MUST  | `--passWithNoTests`: check count ≠ 0 |
| `go vet ./...`                      | MUST  |                              |
| `golangci-lint run`                 | MUST  |                              |
| `go test -race ./...`               | MUST  |                              |
| E2E via `e2e-dev`        | SHOULD | for behavior-affecting changes |
| `npm run build` produces clean dist | MUST  | before running e2e            |

## Datasource specifics

- `src/datasource.ts` `query()` caches request state on the instance:
  - `this.options = request` — gated by `request.range !== ZERO_TIME_RANGE`.
    Read later by `getTagKeysForMap` and `getInterpolatedQuery`.
  - `this.filters = request.filters` — gated by `app === CoreApp.Dashboard`.
    Same readers.
- `ZERO_TIME_RANGE` is the sentinel `{from: 0, to: 0}` defined in
  `src/editor/metadataProvider.ts`. Internal metadata queries use it so they
  don't clobber the cached dashboard range.
- Annotation queries (spec: `annotations`) arrive from Grafana with
  `app === CoreApp.Dashboard`. The plugin retags them at `query()` entry
  to `app === 'annotation'` so the existing guard naturally skips the
  filter cache assignment.
- Macro expansion lives server-side via the `/interpolate` backend resource
  (`getInterpolatedQuery` in `datasource.ts`). The frontend ships SQL +
  range + filters; do not duplicate macro logic on the frontend.
- `pkg/plugin/driver.go:421-467` attributes queries by `panelId` /
  `panelName` with an `"unknown"` fallback. Annotation queries arrive
  without these — the fallback is expected.

## E2E locator quirks

- Grafana react-select widgets (variable picker, querySettings picker,
  etc.): **always** start with `[data-value=""]`. Other locators silently
  time out for ~12 minutes per miss. Already captured in global memory
  (`feedback_grafana_select_locator`).
- Monaco editor, panel chrome, state-dependent QueryEditor interactions:
  see the `e2e-dev` skill.

## Logging

- Frontend errors: use `@grafana/runtime`'s `logError` (see the `query()`
  error path in `src/datasource.ts`). Never ship `console.error`.
- Go: structured logger that already lives in `pkg/`. Never log secrets
  (auth tokens, OAuth credentials, datasource secure JSON).

## OpenSpec

- Slash commands: `/opsx:propose` (new change), `/opsx:apply` (implement),
  `/opsx:verify` (validate vs artifacts), `/opsx:archive` (finalize).
- General principles and artifact format rules: `openspec/config.yaml`.
- Reference OpenSpec work **by capability name only** (e.g. the `annotations`
  spec), never by status or by a `openspec/changes/...` path — changes get
  archived under a date prefix, so paths and statuses go stale. Look the
  current state up in `openspec/specs/` and `openspec/changes/` when it
  matters.
- A change ships when all artifacts (proposal, design, specs, tasks) are
  done **and** the tasks checklist is complete.

## Findings & memory

- Per-topic exploration notes live in `.claude/findings/` — read before
  starting an open-ended change in the same area.
- Cross-conversation memory is global, at
  `~/.claude/projects/.../memory/`. Salient: the react-select locator
  rule and any other feedback the user has added there.

---

This plugin ships into Grafana's process and talks to a production
Hydrolix cluster. Correctness over cleverness; tests over assumptions.
