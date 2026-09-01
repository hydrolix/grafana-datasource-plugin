# editor-interpolation-context Specification (delta)

## ADDED Requirements

### Requirement: Interpolation context is supplied by the caller

The interpolation entry points SHALL receive the time range, interval and
ad-hoc filters as an explicit parameter from their caller, and SHALL NOT read
request state cached on the datasource instance. This covers
`interpolateQuery` and `getInterpolatedQuery`. `QueryEditor` SHALL build that
parameter from its own props — the editor's currently selected range, and the
filters attached to the panel's data request.

#### Scenario: Editor supplies its own context

- **GIVEN** a panel editor showing the interpolated query
- **WHEN** the interpolation request is built
- **THEN** the posted `range` SHALL be the range the editor currently has
  selected
- **AND** the posted `filters` SHALL be the panel data request's ad-hoc
  filters

#### Scenario: A metadata query cannot influence the preview

- **GIVEN** a dashboard whose ad-hoc value dropdown has been opened, issuing
  a preload query on a capped trailing-24h window
- **WHEN** the interpolated query is shown for a panel on that dashboard
- **THEN** the posted `range` SHALL be the dashboard's selected range, not
  the preload's capped window

#### Scenario: Changing the time picker changes the preview

- **GIVEN** a panel editor showing the interpolated query
- **WHEN** the user changes the time range and no panel query has run since
- **THEN** the next interpolation request SHALL carry the newly selected
  range

### Requirement: Interpolation interval is derived from the supplied range

The interval sent with an interpolation request SHALL be computed from the
range in that same request, together with the panel's resolution when one is
available. It SHALL NOT be carried over from a previously completed query
request, so that the interval and the range bounds always describe the same
window.

#### Scenario: Interval follows the range it is sent with

- **GIVEN** an interpolation request for a 6-hour range
- **WHEN** the same editor issues a later request for a 90-day range
- **THEN** the interval sent with the second request SHALL be computed from
  the 90-day range

#### Scenario: Resolution falls back to a default

- **GIVEN** an interpolation request where no panel resolution is available
- **WHEN** the interval is derived
- **THEN** a documented default resolution SHALL be used
- **AND** the request SHALL still carry a valid interval

### Requirement: Interval is expressed in a unit the backend can parse

The interval SHALL be sent as a millisecond duration. It SHALL NOT be sent
using duration units the backend's parser rejects (`d`, `w`, `M`, `y`), for
any range width or resolution.

#### Scenario: A wide range still yields a parseable interval

- **GIVEN** a range of 5 years and a low panel resolution, for which
  Grafana's own interval formatting would produce a year-based unit
- **WHEN** the interpolation request is built
- **THEN** the interval SHALL be expressed in milliseconds
- **AND** the interpolation resource SHALL NOT reject the request

#### Scenario: Narrow ranges are unaffected

- **GIVEN** a range of 6 hours
- **WHEN** the interpolation request is built
- **THEN** the interval SHALL be expressed in milliseconds
- **AND** it SHALL equal the derived interval for that range

### Requirement: The preview renders without a preparatory query

Showing the interpolated query on a freshly-opened panel SHALL NOT require
the plugin to issue a query in order to populate datasource state. No
preparatory or skipped-execution query SHALL be issued as a precondition of
interpolation.

#### Scenario: Fresh panel interpolates directly

- **GIVEN** a freshly-opened panel editor on which no query has yet run
- **WHEN** the user shows the interpolated query
- **THEN** the interpolated SQL SHALL render
- **AND** no additional data query SHALL be issued to make it render

#### Scenario: Interpolation does not wait on a query round trip

- **GIVEN** a freshly-opened panel editor with SQL entered
- **WHEN** the user shows the interpolated query
- **THEN** the interpolation request SHALL be issued on the first attempt
  rather than after a re-render triggered by a preparatory query
