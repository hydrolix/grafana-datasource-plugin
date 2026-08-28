# adhoc-value-preload Specification (delta)

## MODIFIED Requirements

### Requirement: Preload time window is bounded to the trailing 24h and rounded

The preload paths (`getTagValues` and `getTagKeysForMap`) SHALL cap the time
range passed to the metadata query: the effective `from` SHALL be
`max(range.from, range.to − 86400s)` and `to` SHALL be unchanged. The base
range SHALL come from the caller: `getTagValues` uses the time range Grafana
passes in its options argument, and `getTagKeysForMap` uses the time range
threaded from `getTagKeys`'s own options argument, falling back to the
template service's current range when Grafana supplies none. Neither path
SHALL read request state cached on the datasource instance. The
metadata query target SHALL carry `round = "5m"` so the plugin backend's
existing round mechanism snaps both endpoints to 5-minute boundaries before
macro expansion, making the interpolated SQL text stable within a rounding
window. The SQL template SHALL keep the plain `$__timeFilter(${timeColumn})`
filter (no cap arithmetic in SQL).

#### Scenario: Long dashboard range is capped

- **GIVEN** a dashboard time range spanning 90 days
- **WHEN** the value dropdown triggers a preload query
- **THEN** the range passed to the metadata query SHALL have
  `from = to − 86400s`
- **AND** `to` SHALL equal the dashboard range's end

#### Scenario: Short dashboard range is untouched

- **GIVEN** a dashboard time range spanning 6 hours
- **WHEN** the value dropdown triggers a preload query
- **THEN** the range passed to the metadata query SHALL equal the dashboard
  range

#### Scenario: Map-key discovery takes its window from the tag-keys options

- **GIVEN** a dashboard with a `Map`-typed ad-hoc filter key
- **WHEN** map-key discovery runs and Grafana supplies a time range in the
  tag-keys options argument
- **THEN** the range passed to the map-key query SHALL be that range, capped
  to the trailing 24h

#### Scenario: Map-key discovery falls back when no range is supplied

- **GIVEN** a Grafana version that omits the time range from the tag-keys
  options argument
- **WHEN** map-key discovery runs
- **THEN** the range passed to the map-key query SHALL be derived from the
  template service's current range, capped to the trailing 24h

#### Scenario: Metadata target requests 5-minute rounding

- **GIVEN** any preload or map-key discovery query
- **WHEN** the metadata query runner builds the query target
- **THEN** the target's `round` SHALL be `"5m"`

#### Scenario: Executed SQL is stable across opens within a rounding window

- **GIVEN** a dashboard with a `now`-relative time range
- **WHEN** the value dropdown is opened twice a few seconds apart within a
  single 5-minute rounding window
- **THEN** the executed (rounded, macro-expanded) SQL SHALL be identical
  across the two opens
- **AND** the interpolated time literals SHALL fall on 5-minute boundaries

#### Scenario: Rounding never trips the timerange enforcement

- **GIVEN** a dashboard range just under 24h whose endpoints round outward
- **WHEN** the preload query executes with
  `hdx_query_max_timerange_sec = 87000`
- **THEN** the query SHALL NOT be cancelled by the timerange setting (the
  87000 value carries 2×round-interval slack over the 86400 cap)
