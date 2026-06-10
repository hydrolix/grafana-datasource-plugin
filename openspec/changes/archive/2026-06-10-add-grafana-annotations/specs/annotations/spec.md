## ADDED Requirements

### Requirement: Hydrolix SHALL be selectable as a Grafana annotation source

The plugin SHALL declare the `annotations` capability so Grafana exposes the Hydrolix datasource in the dashboard "Annotations" datasource picker.

#### Scenario: Annotation picker lists Hydrolix

- GIVEN a Grafana dashboard with a configured Hydrolix datasource
- WHEN the user opens the dashboard "Annotations" datasource picker
- THEN "Hydrolix" SHALL appear as a selectable annotation datasource

### Requirement: The annotation editor SHALL open with an empty SQL field

When a user adds a new annotation against the Hydrolix datasource, the SQL editor SHALL open blank. The plugin MUST NOT seed a starter query.

#### Scenario: New annotation opens with blank SQL

- GIVEN a Hydrolix annotation that has just been added to a dashboard
- WHEN the annotation editor is opened for the first time
- THEN the SQL editor SHALL contain no text

### Requirement: An annotation query with empty SQL SHALL skip execution silently

`prepareQuery` SHALL return `undefined` whenever the stored target has no usable SQL — missing target, empty string, or whitespace-only `rawSql`. Grafana's contract is that an `undefined` return skips execution; no backend call SHALL be issued and no error toast SHALL surface.

#### Scenario: Empty rawSql skips execution

- GIVEN an annotation target with `rawSql === ''`
- WHEN `prepareQuery` is called
- THEN it SHALL return `undefined`

#### Scenario: Whitespace-only rawSql skips execution

- GIVEN an annotation target with `rawSql === '   \n  '`
- WHEN `prepareQuery` is called
- THEN it SHALL return `undefined`

#### Scenario: Missing target skips execution

- GIVEN an annotation object whose `target` is `undefined`
- WHEN `prepareQuery` is called
- THEN it SHALL return `undefined`

#### Scenario: Cleared SQL surfaces no toast and no markers

- GIVEN a dashboard with a Hydrolix annotation whose SQL the user has just cleared
- WHEN the dashboard refreshes
- THEN no annotation markers SHALL render
- AND no error toast SHALL appear

### Requirement: A non-empty annotation query SHALL execute and render markers

When the stored annotation target carries non-empty SQL, the query SHALL flow through the existing sqlds backend pipeline and any rows it returns SHALL be rendered by Grafana's standard annotation post-processor.

#### Scenario: Annotation query renders markers

- GIVEN a Hydrolix annotation whose SQL returns rows bound to time / text / tags via Grafana's annotation field-mapping UI
- WHEN the dashboard refreshes
- THEN annotation markers SHALL appear at the timestamps returned by the query

### Requirement: prepareQuery SHALL tag every returned target with `source: 'annotation'` without mutating the input

`prepareQuery` SHALL return a shallow copy of the input target with `source: 'annotation'` set. The input target reference MUST NOT be mutated, so Grafana's cached reference for the editor view is preserved.

#### Scenario: Returned target carries the source field

- GIVEN an annotation target with non-empty `rawSql` and no `source` field
- WHEN `prepareQuery` is called
- THEN the returned object SHALL have `source === 'annotation'`

#### Scenario: Input target is not mutated

- GIVEN an annotation target with non-empty `rawSql` and no `source` field
- WHEN `prepareQuery` is called
- THEN the input target SHALL still have no `source` field
- AND the returned object SHALL be a different reference from the input

### Requirement: getDefaultQuery SHALL return `source: 'annotation'` and no SQL

`getDefaultQuery` SHALL return a `Partial<HdxQuery>` with `source: 'annotation'` set. It MUST NOT include a `rawSql` key, so the editor opens blank (see "The annotation editor SHALL open with an empty SQL field").

#### Scenario: Default query carries source but no SQL

- WHEN `getDefaultQuery` is called
- THEN the returned object SHALL have `source === 'annotation'`
- AND the returned object MUST NOT have a `rawSql` key

### Requirement: Annotation requests entering `query()` SHALL be retagged with `app: 'annotation'`

When `DataSource.query()` receives a `DataQueryRequest` whose targets include any with `source === 'annotation'`, the method SHALL replace `request.app` with the string `'annotation'` via a spread copy before any instance state (`this.options`, `this.filters`) is read or updated. Detection SHALL use the `isAnnotationRequest` helper, which inspects only `target.source` — no refId, panelId, or other heuristic.

#### Scenario: Annotation request is retagged before super.query

- GIVEN a `DataQueryRequest` whose targets include at least one with `source: 'annotation'` and `request.app === CoreApp.Dashboard`
- WHEN `DataSource.query()` is invoked
- THEN the request reaching `super.query()` SHALL have `app === 'annotation'`

#### Scenario: Panel request is not retagged

- GIVEN a `DataQueryRequest` whose targets all lack `source: 'annotation'`
- WHEN `DataSource.query()` is invoked
- THEN the request reaching `super.query()` SHALL retain its original `app` value

### Requirement: Annotation refreshes SHALL preserve the cached panel-query filter state

`DataSource.query()` MUST NOT update `this.filters` from an annotation request. Subsequent metadata calls (`getTagKeysForMap`, `getInterpolatedQuery`) SHALL continue to see the last panel's ad-hoc filters.

#### Scenario: Annotation request does not overwrite this.filters

- GIVEN `this.filters` holds the most recent panel `request.filters`
- WHEN an annotation request enters `query()`
- THEN `this.filters` SHALL remain equal to the previous panel filters after `query()` returns

#### Scenario: Panel request still updates this.filters

- GIVEN a `DataQueryRequest` from a panel (no annotation targets, `app === CoreApp.Dashboard`)
- WHEN `query()` is invoked
- THEN `this.filters` SHALL be set to `request.filters`

#### Scenario: Ad-hoc filter survives an annotation refresh end-to-end

- GIVEN a dashboard with both an active Hydrolix annotation and a panel constrained by an ad-hoc filter
- WHEN the dashboard refreshes (firing both the annotation and the panel query)
- THEN the panel SHALL render with the ad-hoc filter still applied

### Requirement: Annotation requests SHALL update `this.options` when the range is real

`this.options` caches `range` + `interval`, which annotation and panel queries share. Annotation requests with a real time range SHALL update `this.options`; requests with `range === ZERO_TIME_RANGE` SHALL NOT, matching the existing internal-metadata guard.

#### Scenario: Annotation request with real range updates this.options

- GIVEN an annotation request whose `range` is not `ZERO_TIME_RANGE`
- WHEN `query()` is invoked
- THEN `this.options` SHALL be set to the (retagged) request

#### Scenario: ZERO_TIME_RANGE leaves this.options unchanged

- GIVEN any request whose `range === ZERO_TIME_RANGE`
- WHEN `query()` is invoked
- THEN `this.options` SHALL hold its previous value

### Requirement: The DataQueryRequest handed to `query()` SHALL NOT be mutated

Any retag inside `query()` MUST use a spread copy. The original request reference Grafana passes in SHALL remain unchanged after `query()` returns.

#### Scenario: Original request reference is unchanged

- GIVEN a `DataQueryRequest` reference `r` carrying annotation targets and `r.app === CoreApp.Dashboard`
- WHEN `DataSource.query(r)` is invoked
- THEN `r.app` SHALL still equal `CoreApp.Dashboard`
- AND `r.targets` SHALL hold its original array reference
