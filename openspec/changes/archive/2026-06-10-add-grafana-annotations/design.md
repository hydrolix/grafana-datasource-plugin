## Context

The Hydrolix datasource plugin extends `DataSourceWithBackend` and routes all queries through the sqlds backend. Grafana's annotation pipeline is built around the `AnnotationSupport<TQuery>` interface from `@grafana/data`: a datasource exposes a handful of hooks via its `annotations` instance property (set in the `DataSource` constructor — `DataSourcePlugin` in `@grafana/data@12.x` has no `setAnnotationSupport` builder method), Grafana renders the annotation editor for it, and stored annotation definitions flow through `(optional prepareAnnotation) → prepareQuery → standard runQuery pipeline → frame-to-event mapper`.

**Compatibility baseline**: this plugin only supports Grafana ≥ 10. Annotations on 10+ are always stored in the modern `AnnotationQuery<HdxQuery>` shape (`target: HdxQuery`); there is no pre-8.0 `rawQuery` string shape to migrate. The plugin has also never previously offered annotation support, so no historical Hydrolix annotation JSON exists in any form. Migration concerns drop out entirely — `prepareAnnotation` is not registered.

Today:
- `src/plugin.json` declares `metrics`, `backend`, `alerting`, `multiValueFilterOperators` — no `annotations`.
- `src/types.ts` has `QueryType { TimeSeries=0, Table=1, Logs=2 }` and no annotation variant.
- `src/datasource.ts:92-97` caches the latest `DataQueryRequest` on the instance:
  ```ts
  if (request.range !== ZERO_TIME_RANGE) { this.options = request; }
  if (request.app === CoreApp.Dashboard) { this.filters = request.filters; }
  ```
  These cached values feed `getTagKeysForMap` (line 334), `getInterpolatedQuery` (lines 345–346), and other metadata calls. Annotation queries arrive from Grafana with `app === CoreApp.Dashboard`, so without intervention they would participate in the second assignment and clobber the cached panel filters.
- `pkg/plugin/driver.go:421-467` attributes queries by `panelId` / `panelName` with an `"unknown"` fallback when the request lacks them.

We stick to the smallest scope, reuse the existing `QueryEditor`, do not ship a starter SQL.

## Goals / Non-Goals

**Goals:**
- Expose the Hydrolix datasource as a Grafana annotation source: appears in the dashboard "Annotations" picker, executes user SQL, renders markers.
- Support both instant annotations (`time` column only) and region annotations (`time` + `timeEnd`).
- Silently skip execution when the annotation's SQL is empty — surface zero markers, not a backend error.
- Prevent annotation refreshes from clobbering the cached panel-query state used by ad-hoc filter resolution. Achieved by retagging annotation requests with `app: 'annotation'` at `query()` entry — the existing `app === CoreApp.Dashboard` guard then naturally excludes them.

**Non-Goals:**
- Field-mapping UI — Grafana already provides field-mapping in the annotation editor; the plugin does not duplicate it.
- Custom annotation `QueryEditor`.
- Hiding the Query Type dropdown / Round picker / Table format selector when the existing editor is used in annotation mode — v1 leaves the full editor surface visible.
- Backend (Go) changes. Annotation queries flow through the same sqlds path as panel queries.
- New ad-hoc behavior specific to annotations (e.g., per-annotation filters separate from dashboard filters).
- Compatibility with Grafana < 10 (already out of scope at the plugin level). No `prepareAnnotation` migration hook.

## Decisions


### D1. No `prepareAnnotation` registration

`prepareAnnotation` is **not** registered. The hook exists in `AnnotationSupport` primarily to migrate pre-8.0 `rawQuery` string targets into the modern `target: HdxQuery` shape. Two facts make it unnecessary here:

1. The plugin supports Grafana ≥ 10 only. 10+ always stores the modern shape.
2. The Hydrolix datasource has never previously declared annotation support, so there is no historical Hydrolix annotation JSON of any vintage to migrate.

Without registration, Grafana passes the stored JSON through unchanged, then `prepareQuery` runs on it.

**Alternative considered**: register a no-op pass-through for symmetry / future-proofing. Rejected — `AnnotationSupport` hooks are optional; an empty hook adds a function call per annotation per refresh for zero behavioral gain. If a future shape change requires migration, add the hook then.

**Why migration concerns drop entirely vs. partial scope**: a migration hook that does nothing is dead code; a migration hook that "future-proofs" is speculation. Leave it out.

### D2. `prepareQuery` empty-SQL skip and source-tagging

Return `undefined` when `target?.rawSql` is empty/whitespace. Grafana's contract is that an `undefined` return from `prepareQuery` skips execution silently. Otherwise return a shallow copy of the target with `source: 'annotation'` set (see D5 for the type and the rationale for tagging at the query level rather than the request level).

```ts
const sql = anno.target?.rawSql?.trim();
if (!sql) return undefined;
return { ...anno.target, source: 'annotation' };
```

The copy is intentional — never mutate the stored target object, since Grafana may reuse the reference for the editor view and on subsequent reloads.

**Alternative considered**: substitute a fallback SQL (the `getDefaultQuery` template) when the user's `rawSql` is empty. Rejected — if a user clears the SQL while editing, they should not see a backend error *or* a phantom query firing against random data. `undefined` is Grafana's documented escape hatch and matches the user's intent (no annotation should appear).

**Alternative considered**: strip auxiliary fields (`panelId`, `format`, etc.) from `target` before returning. Rejected — nothing on `HdxQuery` needs to be removed for annotation execution; sqlds tolerates all of it, and stripping fields adds maintenance burden whenever `HdxQuery` grows.

**Alternative considered**: mutate the input target in place and return it (`anno.target.source = 'annotation'; return anno.target`). Rejected — mutating the input object handed to us by Grafana risks cross-talk if Grafana caches the reference. The shallow-copy cost is one allocation per annotation refresh; trivial.

### D3. `getDefaultQuery` seeds defaults but no starter SQL

Return a `Partial<HdxQuery>` with `{source: 'annotation'}`. Do **not** seed `rawSql` — the editor opens blank and `prepareQuery` (D2) returns `undefined` until the user writes SQL.


### D4. Retag annotation requests with `app === 'annotation'`

Grafana hands annotation queries to `datasource.query()` with `request.app === CoreApp.Dashboard` (the same value panel queries carry). To centralize the "is this an annotation?" distinction, the `query()` method retags annotation requests at entry to a dedicated app string `'annotation'`:

```ts
query(request: DataQueryRequest<HdxQuery>): Observable<DataQueryResponse> {
  if (isAnnotationRequest(request)) {
    request = { ...request, app: 'annotation' };
  }
  if (request.range !== ZERO_TIME_RANGE) {
    this.options = request;
  }
  if (request.app === CoreApp.Dashboard) {   // unchanged — naturally skips 'annotation'
    this.filters = request.filters;
  }
  ...
}

function isAnnotationRequest(request: DataQueryRequest<HdxQuery>): boolean {
  return request.targets.some(t => t.source === 'annotation');
}
```

Verified against `grafana/grafana@main` that annotation and panel queries arrive as separate `DataQueryRequest`s and are never mixed: `executeAnnotationQuery.ts:70-75` builds one request per annotation with a single `Anno` target, `PanelQueryRunner.ts:311` builds its own request from the panel's own targets only, and unified alerting bypasses `datasource.query()` entirely by POSTing to `/api/v1/eval`. The `.some()` is conservative — every target in a Grafana-issued request is either all annotation or all panel.

Detection reads the `source` field that `prepareQuery` (and `getDefaultQuery`) set on annotation targets (see D5). No refId inspection, no panelId inference, no heuristics.

`isAnnotationRequest` lives as a free function in `src/annotations.ts`, alongside the hooks that set `source: 'annotation'`. It is pure over the request shape (no `DataSource` state), so the reader and the writer of the `source` field sit in one file and the function is unit-testable without instantiating `DataSource`. `datasource.ts` imports it.

**Why retag the request as well as tagging the target**:
- The existing guard `request.app === CoreApp.Dashboard` (value `'dashboard'`) naturally evaluates to false once `app` is `'annotation'`. No new condition on the filter assignment; just a tag swap. Less code, less risk of forgetting the guard in a future code path.
- `app` becomes a self-describing discriminator at the request level — any future code branching on "is this an annotation request?" can read `request.app` without re-inspecting `targets`.
- The retag is a single spread copy (`{ ...request, app: 'annotation' }`) — the original `request` object handed to us by Grafana is not mutated.

**Why `'annotation'` (string) and not a new `CoreApp` enum value**: `DataQueryRequest.app` is typed as `CoreApp | string`. A custom string is the documented escape hatch; we don't need (and shouldn't pollute) the shared enum.

**Why detect via `target.source` rather than `request.panelId === undefined` or refId conventions**: see D5 for the full case. Summary: the source field is the only signal we own end-to-end. RefId conventions can drift, panelId absence collides with other dashboard-context-no-panel flows in some Grafana minors. Owning a query-level marker eliminates the entire class of detection bugs.

**Why not also retag `this.options`**: `this.options` caches `range` + `interval`, which annotation and panel queries share (same dashboard time window). Overwriting it from an annotation request is harmless. The existing `ZERO_TIME_RANGE` guard already excludes internal metadata calls. No change there.

**What downstream code needs to know**: `super.query(...)` is called with the retagged request. The sqlds backend pipeline does not key off `app` in any way that matters for execution; it forwards the targets to Go. The Go side reads `panelId` / `panelName` from the request and does not consume `app` or `target.source`. So the retag and the source field are purely frontend-side semantic tags.

### D5. `HdxQuery.source` discriminator field

Add an optional `source` field to `HdxQuery` in `src/types.ts`:

```ts
export type HdxQuerySource = 'annotation';

export interface HdxQuery extends DataQuery {
  // existing fields...
  source?: HdxQuerySource;
}
```

`prepareQuery` sets `source: 'annotation'` on every annotation target it returns. `getDefaultQuery` also includes `source: 'annotation'` in its returned partial — so brand-new annotations carry the marker from the very first edit, and the saved dashboard JSON reflects it. The two setters are belt-and-braces: `prepareQuery` is the canonical place, `getDefaultQuery` makes the marker self-documenting in raw dashboard exports.

**Why a query-level field rather than a request-level flag**: the SDK gives us no API to attach a request-level property — `DataQueryRequest` is constructed by Grafana, and `prepareQuery` only returns the `TQuery`. The query-level field is the only thing we control from inside the annotation pipeline.

**Why a string-literal type (`'annotation'`) rather than a boolean**:
- Self-describing in dashboard JSON exports (`"source": "annotation"` vs. `"isAnnotation": true`).
- Composable for future sources without renaming. If we ever need to mark queries that originate from a different non-panel path, widen the union: `type HdxQuerySource = 'annotation' | 'alert' | ...`. A boolean wouldn't survive that transition without a migration.

**Why optional (`source?:`) rather than required**:
- Backward compatibility with every existing `HdxQuery` instance (panel queries, stored variable queries, etc.). They don't carry `source` and don't need to — `t.source === 'annotation'` is false for `undefined`, which is exactly what we want for non-annotation targets.
- No on-disk migration for dashboards that pre-date this change.

**Alternative considered**: attach the marker to a side-channel (request meta, instance settings, `this.someFlag`). Rejected — none survive `prepareQuery → runRequest → query()` cleanly without inventing a fragile bridge.

**Alternative considered**: detect via `request.panelId === undefined && request.app === CoreApp.Dashboard`. Rejected even though plausible — collides with variable queries and ad-hoc filter value fetches in some Grafana minors (those route through other methods in this plugin today, but not by SDK contract; a future SDK change could send them through `query()`).

**Alternative considered**: detect via target `refId` convention (`refId === 'Anno'`). Rejected — refId is whatever the stored target carries. Imported dashboards or hand-edited JSON can have any refId; the marker drifts silently.

### D6. No new `QueryType.Annotation`

`format` on `HdxQuery` is read only by the frontend dropdown; the Go side ignores it. Annotations don't need a distinct execution path. Reuse `QueryType.Table` as the implicit format for annotation queries; users see the existing dropdown but the Grafana annotation editor's field-mapping UI is what controls how result columns become event fields.

**Rejected alternative**: add `QueryType.Annotation = 3` to drive distinct editor UI. Defer to a future change if/when we hide editor controls in annotation mode (currently a non-goal).

### D7. Three-layer test split (annotation-hooks unit, datasource retag unit, end-to-end Playwright)

Three independent test surfaces, each covering a distinct invariant:

- **Annotation-hooks unit (`src/annotations.test.ts`)**: pure-function tests for `prepareQuery` (happy path, empty SQL, whitespace SQL, absent target), `getDefaultQuery` (returned shape), and `isAnnotationRequest` (true when any target has `source === 'annotation'`, false otherwise — including the all-panel-targets and empty-targets cases).
- **Datasource retag unit (`src/datasource.test.ts` additions)**: (a) annotation-shaped requests are retagged so the request reaching `super.query()` carries `app === 'annotation'`; (b) `this.filters` is *not* updated for annotation requests; (c) `this.filters` *is* updated for panel requests (regression check on the existing path); (d) `this.options` *is* still updated for annotation requests with a real range; (e) the original request object handed to `query()` is not mutated. Mock `super.query()` so the backend is not exercised.
- **End-to-end Playwright (`tests/annotations.spec.ts`)**: against the docker-compose dev stack, drives a real Grafana dashboard through the annotation lifecycle — open Annotations editor → assert SQL editor opens blank (no starter template) → paste instant fixture query and map fields → assert markers → switch to region query and map fields → assert region markers → clear SQL → assert no toast, no markers → re-introduce an ad-hoc filter on a panel and assert the filter still applies (filter-cache preservation across the annotation refresh).

**Alternative considered**: skip the datasource-retag unit layer and rely on the e2e to catch filter-cache clobber. Rejected — the e2e is slow and depends on docker-compose; the retag is a small pure-logic change that deserves a fast, deterministic unit-level check. Catching the regression in <1s of jest beats catching it 30s into a Playwright run.

**Alternative considered**: skip the e2e entirely and rely on unit tests alone. Rejected — annotation rendering depends on Grafana's frame-to-event mapper, which the unit tests cannot exercise. Without the e2e, a column-convention break in a future Grafana minor would only surface in production.

## Risks / Trade-offs

- **[Filters clobber risk — D4]** → Mitigation: retag `request.app` to `'annotation'` at `query()` entry; existing `app === CoreApp.Dashboard` guard naturally excludes annotation requests from `this.filters` cache. Covered by unit test. Verified in the e2e suite by mixing an annotation refresh with an ad-hoc-filtered panel query.
- **[Stale dashboard JSON missing `source: 'annotation'`]** → If somehow an annotation target is persisted without `source` set (e.g., a future code path forgets to set it, or a hand-edited dashboard JSON drops it), `isAnnotationRequest` returns false and the request is treated like a panel — clobbering `this.filters`. Mitigation: `prepareQuery` always re-sets `source: 'annotation'` on every annotation refresh (D2), so the marker is restored on the next load regardless of stored state. Proving signal: the prepareQuery unit test (D7 row 1, "happy path" scenario) asserts the returned target carries `source: 'annotation'` even when the input target lacks it.
- **[Attribution metadata]** Annotation requests reach `pkg/plugin/driver.go:421-467` without `panelId` / `panelName`. → Mitigation: the existing `"unknown"` fallback handles this. We will inspect Go logs during e2e to confirm no warnings escalate; if they do, file a follow-up to set a synthetic `panelName: 'annotation:<refId>'` in `prepareQuery`. Not a blocker.
- **[Debounced interpolation without panel context]** `QueryEditor.tsx:154-177` calls `props.datasource.interpolateQuery(...)` with the panel's context. In annotation mode there is no panel. → Mitigation: the call already routes through `getInterpolatedQuery` which uses `this.options?.range` (still set per D4 — annotation requests update `this.options`) and `this.filters` (preserved from the last panel `query()`). Exercised in e2e by editing the annotation SQL and confirming the preview renders.

## Migration Plan

This is purely additive on the frontend. No data migration, no config migration. No `prepareAnnotation` is registered (see D1) — Grafana ≥ 10 baseline means there is no historical annotation JSON shape to migrate.

- **Forward**: ship `plugin.json` (capability flag) + `annotations.ts` (hooks + detector) + `datasource.ts` (constructor `this.annotations` assignment plus `query()` retag) together. On first load, existing dashboards see no change; new annotations against Hydrolix become possible.
- **Rollback**: revert all three files. Any annotations created against Hydrolix in the meantime would stop executing (the capability flag is gone), but the stored JSON remains intact and would resume working if the capability is restored. Low blast radius — no schema, no DB, no backend code.

## Open Questions

- Do we want to set a synthetic `panelName: 'annotation:<refId>'` in `prepareQuery` to make Go-side query attribution self-describing? Defer until we observe the e2e Go logs.
