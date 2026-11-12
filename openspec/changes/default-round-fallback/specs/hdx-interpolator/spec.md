## MODIFIED Requirements

### Requirement: `Round` time-range adjustment runs before macro dispatch

The interpolator SHALL determine the effective round as the query's `Round` when it is non-empty, and otherwise as the data source's `defaultRound` (`jsonData.defaultRound`). When the effective round is a non-zero duration string, the interpolator SHALL round `query.TimeRange.From` and `query.TimeRange.To` to multiples of that duration before any macro dispatches. A query `Round` of `"0"` SHALL disable rounding even when a data source default is set. `defaultRound` SHALL be read only when it is a JSON string; any other type SHALL be ignored without an error.

#### Scenario: Round applied before macro reads TimeRange

- **GIVEN** a query with `Round = "1m"` and `TimeRange.From = 12:00:20 UTC`
- **WHEN** a macro that reads `query.TimeRange.From` runs
- **THEN** the macro SHALL observe the rounded value (`12:00:00 UTC`)

#### Scenario: Invalid or sub-second `Round` is a no-op

- **GIVEN** `Round = "500ms"` or `Round = "not-a-duration"`
- **WHEN** the interpolator runs
- **THEN** `query.TimeRange` SHALL be unchanged

#### Scenario: Data source default applies when the query has no round

- **GIVEN** a data source `defaultRound = "1m"` and a query with empty `Round` and `TimeRange.From = 12:00:20 UTC`
- **WHEN** a macro that reads `query.TimeRange.From` runs
- **THEN** the macro SHALL observe `12:00:00 UTC`

#### Scenario: Query round wins over the data source default

- **GIVEN** a data source `defaultRound = "5m"` and a query with `Round = "1m"` and `TimeRange.From = 12:02:30 UTC`
- **WHEN** a macro that reads `query.TimeRange.From` runs
- **THEN** the macro SHALL observe `12:03:00 UTC`

#### Scenario: Query round 0 opts out of the default

- **GIVEN** a data source `defaultRound = "1m"` and a query with `Round = "0"` and `TimeRange.From = 12:00:30 UTC`
- **WHEN** a macro that reads `query.TimeRange.From` runs
- **THEN** the macro SHALL observe `12:00:30 UTC`

#### Scenario: No round anywhere leaves the range alone

- **GIVEN** no data source `defaultRound` and a query with empty `Round`
- **WHEN** the interpolator runs
- **THEN** `query.TimeRange` SHALL be unchanged

#### Scenario: A non-string default is ignored

- **GIVEN** `jsonData.defaultRound` is the JSON number `60`
- **WHEN** the data source settings are parsed
- **THEN** parsing SHALL succeed and `DefaultRound` SHALL be empty
