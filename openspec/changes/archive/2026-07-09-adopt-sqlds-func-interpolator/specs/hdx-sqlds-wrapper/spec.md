# hdx-sqlds-wrapper

## RENAMED Requirements

- FROM: `### Requirement: `Interpolate` handler uses the new sqlds Interpolator interface`
- TO: `### Requirement: `Interpolate` handler calls the sqlds Interpolator func`

## MODIFIED Requirements

### Requirement: `Interpolate` handler calls the sqlds Interpolator func

The `pkg/api/routes.go` `Interpolate` handler SHALL call
`ds.Interpolator(ctx, *sqlutil.Query, json.RawMessage)` using the func-typed
extension surface. When `ds.Interpolator` is nil, the handler SHALL return an
error rather than falling back to a default — `NewHdxSqlDatasource` always
installs the Hydrolix interpolator, so a nil field signals a construction bug.
The Hydrolix-specific query fields (`Filters`, `Round`, `TimeRange`,
`Interval`, `Headers`) SHALL travel via the `rawJSON` payload, marshalled from
`models.HdxQuery`.

#### Scenario: Handler reports an error when no interpolator is wired

- **GIVEN** an `*sqlds.SQLDatasource` with `ds.Interpolator == nil`
- **WHEN** the `/interpolate` handler is invoked
- **THEN** the response SHALL have `error: true`
- **AND** the `errorMessage` SHALL contain `"interpolator not configured"`

#### Scenario: Handler dispatches to the wired interpolator

- **GIVEN** an `*sqlds.SQLDatasource` whose `Interpolator` func returns a canned rewrite
- **WHEN** the `/interpolate` handler is invoked with a query body
- **THEN** the handler SHALL return that rewrite with `error: false`
- **AND** the interpolator SHALL receive the `rawJSON` marshalled from `models.HdxQuery` (carrying `Round` and `Filters`)
