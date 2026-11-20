## MODIFIED Requirements

### Requirement: `Round` time-range adjustment runs before macro dispatch

The interpolator SHALL determine the effective round as the query's `Round` when it is non-empty, and otherwise as the data source's `defaultRound` (`jsonData.defaultRound`). When the effective round is a non-zero duration string, the interpolator SHALL round `query.TimeRange.From` and `query.TimeRange.To` to multiples of that duration before any macro dispatches. A query `Round` of `"0"` SHALL disable rounding even when a data source default is set. A non-empty query `Round` SHALL be used as sent: when it is not a duration of one second or more, the range SHALL be left unrounded and the data source default SHALL NOT apply. A data source `defaultRound` that is not a duration of one second or more SHALL be treated as unset.

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

#### Scenario: An invalid query round does not fall back to the default

- **GIVEN** a data source `defaultRound = "1m"` and a query with `Round = "not-a-duration"` and `TimeRange.From = 12:00:20 UTC`
- **WHEN** a macro that reads `query.TimeRange.From` runs
- **THEN** the macro SHALL observe `12:00:20 UTC`

#### Scenario: No round anywhere leaves the range alone

- **GIVEN** no data source `defaultRound` and a query with empty `Round`
- **WHEN** the interpolator runs
- **THEN** `query.TimeRange` SHALL be unchanged

#### Scenario: An unusable default is treated as unset

- **GIVEN** a data source `defaultRound` of `"0s"`, `"500ms"` or `"not-a-duration"` and a query with empty `Round`
- **WHEN** the interpolator runs
- **THEN** `query.TimeRange` SHALL be unchanged
