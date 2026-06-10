## Why

The Hydrolix datasource plugin cannot currently be used as an annotation source in Grafana dashboards. Users who store events in Hydrolix (deploys, incidents, audit records) have to either pipe them through a second datasource or forgo annotations entirely, which is a glaring gap given the plugin already ships full SQL query support, the `alerting` capability, and ad-hoc filter integration.

Adding annotation support is small in scope (the sqlds backend executes annotation queries identically to panel queries) and aligns the plugin with sibling SQL plugins (`grafana-clickhouse`, postgres, MSSQL).

## What Changes

- Declare the `annotations` capability in `src/plugin.json`.
- Register a minimal `AnnotationSupport` (`prepareQuery` + `getDefaultQuery`) by assigning it to `this.annotations` in the `DataSource` constructor (the `DataSourcePlugin` builder in this SDK version exposes no `setAnnotationSupport` chain; the `annotations` instance property is the supported entry point).
- Add `src/annotations.ts` to host those hooks.
- Extend `HdxQuery` in `src/types.ts` with a discriminator field that marks annotation-sourced targets.
- Reuse the existing `QueryEditor`. No custom annotation UI, no field-mapping UI.
- Differentiate annotation requests inside `src/datasource.ts` `query()` so they do not share the panel-query filter cache.
- Unit + Playwright e2e coverage.
- README — annotation column convention + worked example.

Not breaking. Existing dashboards keep working untouched.

## Capabilities

### New Capabilities
- `annotations`: Exposes the Hydrolix datasource as a Grafana annotation source. Owns the annotation-query lifecycle hooks (`prepareQuery`, `getDefaultQuery`) and the rules for how annotation requests interact with shared datasource state (ad-hoc filter cache, cached request options). Result-column-to-event-field binding is delegated to Grafana's built-in annotation field-mapping UI; the plugin does not impose a column convention.

### Modified Capabilities
<!-- None — no existing specs to amend (openspec/specs/ is currently empty). -->

## Impact

- **Frontend**: `src/plugin.json` (capability flag), new `src/annotations.ts` (hooks + detector), `src/datasource.ts` constructor (assign `this.annotations`) and `query()` retag (~lines 92-98). No `QueryType.Annotation`. `src/module.ts` unchanged.
- **Backend (Go)**: None expected. Annotation queries flow through the existing sqlds pipeline (`pkg/plugin/driver.go`); the `panelId` / `panelName` "unknown" fallback (`driver.go:421-467`) is exercised but not changed.
- **Tests**: New `src/annotations.test.ts`, additions to `src/datasource.test.ts` for the retag behavior, new `tests/annotations.spec.ts` playwright suite.
- **Docs**: README — new "Annotations" subsection.
- **Dependencies**: None added. `AnnotationSupport` is already exported by `@grafana/data` (pinned ≥ v10).
- **User-visible**: Hydrolix datasource appears in the dashboard "Annotations" datasource picker; the annotation editor opens with a blank SQL editor — the user supplies the query.
