<!-- Canonical skill body, hand-maintained and owned by this repo. Harness stubs
     (.claude/skills/e2e-dev/SKILL.md, .codex/skills/e2e-dev/SKILL.md)
     carry the frontmatter and @-import this file. Edit here, not in the stubs.
     WARNING: this file lives under .config/, which `create-plugin update`
     regenerates in place. Keep it committed and re-check it after any update. -->

# Grafana Plugin E2E (Playwright)

## Where things live

```
tests/*.spec.ts                          — specs (Playwright auto-discovers)
tests/helpers.ts                         — utilities + ConfigPageSteps
tests/grafanaSelect.ts                   — cross-version Select helpers
tests/queryEditorRow.ts                  — QueryEditorRow page-object
tests/variablePicker.ts                  — VariablePicker page-object
tests/adHocFilter.ts                     — AdHocFilter page-object (ad-hoc filters variable)
tests/dashboardBuilder.ts                — fluent DashboardBuilder (POSTs JSON)
.config/playwright/Dockerfile            — runner image (apt + node + deps + chromium)
.config/supervisord/supervisord.conf     — dev Grafana under supervisord
playwright.config.ts                     — projects, timeouts, retries
```

## Plugin artifacts the e2e suite consumes — **read this before testing source changes**

Grafana loads the plugin from `./dist/` via the bind mount in `docker-compose.yaml` (`../dist:/var/lib/grafana/plugins/hydrolix-hydrolix-datasource`). What's in `dist/` is what's running. The two artifact families have **different rebuild paths**:

| Artifact | Built by | When |
|---|---|---|
| `dist/gpx_plugin_linux_arm64` (and other arches) | `mage build:debug` | **Automatically inside the dev grafana container** via supervisord's `[program:mage-watcher]` (`mage -v watch`). The `[program:grafana]` shell wrapper blocks on `gpx_plugin*` existing before starting Grafana, so Go changes are always picked up after a container restart. No host action needed. |
| `dist/module.js`, `dist/module.js.map`, `dist/plugin.json` | `webpack` via `npm run build` / `npm run dev` | **On the host only.** Nothing in any container rebuilds these. |

**Before running e2e against a frontend source change** (anything in `src/components/`, `src/macros/`, `src/labels.ts`, etc.):

```sh
npm run build         # one-shot
#  — or keep this running in another terminal:
npm run dev           # webpack -w
```

If you skip this, the loaded plugin is whatever was last built. Your edit is in the repo but **not** in the running plugin, and tests silently pass/fail against stale frontend. This is the #1 way an agent gets a misleading "fix verified" result on a frontend change. *(Backend Go changes are immune — the container rebuilds them on every grafana restart.)*

Quick sanity check when behaviour doesn't match the source you're reading:

```sh
stat -f '%Sm %N' dist/module.js src/components/QueryEditor.tsx
```

If `module.js` predates your edit, the dev container is serving stale frontend.

## How to run

The e2e suite runs inside the `playwright` service in `docker-compose.yaml`. The service is built from `.config/playwright/Dockerfile`, which bakes Node 22, the npm dependencies (`npm ci`), and the Chromium browser into the image at build time. The image's `ENTRYPOINT` is `npx playwright test --reporter=list,junit`, so extra args passed to `run --rm` are appended naturally.

**Do I need to rebuild the image?**
- *Image doesn't exist locally* (first time, or `docker image rm` happened) → yes.
- `package.json` / `package-lock.json` changed → yes (the `npm ci` layer is busted).
- `@playwright/test` bumped → yes (so the bundled browser version stays in sync).
- Source-only changes (specs, helpers) → no, the bind mount picks them up.

```sh
# rebuild when one of the conditions above applies (one-shot, layer-cached)
docker compose build playwright

# full suite
docker compose run --rm playwright

# single test or grep — args append to the entrypoint
docker compose run --rm playwright --grep "interpolated query is shown"

# a single spec file
docker compose run --rm playwright tests/configEditor.spec.ts

# combine: file + grep
docker compose run --rm playwright tests/queryEditor.spec.ts --grep "template variable"
```

**Cost** (Apple M-series, observed in regression matrix):
- Image build, cold (no Docker layer cache): ~5 min.
- Image rebuild, warm (only npm-ci layer invalidated): ~2 min.
- Per `run --rm` invocation: **1.9–3.3 min** — anonymous-volume init from the image (~5 s) + the tests themselves. No network for deps at test time.

**Result artifacts** land in `./test-results/<test-name>/`:
- `error-context.md` — error message + page snapshot (YAML accessibility tree). Read this first.
- `test-failed-1.png`, `test-failed-2.png` — screenshots at failure.
- `trace.zip` — open with `npx playwright show-trace <path>` for step-by-step debugging.

`junit_report.xml` lands at repo root.

## docker-compose `playwright` service — non-obvious choices

- **Built from `.config/playwright/Dockerfile`**, not pulled. The Dockerfile installs node + npm deps + Chromium into the image so `run --rm` invocations start fast. Source changes don't bust the npm-ci layer (only `package.json` / `package-lock.json` do); `@playwright/test` bumps require a rebuild so the bundled browser stays in sync.
- **Base image: `ubuntu:24.04`, not `ubuntu`**. Bare `ubuntu` = Ubuntu 26.04 (Resolute) which Playwright ≤1.59 does not recognize — `playwright install --with-deps` fails with `does not support chromium on ubuntu26.04-arm64`.
- **Node 22 via NodeSource setup script**. `package.json` `engines` requires Node ≥22; Node 20 only logs an `EBADENGINE` warning but is a foot-gun.
- **`network_mode: "service:grafana"`**. The runner shares the grafana container's network namespace, so it sees the same `localhost:3000` Grafana sees. Don't reintroduce a separate network — it complicates `E2E_GRAFANA_URL`.
- **Anonymous volume `- /work/node_modules`**. Lets the bind mount of `.` over `/work` not blow away the image-baked `node_modules`. Each `docker compose run --rm` creates a *fresh* container, which gets a *fresh* anonymous volume that Docker initialises by copying the image's `/work/node_modules` into it. The `--rm` flag tears the container (and that anonymous volume) down on exit; the image itself is unchanged, so the next run repeats the cheap volume-init step rather than an npm install.
- **No `profiles:` block**. The user explicitly chose against profiles — invoke on-demand via `docker compose run --rm playwright`. (Running plain `docker compose up` will *also* try to start it; that's an accepted tradeoff.)
- **`CLICKHOUSE_*` / `HYDROLIX_*` declared empty** in `environment:` — Compose passes them through from the host shell when set; matches the pattern in `x-common-variables` at the top of the file.

## Locator patterns that work — and ones that don't

### `getByText` is a strict-mode trap when text is a substring of another role's name

The "Interpolated Query" heading and the "Hide Interpolated Query" button both contain the same substring. `page.getByText("Interpolated Query")` matches both → strict-mode violation.

```ts
// ✗ matches heading AND button → strict-mode violation
await expect(page.getByText("Interpolated Query")).toBeVisible();

// ✓ explicit role
await expect(
  page.getByRole("heading", { name: "Interpolated Query" })
).toBeVisible();
```

### Grafana's `Select` is react-select — drive it via the empty-value container

The visible "Choose" placeholder sits *under* a `[data-value=""]` wrapper that owns the `onClick` and intercepts pointer events. Clicking the text directly fails with "intercepts pointer events".

```ts
// ✗ intercepted by [data-value=""]
await queryRow.getByText("Choose").last().click();

// ✓ click the actual interactive surface
await queryRow.locator('[data-value=""]').last().click();

// alternative (less idiomatic but very robust):
await queryRow.getByText("Choose").last().click({ force: true });
```

Once the menu opens, options live in a portal at the document root (NOT inside `queryRow`), so the option click must be **page-scoped**. Match by inner text rather than accessible name so the locator works on Grafana 10 too (see "Cross-version" below):

```ts
// Inner text starts with the setting name on every version we support:
//   - 12+: "hdx_query_max_rows Set the maximum number of rows ..." (space separator)
//   - 10.x: "hdx_query_max_rowsSet the maximum number ..." (no separator)
// Anchor at start, but DON'T use `\b` after the prefix — on 10.x there is no
// word boundary between the prefix and the description (s→S is word→word).
await page
  .getByRole("option")
  .filter({ hasText: /^hdx_query_max_rows/ })
  .first()
  .click();
```

`'[role="select"]'` is **not a valid ARIA role** — neither `role="select"` nor `role="combobox"` selectors reliably open the menu. Stick to the `[data-value=""]` pattern.

### Always scope to the query row when the right-side panel exists

The panel-editor right pane (Visualization → Legend → Values) has its own Selects with the same "Choose" placeholder. Page-wide `.last()` will grab the wrong one.

```ts
const queryRow = panelEditPage.getQueryEditorRow("A");
await queryRow.getByText("Query Settings", { exact: true }).click();
await queryRow.getByLabel("new setting").click();
await queryRow.locator('[data-value=""]').last().click(); // empty row's name picker
// option click stays page-scoped (portal):
await page
  .getByRole("option")
  .filter({ hasText: /^hdx_query_max_rows/ })
  .first()
  .click();
await queryRow.getByLabel("hdx_query_max_rows").fill("42");
```

### Cross-Grafana-version locator differences

The CI matrix runs Grafana 10.4.x, 11.5.x, 12.0.x, 12.3.x, 13.0.x. Three on-page widgets render differently across that range; tests that touch them need version-agnostic locators.

**Dashboard variable picker on the dashboard page**

| Grafana    | Markup                                                 |
| ---------- | ------------------------------------------------------ |
| 10.x       | `<button aria-label="$varName">` containing the value  |
| 11.x–13.x  | react-select wrapped in `[data-value=""]` (no role)    |

```ts
// Open the variable picker (works on 10.x and 11.x–13.x).
await page
  .getByRole("button", { name: "tbl", exact: true })   // 10.x
  .or(page.locator('[data-value=""]'))                  // 11.x–13.x
  .first()
  .click();
```

**Dashboard variable dropdown items (after opening the picker)**

| Grafana    | Role         | Accessible name              |
| ---------- | ------------ | ---------------------------- |
| 10.x       | `checkbox`   | the value text (e.g. `no_such_table`) |
| 11.x–13.x  | `option`     | the value text                |

```ts
await page
  .getByRole("option", { name: "no_such_table" })           // 11.x–13.x
  .or(page.getByRole("checkbox", { name: "no_such_table" })) // 10.x
  .first()
  .click();
```

**QuerySettings Select option (accessible name vs inner text)**

react-select renders one `<option>` per setting on every version, but the *accessible name* differs:

| Grafana    | Option accessible name                       | Option inner text                        |
| ---------- | -------------------------------------------- | ---------------------------------------- |
| 10.x       | `"Select option"` (constant — useless)       | `"hdx_query_max_rowsSet the maximum…"` (no separator) |
| 11.x–13.x  | `"hdx_query_max_rows Set the maximum…"`      | `"hdx_query_max_rows Set the maximum…"`  |

Use `.filter({ hasText: /^prefix/ })` against inner text — works on both. Avoid `\b` after the prefix: on 10.x there's no word boundary between the setting name and the description start (`...rowsSet...`).

**Leading-whitespace trap on 11.5–12.x option markup.** Some option lists (observed: the ad-hoc filter operator listbox) indent the option's inner markup, so its raw text content starts with a newline. A bare `^prefix` anchor then matches **nothing** — the option is on screen and visible, yet `hasText` reports zero matches and the click times out. Anchor with `^\s*prefix` instead. `pickOptionByPrefix` / `pickOptionByExactText` in `tests/grafanaSelect.ts` already do this; the trap only bites hand-rolled regexes.

**Dashboard ad-hoc filters variable — three renderers**

| Grafana            | Renderer  | Entry point                                            |
| ------------------ | --------- | ------------------------------------------------------ |
| 10.4               | segments  | `+` button (`Add Filter`) → key / `=` / value segment buttons (`AdHocFilterKey-*` / `AdHocFilterValue-*` test ids) |
| 11.5 – 12.3        | combobox  | `input[placeholder="Filter by label values"]`          |
| 13.x               | combobox  | `input[placeholder="+ label = value"]`                 |

What triggers the value preload (`getTagValues`) also differs: picking the operator (combobox) vs clicking the value segment (segments). Don't hand-roll this — drive it through the `AdHocFilter` page-object (`tests/adHocFilter.ts`), which probes the DOM for the renderer (13.x's `AdHocFilter-label-announcer` live-region makes prefix test-id sniffing unreliable) and hides the differences behind `selectKey` / `openValues` / `reopenValues` / `pickValue` / `typeValue` / `dismiss`.

**Dashboard settings button**

| Grafana    | Accessible name        |
| ---------- | ---------------------- |
| 10.x–11.x  | `Dashboard settings`   |
| 12.x–13.x  | `Settings`             |

If you actually need to click into Settings → Variables (rare — prefer the API approach below), use `getByRole("button", { name: /^(?:Dashboard )?[Ss]ettings$/ })`.

**Prefer the API over the variable-creation UI**

The Settings → Variables UI shifts so much between versions (button names, tab labels, type-picker, value-input labels) that driving it from a test is a maintenance sink. Create the dashboard via Grafana's HTTP API instead — the JSON model is stable across all five versions:

```ts
const dsUid = dsConfigPage.datasource.uid;
const dsType = dsConfigPage.datasource.type;
const resp = await page.request.post("/api/dashboards/db", {
  data: {
    dashboard: {
      title: `template-var-test-${Date.now()}`,
      schemaVersion: 38,
      panels: [{
        id: 1, title: "panel", type: "table",
        datasource: { type: dsType, uid: dsUid },
        targets: [{ refId: "A", datasource: { type: dsType, uid: dsUid }, rawSql: "..." }],
        gridPos: { x: 0, y: 0, w: 24, h: 8 },
      }],
      templating: {
        list: [{
          name: "tbl", type: "custom", query: "macros,no_such_table",
          current: { text: "macros", value: "macros", selected: true },
          options: [
            { text: "macros", value: "macros", selected: true },
            { text: "no_such_table", value: "no_such_table", selected: false },
          ],
          includeAll: false, multi: false,
        }],
      },
      time: { from: "2025-04-10T00:00:00.000Z", to: "2025-04-10T23:59:59.000Z" },
      timezone: "utc",
    },
    overwrite: true,
  },
});
const { uid } = await resp.json();
const dashboardPage = await gotoDashboardPage({ uid });
```

### Grafana `IconButton`: tooltip overrides aria-label

When both `aria-label` and `tooltip` are passed, Grafana derives the rendered `aria-label` from the **tooltip**. So `<IconButton aria-label="copy-formatted-data-to-clipboard" tooltip="copy to clipboard" />` is queried as:

```ts
page.getByRole("button", { name: "copy to clipboard" }) // ✓
page.getByLabel("copy-formatted-data-to-clipboard")    // ✗ doesn't exist
```

### Where per-query errors render — `PanelDataErrorMessage` is the wrong target

When the backend returns both an `error` **and** an empty `frames` array (which the Hydrolix backend does for `Code: 62` syntax errors — see `pkg/.../...` and the JSON returned at `/api/ds/query`), Grafana puts `"No data"` in `data-testid="data-testid Panel data error message"` rather than the error text. The beautified per-query error from `ErrorMessageBeautifier` instead appears in a **plain `<div>` inside `QueryEditorRow`** — no test-id, just hashed CSS classes (`css-1c84tl5` and similar). Scope to the row and assert on text:

```ts
// ✗ matches but contains "No data", never the beautified error
panelEditPage.getByGrafanaSelector(
  selectors.components.Panels.Panel.PanelDataErrorMessage
);

// ✓ scope to the query row, find by text
const queryRow = panelEditPage.getQueryEditorRow("A");
await expect(queryRow.getByText(/Syntax error/i)).toBeVisible({ timeout: 30000 });
// And separately assert the raw transport noise doesn't leak through:
await expect(queryRow).not.toContainText(/error querying the database|sendQuery|HTTP 400|DB::Exception/i);
```

Also note the nesting in `@grafana/e2e-selectors`: `PanelDataErrorMessage` lives at `selectors.components.Panels.Panel.PanelDataErrorMessage`, **not** `selectors.components.Panels.PanelDataErrorMessage`. `getByGrafanaSelector(undefined)` fails with `TypeError: Cannot read properties of undefined (reading 'startsWith')` — when you see that, the selector path is wrong; check `node_modules/@grafana/e2e-selectors/dist/types/selectors/components.d.ts`.

## State-dependent flows in `QueryEditor`

### `Show Interpolated Query` — how the dryRun fallback works (no workaround needed)

`QueryEditor.tsx`'s interpolation debounce has two branches:

```ts
useDebounce(async () => {
  if (showSql || SHOW_VALIDATION_BAR) {
    if (props.datasource.options) {
      setInterpolationResult(await props.datasource.interpolateQuery(...));
    } else {
      dryRun();          // ← fallback: fires onRunQuery with skipNextRun
    }
  }
}, 300, [showSql, interpolationId, props.datasource.options]);
```

`props.datasource.options` is mutated by `datasource.query()` (see `src/datasource.ts:91-93`) — interpolation reads time range / scoped vars / filters from it. On a freshly-opened panel editor it's `undefined` until Grafana runs the query for the first time. When the user clicks "Show Interpolated Query" *before* any panel refresh has completed, the debounce takes the `dryRun()` branch, which calls `onRunQuery` with a `skipNextRun` flag — Grafana runs the query, sets `this.options = request`, but the target's SQL is skipped.

`props.datasource.options` is listed as the third dependency of `useDebounce`. After `dryRun` populates `options` and the `setDryRunTriggered(true)` re-render lands, deps change (undefined → request object), the debounce re-fires, and the second pass takes the `interpolateQuery` branch. The user sees "processing" briefly (~300 ms × 2 debounces) then the copy button appears with the expanded SQL.

**Historical note**: before the third dependency was added, the debounce never re-fired after `dryRun()` populated options. The first click of "Show Interpolated Query" left the spinner stuck forever, and tests had to call `panelEditPage.refreshPanel()` before clicking Show as a workaround. That workaround is no longer needed — the e2e test in `tests/queryEditor.spec.ts` confirms the unguarded flow works.

## Unit-test parallels worth knowing

These aren't e2e patterns but bite the same code under Jest in `src/components/*.test.tsx`. Captured here so an agent fixing one layer doesn't re-learn it in the other.

### Controlled inputs need a stateful wrapper (Jest)

Grafana's `Input` is fully controlled by `props.query`. With a static `jest.fn()` for `onChange`, the DOM falls out of sync after each keystroke — `clear` then `type "X"` produces `"SELECT 1X"` because React re-syncs the DOM to the unchanged prop. For keystroke-driven assertions (round, rawSql, invalid duration), use a `<StatefulHarness>` that feeds `onChange` back into local state. See `src/components/QueryEditor.test.tsx`.

For e2e the real component tree updates props normally, but: prefer `fireEvent.change(input, { target: { value: ... } })` over `userEvent.type` when a test runs faster than React's controlled re-render can keep up.

## Timeouts

- Default per-test budget: **60s** (`playwright.config.ts:timeout`).
- Per-test override when needed: `testInfo.setTimeout(150000)` inside the test body.
- For interpolation specifically (Go backend cold-call): the copy button can take 10–60s to appear. Use `await expect(copyButton).toBeVisible({ timeout: 60000 })`. Don't trust the default 5s.

## Timezone discipline — load-bearing

The fixture data in `e2e.macros` is in UTC. The playwright project pins `timezoneId: "UTC"` (`playwright.config.ts`) so the browser doesn't render timestamps in the host's local zone. But that's only half the story — every panel time-range set must *also* be UTC, or the panel sends `from`/`to` in the host's zone:

```ts
await panelEditPage.timeRange.set({
  from: "2025-04-10 00:00:00",
  to:   "2025-04-10 23:59:59",
  zone: "Coordinated Universal Time",  // required — or fixture rows drift by host TZ offset
});
```

Omitting `zone` causes 1–24h drift depending on host timezone, which manifests as "rows missing from the panel" in `runs a SELECT and renders fixture rows…`-style tests. The error looks like a data bug and is hard to spot.

## `expect.poll` vs `await expect(locator)`

Two different shapes for two different things:

- **`await expect(locator).toX()`** — re-checks the same DOM probe until it passes or times out. Use for "this element exists / has this state / contains this text". Polling is implicit and cheap because Playwright reuses the locator.
- **`expect.poll(() => valueFn(), { timeout })`** — re-invokes `valueFn` each tick. Use for **mutable JS state captured by route handlers** (e.g. an array of SQL strings appended as requests fire). `await expect(sqls.some(...))` would lock in the array's state at call time and miss later pushes; `expect.poll` re-reads each tick.

Pattern in this suite for captured-SQL assertions:

```ts
const sqls = await captureSqls(context);
await dashboardPage.refreshDashboard();
await expect
  .poll(() => sqls.some((s) => /FROM\s+e2e\.macros\b/i.test(s)), { timeout: 30000 })
  .toBe(true);
```

## Test isolation

The suite runs with 4 parallel workers against a single shared Grafana. Tests must not collide on shared state:

- **Each test creates its own datasource** with a unique name via `createDatasourceConfigPage(name, …)`. Existing convention is to name them after the test ("render", "valid config", "queryEditor querySettings", etc.).
- **`deleteDataSourceAfterTest: false`** is intentional in `ConfigPageSteps.createDatasourceConfigPage`. Teardown races between parallel workers + retries were the original problem; persisting the DS until the next clean-up cycle is cheaper than fighting the race.
- **Don't reuse a hard-coded dashboard uid.** When a test needs a pre-built dashboard, build a fresh one via `DashboardBuilder` (the title carries `Date.now()` by default).

## Known pre-existing flake — `macroFunctions` first serial test

`tests/macroFunctions.spec.ts` is `test.describe.configure({ mode: "serial" })` with a shared `panelEditPage`. The *first* test in that block (typically `fromTime` or `fromTime_ms`, whichever Playwright schedules first) flakes ~20% on cold runs with a transient **404 from `panelEditPage.refreshPanel()`** — the datasource is created but `/api/ds/query` isn't routable in the first ~100 ms. The next test runs, the route is up, the retry passes. Playwright marks the test as "flaky" but the suite is green.

This pre-dates the abstractions added in v0.3.0 / v0.4.0. Don't dig in unless retry #1 *also* fails — at that point the symptom is real, not a race.

## Capturing request bodies for assertions

When asserting on what the plugin actually **sent** (template-var substitution, querySettings serialization, etc.), intercept and pass-through:

```ts
let capturedSqls: string[] = [];
await context.route("**/api/ds/query**", async (route, request) => {
  if (request.method() === "POST") {
    try {
      const json = JSON.parse(request.postData() ?? "");
      const sql = json?.queries?.[0]?.rawSql;
      if (sql) capturedSqls.push(sql);
    } catch {}
  }
  // Wrap fetch+fulfill in try/catch: when the page navigates or the test
  // ends between the two calls, the Response is disposed and fulfill throws
  // `route.fulfill: Fetch response has been disposed`. Seen intermittently
  // on Grafana 11.x. Swallowing it doesn't drop captured data (we pushed
  // before fetching) and keeps the route from failing the test.
  try {
    const response = await route.fetch();
    await route.fulfill({ response });
  } catch {
    // route disposed / target closed — ignore
  }
});
```

Pattern works for `/interpolate` (use `.includes("/interpolate")` filter) and `/api/ds/query` (use `**/api/ds/query**`).

## Clipboard testing

Permissions must be granted at the **context** level before any clipboard read:

```ts
await context.grantPermissions(["clipboard-read", "clipboard-write"]);
// ... interactions ...
const text = await page.evaluate(() => navigator.clipboard.readText());
```

Only Chromium supports this reliably in headless mode.

## Test-side abstractions — reach for these first

Before reaching for raw locator chains, check whether one of the existing helpers/page-objects already covers the surface. They bake in the cross-version `.or()` chains documented in this skill, so callers can stay version-agnostic.

`tests/helpers.ts`:
- `ConfigPageSteps` — proxy-based locator factory keyed off `src/labels.ts`. Method names like `defaultDatabase()`, `defaultRound()`, `passwordReset()` chain through `ElementContext.input`/`button`/`switch`/`expandable`/`alert`/`timerange` based on the trailing suffix in the method name.
- `closeWhatsNewDialog(page)` — silently dismisses the "What's new in Grafana" modal that appears on first login per Grafana version.
- `queryTextSet(refId, query, panelEditPage)` — types into Monaco via Ctrl+A then keyboard.type.
- `expandQueryOptions(page)` — handles Grafana 12 vs 13 difference for the "Query options" toggle.
- `setMaxDataPoints(page, value)` — for macro tests that rely on `$__timeInterval` resolution.
- `captureSqls(context, urlPattern?)` — installs a passthrough route handler that returns a mutable array of `rawSql` strings; wraps fetch+fulfill in try/catch (`Fetch response has been disposed` swallow). Default pattern is `**/api/ds/query**`.
- `captureRequestBodies(context, urlPattern?)` — same shape but captures the full POST body string when you need more than `rawSql`.

`tests/grafanaSelect.ts` — cross-version Select helpers:
- `openGrafanaSelect(root)` — clicks the `[data-value=""]` wrapper scoped to `root.last()`.
- `pickOption(page, name)` — page-scoped, `getByRole("option").or(getByRole("checkbox"))` to span 11+ vs 10.
- `pickOptionByPrefix(page, prefix)` — same but matches by inner-text prefix (no `\b` after the prefix; tolerates leading whitespace — see Cross-version section).
- `pickOptionByExactText(page, text)` / `optionByExactText(page, text)` — exact inner-text match, whitespace-tolerant, regex-escaped. Use when the label is a prefix of a sibling (`status` vs `status_null`) or on 10.x where every option's accessible name is the constant "Select option".
- `visibleOptionTexts(page)` — trimmed inner text of every rendered option; the building block for "which values did the dropdown offer" assertions.

`tests/adHocFilter.ts` — `AdHocFilter` page-object for the dashboard's ad-hoc filters variable (three renderers across the matrix — see Cross-version section):
- `new AdHocFilter(page)` then `selectKey(key)`, `openValues({timeout, waitForOptions})` → `{options, elapsedMs}`, `reopenValues(key, opts)` (proves a second `getTagValues` fires), `pickValue(value)`, `typeValue(value)` (manual entry of a non-suggested value), `dismiss()`.
- `elapsedMs` starts at the click that actually issues `getTagValues` on each renderer, so timing-budget assertions are comparable across versions. `waitForOptions: false` is for preloads that legitimately return nothing.

`tests/queryEditorRow.ts` — `QueryEditorRow` page-object wrapping `panelEditPage.getQueryEditorRow(refId)`:
- `setSql(sql)`, `setRound(duration)`, `openQuerySettings()`, `addQuerySetting(name, value)`, `toggleInterpolatedQuery(show)`.

`tests/variablePicker.ts` — `VariablePicker` page-object for a dashboard's on-page template-variable picker:
- `new VariablePicker("tbl", page).select("no_such_table")` — opens the picker (G10 button or G11+ `[data-value=""]`) and clicks the option (G10 checkbox or G11+ option).

`tests/dashboardBuilder.ts` — fluent `DashboardBuilder` that POSTs a dashboard JSON via `/api/dashboards/db`:
- `new DashboardBuilder(page, dsConfigPage.datasource).withTitle(...).addCustomVariable(...).addPanel(...).withTimeRange(...).create()` → `{ uid }`.
- Prefer this over Settings → Variables UI for any test that needs predefined variables / panels.

Adding new locator methods to the `ConfigPageLocator` interface flows automatically through the proxy; just match the naming convention. For other surfaces, extend the page-object that owns them rather than inlining locators in the spec.

### Recipe for a new spec

Most existing tests are some variation of these six steps. Reach for the abstractions instead of re-deriving locators.

```ts
import { test, expect } from "@grafana/plugin-e2e";
// @ts-ignore
import { captureSqls, closeWhatsNewDialog, ConfigPageSteps } from "./helpers";
import { DashboardBuilder } from "./dashboardBuilder";
import { QueryEditorRow } from "./queryEditorRow";
import { VariablePicker } from "./variablePicker";

test("<what is being asserted>", async ({
  createDataSourceConfigPage,
  gotoDashboardPage,        // OR dashboardPage if no pre-built dashboard needed
  page,
  context,
}) => {
  // 1. Create the datasource through the UI fixture (gives a real uid).
  const steps = new ConfigPageSteps(page);
  const dsConfigPage = await steps.createDatasourceConfigPage(
    "<unique test name>",   // must be unique within the suite — see "Test isolation"
    createDataSourceConfigPage,
  );
  await steps.fillTestNativeDatasource();
  await steps.saveSuccess(dsConfigPage);

  // 2. Build the dashboard you need (API path is stable; UI is not).
  //    Skip this block if you just need an empty panel editor via addPanel().
  const { uid } = await new DashboardBuilder(page, dsConfigPage.datasource)
    .addCustomVariable({ name: "tbl", query: "macros,no_such_table" })
    .addPanel({ rawSql: "SELECT … FROM e2e.$tbl …" })
    .withTimeRange("2025-04-10T00:00:00.000Z", "2025-04-10T23:59:59.000Z")
    .create();

  // 3. Capture outgoing requests BEFORE the action that triggers them.
  const sqls = await captureSqls(context);

  // 4. Navigate / interact.
  const dashboardPage = await gotoDashboardPage({ uid });
  await closeWhatsNewDialog(page);
  await new VariablePicker("tbl", page).select("no_such_table");
  await dashboardPage.refreshDashboard();

  // 5. Assert on captured state with expect.poll (mutable array → not await expect).
  await expect
    .poll(() => sqls.some((s) => /FROM\s+e2e\.no_such_table\b/i.test(s)), { timeout: 30000 })
    .toBe(true);
});
```

For panel-edit tests that don't need a predefined variable, replace step 2 with `const panelEditPage = await dashboardPage.addPanel()` and wrap the query row in `new QueryEditorRow(panelEditPage, "A")`.

### `@step` decorator (consistency note)

`ConfigPageSteps` decorates its methods with `@step` (defined in `tests/helpers.ts`) so Playwright's HTML report and trace viewer show grouped, named substeps. New page-objects in `tests/queryEditorRow.ts` / `tests/variablePicker.ts` / `tests/dashboardBuilder.ts` *do not* use it currently — methods appear inline in the trace. If trace readability becomes an issue, add `@step` to the page-object methods (the decorator is generic and just wraps the method body in `test.step(...)`).

## When debugging a failing test

1. Read `test-results/<test-name>/error-context.md`. The accessibility-tree snapshot tells you what's actually on screen.
2. If "intercepts pointer events" appears → see react-select section above.
3. If "resolved to 2 elements" → use `getByRole(...)` with a specific role to disambiguate.
4. If the expected locator just isn't there → check the page snapshot. Common causes: collapsed section, wrong tab, modal still up, the right-side panel matched first, **or you're on a Grafana version where the widget renders with a different role** (see "Cross-Grafana-version locator differences").
5. If `route.fulfill: Fetch response has been disposed` shows up → wrap the route handler's fetch+fulfill in try/catch (see "Capturing request bodies"). The built-in `captureSqls` / `captureRequestBodies` helpers already do this.
6. For interpolation-related failures, add a network logger temporarily:
   ```ts
   page.on("request", (r) => r.url().includes("/interpolate") && console.log("→", r.method(), r.url()));
   page.on("response", async (r) => r.url().includes("/interpolate") && console.log("←", r.status(), await r.text()));
   ```
   Then remove once diagnosed.
7. If a test passes on retry but fails first → check for navigation/cleanup races (e.g. `Fetch response has been disposed`). The disposable-route wrap from "Capturing request bodies" usually eliminates the flake.
8. Live Grafana inspection: `localhost:3000` in your browser is the *same* Grafana the tests hit (the playwright runner joins its network namespace). You can reproduce a test's setup steps in the UI and watch the failure live. `docker compose logs -f grafana` streams server logs alongside.
9. **Test behaviour doesn't match the source you just edited?** Run `stat -f '%Sm %N' dist/module.js src/components/<file>.tsx`. If `module.js` is older than your edit, the dev container is loading stale frontend — run `npm run build` (or keep `npm run dev` going) and re-test. See "Plugin artifacts the e2e suite consumes" near the top. Go changes are auto-rebuilt by the container's mage-watcher; only the frontend has this trap.
10. **Inspect the actual request/response bodies in `trace.zip`** when the UI shows something surprising (e.g. `"No data"` instead of an error). `unzip -q trace.zip -d /tmp/x && grep -l rawSql /tmp/x/resources/*.json` finds the POST bodies; the matching response body is referenced by `_sha1` in the same trace's `*.network` file. Use this to confirm whether (a) the request was even sent with the SQL you typed, (b) the backend returned an error, and (c) whether the backend response also carried empty frames (which Grafana renders as `"No data"`, masking the error — see "Where per-query errors render" above).
11. **Validate the SQL against ClickHouse directly** when an e2e test gives surprising output: `docker exec clickhouse-server clickhouse-client -u testuser --password testpass --query "<sql>"`. Distinguishes "the query is actually invalid" from "the plugin's error handling masks the failure." Note that ClickHouse's exact error wording varies by version — what the beautifier strips may differ (e.g. newer ClickHouse appends `(version X.Y.Z (official build))` to error lines).

### Trace & screenshot policy

`playwright.config.ts` ships with:
- `screenshot: "only-on-failure"` — no screenshots when a retry succeeds or when the run is green.
- `trace: "on-first-retry"` — **the first failure has no trace.zip**; only retries do. Locally `retries: 0`, so a failing test on dev gives you `error-context.md` + screenshots but **no trace**. CI sets `retries: 2`, so attempts 2 and 3 leave traces in `test-results/<name>-retry1/` and `-retry2/`.

If you need a trace for a stubborn local failure: bump `retries` in `playwright.config.ts` to `1` temporarily, or override `--retries=1` on the command line: `docker compose run --rm playwright --retries=1 <spec>`. Don't commit the change.

## Switching Grafana versions locally

`docker-compose.yaml` exposes the `grafana_version` build arg via commented-out toggles. Switch the active line, then:

```sh
docker compose stop grafana
docker compose build grafana
docker compose up -d --no-deps grafana          # --no-deps skips keycloak
until curl -sf http://localhost:3000/api/health >/dev/null; do sleep 2; done
```

The suite has been verified against `10.4.16`, `11.5.4`, `12.0.2`, `12.3.1`, `13.0.1`. Budget **1–2 min** (build arg change is layer-cache friendly; the slow step is Grafana's first-startup plugin install via mage) plus **2–3 min** per test run.

Note: these local toggles are the *earliest minor we still support* per channel. The CI matrix (`.github/`) uses the latest patch within each minor — `10.4.18`, `11.6.1`, `12.0.2`, `13.0.1` — so what passes locally on `10.4.16` should pass on `10.4.18` in CI, but minor-version skews are the place to look first when a Grafana-side test breaks only in CI.

### Pointing tests at a non-compose Grafana

`playwright.config.ts` honors `E2E_GRAFANA_URL`. To run the suite against a Grafana that isn't the dev compose service (e.g. a CI integration env, or a remote dev cluster), set the env var on the host shell:

```sh
E2E_GRAFANA_URL=https://grafana.example/ docker compose run --rm playwright tests/configEditor.spec.ts
```

The compose service propagates the variable to the container. Auth still uses plugin-e2e's `auth.setup.js` (basic admin login), so this only works against a Grafana that accepts those credentials — OAuth flows are out of scope for the current suite.

### Dev container's `/run.sh` race on Grafana ≤ 12.0.x

The dev image runs Grafana under supervisord. The grafana program is:

```
command=bash -c '... ; /run.sh'
directory=/usr/share/grafana
```

`directory` **must** be Grafana's homepath (`/usr/share/grafana`), not the data dir. `/run.sh` shells out to `grafana cli ... plugins install` when `GF_INSTALL_PLUGINS` is set; on Grafana ≤ 12.0.x the cli reads its config from cwd rather than honoring `--homepath`, so running it from `/var/lib/grafana` fails with `Grafana-server Init Failed: Could not find config defaults`. The script's `set -e` then aborts before `grafana server` starts and supervisord burns through its retries, entering FATAL. On 12.3+ the cli resolves config differently and tolerated the wrong cwd, masking the bug.

If a freshly-built grafana container is unresponsive on `:3000`, check `docker exec grafana-dashboards supervisorctl status grafana`. FATAL → the `directory=` is wrong.

## Versions in play

- `@playwright/test ^1.52.0` (resolves to **1.59.1** today).
- `@grafana/plugin-e2e ^3.4.1` (resolves to **3.7.0**; latest stable 3.7.2; no 4.x yet).
- Peer-dep alignment: bumping `plugin-e2e` is the lever, not `@playwright/test` standalone. Re-evaluate when 4.x lands.
- Verified Grafana versions (this branch): 10.4.16, 11.5.4, 12.0.2, 12.3.1, 13.0.1. CI matrix in `.github/`: 10.4.18, 11.6.1, 12.0.2, 13.0.1.

## Gaps still worth filling (audit reference)

Already implemented: **#3** additional settings persistence, **#8** interpolated query show/hide + clipboard, **#9** querySettings round-trip via request body, **#1** password secureJsonData round-trip, **#6** run-and-render a panel, **#12** template variable substitution, **#13** ad-hoc filters (`tests/adHocFilterValues.spec.ts` + `tests/adHocGuardrails.spec.ts`, driven through `AdHocFilter` + `DashboardBuilder.addAdHocVariable()`).

Still open: **#2** TLS skip-verify gating + bad-cert error, **#4** port-mandatory-only-when-useDefaultPort-off, **#5** secureSocksDSProxyEnabled, **#7** query-type switching, **#10** round invalid duration, **#11** format query button, **#14** annotations / metricFindQuery (annotations covered by `tests/annotations.spec.ts`; metricFindQuery still open), **#15** save & test with unreachable host, **#16** OAuth pass-through.

Most of the open items hit surfaces already covered by the abstractions:
- #7 / #10 / #11 → extend `QueryEditorRow` (`setQueryType`, validate round-error state, `formatQuery`).
- #14 (metricFindQuery) → `captureRequestBodies` against `/api/ds/query` for the variable-query body.
- #2 / #4 / #5 / #15 → `ConfigPageSteps` proxy (add new locators following the existing naming convention).
- #16 → new auth path; out of scope for the current Playwright `auth.setup.js`.

Add new tests as variants of the [recipe](#recipe-for-a-new-spec) above; the building blocks are documented.
