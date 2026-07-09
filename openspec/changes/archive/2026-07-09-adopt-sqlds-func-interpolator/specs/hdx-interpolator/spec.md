# hdx-interpolator

## MODIFIED Requirements

### Requirement: `HdxInterpolator` implements `sqlds.Interpolator`

The plugin SHALL define `HdxInterpolator` in `pkg/plugin/interpolator.go` whose
`Interpolate` method satisfies the `sqlds.Interpolator` *func type*
(`func(ctx, *sqlutil.Query, json.RawMessage) (string, error)`). The signature
SHALL NOT carry a `*SQLDatasource` parameter — the datasource and metadata the
interpolator needs are captured via `md`/`macros` at construction. A
compile-time assertion (`var _ sqlds.Interpolator = (&HdxInterpolator{}).Interpolate`)
SHALL guard the signature. The implementation SHALL be safe for concurrent use
across queries.

#### Scenario: Func-type conformance

- **GIVEN** the `(&HdxInterpolator{}).Interpolate` method value
- **WHEN** assigned to a `sqlds.Interpolator` variable or to `SQLDatasource.Interpolator`
- **THEN** the assignment SHALL compile without error
- **AND** the signature SHALL omit the `*SQLDatasource` parameter

#### Scenario: Concurrent dispatch is race-free

- **GIVEN** a `*HdxInterpolator` shared across many goroutines
- **WHEN** each goroutine calls `Interpolate(...)` with independent queries
- **THEN** all calls SHALL complete without data races (under `go test -race`)

### Requirement: `Interpolate` routes runtime fields from `sqlutil.Query`

When invoked as the `sqlds.Interpolator` func, the implementation SHALL overlay
`query.RawSQL`, `query.TimeRange`, and `query.Interval` from the upstream
`*sqlutil.Query` onto the unmarshalled `models.HdxQuery`, so the sqlds-derived
runtime context takes precedence over anything carried in `rawJSON`.

#### Scenario: Runtime fields override JSON content

- **GIVEN** a `rawJSON` payload containing one `RawSQL` and a `*sqlutil.Query` containing a different `RawSQL`
- **WHEN** `Interpolate` runs
- **THEN** the macro dispatch SHALL operate on the `*sqlutil.Query`'s `RawSQL`, not the JSON's
