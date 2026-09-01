# annotations Specification (delta)

## MODIFIED Requirements

### Requirement: Annotation requests entering `query()` SHALL be retagged with `app: 'annotation'`

When `DataSource.query()` receives a `DataQueryRequest` whose targets include any with `source === 'annotation'`, the method SHALL replace `request.app` with the string `'annotation'` via a spread copy before the request is handed to `super.query()`, so annotation traffic reaches the backend attributed as annotation work rather than as a dashboard panel query. Detection SHALL use the `isAnnotationRequest` helper, which inspects only `target.source` — no refId, panelId, or other heuristic.

#### Scenario: Annotation request is retagged before super.query

- GIVEN a `DataQueryRequest` whose targets include at least one with `source: 'annotation'` and `request.app === CoreApp.Dashboard`
- WHEN `DataSource.query()` is invoked
- THEN the request reaching `super.query()` SHALL have `app === 'annotation'`

#### Scenario: Panel request is not retagged

- GIVEN a `DataQueryRequest` whose targets all lack `source: 'annotation'`
- WHEN `DataSource.query()` is invoked
- THEN the request reaching `super.query()` SHALL retain its original `app` value

## REMOVED Requirements

### Requirement: Annotation refreshes SHALL preserve the cached panel-query filter state

**Reason**: `DataSource` no longer holds `this.filters` — ad-hoc filters travel to their readers as explicit caller-supplied arguments (see the `editor-interpolation-context` capability), so there is no shared datasource state an annotation refresh could overwrite. The guarantee this requirement protected now holds by construction; the end-to-end behavior (an ad-hoc filter surviving an annotation refresh) is carried by the panel query pipeline itself, which sources filters from each request.

### Requirement: Annotation requests SHALL update `this.options` when the range is real

**Reason**: `this.options` was removed together with its `ZERO_TIME_RANGE` guard. Range and interval are no longer shared between annotation and panel queries through instance state; interpolation receives them as an explicit `InterpolationContext` parameter, and the ad-hoc preload paths take them from Grafana's options argument.
