# Tasks

## 1. Types

- [x] 1.1 In `src/types.ts`, export `HdxQuerySource` (string-literal union, initial value `'annotation'`) and add `source?: HdxQuerySource` to `HdxQuery`.

## 2. Annotation hooks

- [x] 2.1 Create `src/annotations.ts` exporting three pure helpers:
  - `prepareQuery(anno)` — returns `undefined` when `anno.target?.rawSql?.trim()` is falsy; otherwise returns a shallow copy `{ ...anno.target, source: 'annotation' }`. Never mutates the input target.
  - `getDefaultQuery()` — returns `{ source: 'annotation' }` (no `rawSql`).
  - `isAnnotationRequest(request)` — returns `request.targets.some(t => t.source === 'annotation')`.

## 3. DataSource retag

- [x] 3.1 In `src/datasource.ts`, import `isAnnotationRequest` from `./annotations`. At the top of `query()`, before the `this.options` and `this.filters` assignments, retag annotation requests with a spread copy: `if (isAnnotationRequest(request)) request = { ...request, app: 'annotation' };`. Do not introduce new branching on the filter-cache assignment — the existing `app === CoreApp.Dashboard` guard naturally excludes the retagged value.

## 4. Plugin registration

- [x] 4.1 Add `"annotations": true` to `src/plugin.json`.
- [x] 4.2 In the `DataSource` constructor (`src/datasource.ts`), import `prepareQuery` and `getDefaultQuery` from `./annotations` and assign `this.annotations = { prepareQuery, getDefaultQuery }`. This replaces the worktree placeholder `this.annotations = {}`. The `DataSourcePlugin` builder in `@grafana/data@12.x` has no `setAnnotationSupport` chain — the `annotations` instance property is the supported registration point. `src/module.ts` is unchanged.

## 5. Frontend unit tests

- [x] 5.1 Create `src/annotations.test.ts`:
  - `prepareQuery` returns `undefined` for empty-string, whitespace-only, and missing-target inputs.
  - `prepareQuery` happy path returns a shallow copy with `source: 'annotation'` and leaves the input target reference unmodified.
  - `getDefaultQuery` returns `{ source: 'annotation' }` and the returned object has no `rawSql` key.
  - `isAnnotationRequest` returns `true` when any target carries `source === 'annotation'`, and `false` for all-panel-targets and empty-targets requests.
- [x] 5.2 Extend `src/datasource.test.ts`:
  - Annotation-shaped requests passed to `query()` reach a mocked `super.query()` with `app === 'annotation'`.
  - `this.filters` is not updated by an annotation request, while a prior panel-set value is preserved.
  - A subsequent panel request still updates `this.filters` (regression check on the existing path).
  - `this.options` is updated for an annotation request with a real range, and unchanged for `ZERO_TIME_RANGE`.
  - The original `DataQueryRequest` reference handed to `query()` is not mutated (`app` and `targets` keep their original values/references).

## 6. E2E tests

- [x] 6.1 Add `tests/annotations.spec.ts`, driven through the docker-compose `playwright` service (per the `grafana-plugin-e2e` skill). Seed annotations via the dashboard JSON model (extended `DashboardBuilder.addAnnotation`) so the test is robust across Grafana 10–13 — the annotation field-mapping UI is a per-version maintenance sink, the JSON model is stable. SQL uses hardcoded SELECTs (e.g., `SELECT toDateTime(1744286400) AS time, ...`) — no ClickHouse fixture required. Three tests, each asserting at three layers (request → response → chrome):
  - Instant annotation: (1) `/api/ds/query` POST body has `queries[0].source === 'annotation'` and SQL contains `AS time` (proves `prepareQuery` ran and `query()` retag fired); (2) the paired response carries a frame whose schema has a `time` field (no `timeEnd`); (3) the annotation enable toggle for this annotation is visible and checked in the dashboard chrome.
  - Region annotation: (1) request as above, SQL also contains `AS timeEnd`; (2) response frame schema has BOTH `time` and `timeEnd` (the shape Grafana feeds into its region-overlay layer); (3) annotation enable toggle visible and checked.
  - Blank SQL annotation: no request with `source === 'annotation'` is sent (proves `prepareQuery` returns `undefined` for empty SQL).
  - Chrome locator: Grafana 10.x renders the annotation enable toggle as `role="checkbox"`; Grafana 11+ (Scenes layout) renders it as `role="switch"`. The test matches either via `getByRole("switch", {name}).or(getByRole("checkbox", {name}))` — Grafana's on-panel region marker is drawn directly on uPlot's canvas with no stable test-id, so the chrome toggle is the highest-fidelity DOM signal that's stable across all five supported versions.

## 7. Docs

- [x] 7.1 Add an "Annotations" subsection to the README: a one-paragraph capability summary, a worked example SQL query (instant + region variants), and a pointer to Grafana's annotation field-mapping UI for binding result columns to event fields. No "column convention" language — the plugin imposes none.
