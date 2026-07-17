# hdx-sqlds-wrapper Specification

## Purpose
TBD - created by archiving change pin-sqlds-extension-revision. Update Purpose after archive.
## Requirements
### Requirement: Module pin to extension-points-only sqlds revision

The plugin SHALL pin `github.com/grafana/sqlds/v5` to the pseudo-version produced from commit `ef925e1` on the `hydrolix/sqlds` fork's `feat/sqlds-extension` branch. A `replace` directive in `go.mod` SHALL route the upstream module path to the hydrolix fork until C8 swaps to a released upstream tag.

#### Scenario: `go.mod` pins the extension-points revision

- **GIVEN** the plugin's `go.mod` after this change
- **WHEN** `go list -m github.com/grafana/sqlds/v5` is invoked
- **THEN** the output SHALL reference `v5.0.0-20260613103402-ef925e15e15e`
- **AND** the file SHALL contain a `replace github.com/grafana/sqlds/v5 => github.com/hydrolix/sqlds/v5 v5.0.0-20260613103402-ef925e15e15e` directive

### Requirement: `HdxSqlDatasource` wrapper embeds `*sqlds.SQLDatasource`

The plugin SHALL define `HdxSqlDatasource` in `pkg/plugin/hdx_sqlds.go` as a struct that embeds `*sqlds.SQLDatasource` and exposes the embedded pointer's promoted methods (`QueryData`, `CheckHealth`, `Dispose`, `GetDBFromQuery`, etc.) without delegation.

#### Scenario: Wrapper promotes upstream methods

- **GIVEN** an `*HdxSqlDatasource` value
- **WHEN** any method defined on `*sqlds.SQLDatasource` is invoked
- **THEN** the call SHALL succeed without a compile-time error or a runtime panic
- **AND** the receiver inside the upstream method SHALL be the embedded `*sqlds.SQLDatasource`

### Requirement: `NewHdxSqlDatasource` sets `EnableMultipleConnections` and returns the wrapper

The plugin SHALL define `NewHdxSqlDatasource(driver sqlds.Driver) *HdxSqlDatasource` that constructs the upstream `*sqlds.SQLDatasource` via `sqlds.NewDatasource(driver)`, sets `EnableMultipleConnections = true`, and returns the wrapper.

#### Scenario: Constructor wires substrate invariants

- **GIVEN** a non-nil `sqlds.Driver`
- **WHEN** `NewHdxSqlDatasource(driver)` is invoked
- **THEN** the returned wrapper SHALL have a non-nil embedded `*sqlds.SQLDatasource`
- **AND** `EnableMultipleConnections` SHALL be `true`
- **AND** the wrapper's `Interpolator` and `ConnectionCacheFactory` slots SHALL be unset (those wire in C5 and C3 respectively)

### Requirement: Wrapper-level `NewDatasource` returns the wrapper as the instance

The plugin SHALL override `NewDatasource(ctx, settings)` on `*HdxSqlDatasource` so the value returned to the Grafana SDK's instance manager is the wrapper, not the embedded `*sqlds.SQLDatasource`.

#### Scenario: Instance returned matches wrapper type

- **GIVEN** an `*HdxSqlDatasource` value
- **WHEN** `ds.NewDatasource(ctx, settings)` is invoked
- **THEN** the returned `instancemgmt.Instance` SHALL be a `*HdxSqlDatasource`
- **AND** a type-switch in the caller against `*HdxSqlDatasource` SHALL match

### Requirement: `Driver.Settings().ForwardHeaders` is pinned `false`

The plugin's `Hydrolix.Settings` method SHALL return `sqlds.DriverSettings{ForwardHeaders: false}` regardless of `pluginSettings.CredentialsType`. The previous behaviour (where `forwardOAuth` enabled `ForwardHeaders`) is incompatible with C4's OAuth-keying flow.

#### Scenario: All credentials types disable ForwardHeaders

- **GIVEN** a `DataSourceInstanceSettings` with any `credentialsType` value (`forwardOAuth`, `userAccount`, `serviceAccount`, empty)
- **WHEN** `Hydrolix.Settings(ctx, settings)` is invoked
- **THEN** the returned `sqlds.DriverSettings.ForwardHeaders` SHALL be `false`

### Requirement: `pkg/api/routes.go` consumes `*sqlds.SQLDatasource`

The plugin's HTTP route registration SHALL pass the embedded `*sqlds.SQLDatasource` (obtained via `ds.SQLDatasource`) into `pkg/api/routes.go`'s `Routes(...)` and `Interpolate(...)` functions, rather than a wrapper-specific type. This keeps `pkg/api` free of a `pkg/plugin` import.

#### Scenario: Routes registration uses upstream type

- **GIVEN** the plugin's `pkg/plugin/datasource.go`
- **WHEN** route registration is invoked
- **THEN** `api.Routes(...)` SHALL receive an `*sqlds.SQLDatasource` value
- **AND** the `pkg/api` package SHALL NOT import `github.com/hydrolix/plugin/pkg/plugin`

### Requirement: Stubbed `MacroCTEs` handler keeps the build green during migration

The `pkg/api/routes.go` `MacroCTEs` handler SHALL be temporarily stubbed during the C2-C7 coordinated set, returning an empty CTE list. C5 (`plugin-hdx-interpolator`) restores the full implementation when `GetMacroCTEs` and `CTE` move into the plugin.

#### Scenario: Stub returns empty CTE list

- **GIVEN** an HTTP request to `/macroCTE` with valid AST JSON
- **WHEN** the handler at C2's revision is invoked
- **THEN** the response SHALL be `{"error": false, "errorMessage": "", "data": []}`

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

