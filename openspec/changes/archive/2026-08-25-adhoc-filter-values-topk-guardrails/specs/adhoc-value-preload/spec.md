# adhoc-value-preload Specification (delta)

## ADDED Requirements

### Requirement: Ad-hoc value preload uses bounded-memory topK aggregation

The ad-hoc filter value preload query (`AD_HOC_VALUE_QUERY`) SHALL compute
the candidate values with the `topK` aggregate function bounded to 100
values, and SHALL NOT use an unbounded `GROUP BY <column>` with a
post-aggregation `ORDER BY count` sort. The query SHALL remain a single
statement over the target table, filtered by `$__timeFilter(<timeColumn>)`,
`$__adHocFilter()`, and the optional ad-hoc condition variable, exactly as
before.

#### Scenario: High-cardinality column produces a bounded query

- **GIVEN** an ad-hoc filter on column `clientIP` of a raw table with more
  distinct values than the dropdown limit
- **WHEN** `getTagValues` builds the preload SQL
- **THEN** the SQL SHALL aggregate with `topK(100)(clientIP)`
- **AND** SHALL NOT contain `GROUP BY clientIP` or `ORDER BY count`

#### Scenario: Values arrive in approximate popularity order

- **GIVEN** a table where value `A` occurs in the vast majority of rows in
  the dashboard range and value `Z` occurs once
- **WHEN** the value dropdown is populated
- **THEN** `A` SHALL be present in the returned values
- **AND** the returned list SHALL contain at most 100 values

#### Scenario: Array columns keep arrayJoin expansion

- **GIVEN** an ad-hoc filter key whose column type is an Array type
- **WHEN** `getTagValues` builds the preload SQL
- **THEN** the aggregated expression SHALL be the `arrayJoin(<column>)`
  expansion of the column, as today

#### Scenario: Map-key columns keep map access expression

- **GIVEN** an ad-hoc filter key of the form `attributes['env']`
- **WHEN** `getTagValues` builds the preload SQL
- **THEN** the aggregated expression SHALL be `attributes['env']`

### Requirement: Synthetic empty and null entries survive the topK shape

The preload result handling SHALL keep offering the synthetic `__empty__`
entry when the empty string is among the returned values. The synthetic
`__null__` entry SHALL be offered based on the column's type from the
already-fetched `DESCRIBE` metadata: for plain columns, appended when the
type is Nullable; for map-access keys (`col['key']`), appended when the base
map column's value type is Nullable (a `Map(String, Nullable(…))` variant);
omitted otherwise — since aggregate functions skip NULLs and the value list
therefore carries no NULL signal. The preload response SHALL remain a single
value column (no companion null-count column).

#### Scenario: Empty string maps to synthetic empty

- **GIVEN** the preload query returns `''` among the top values
- **WHEN** `getTagValues` transforms the response
- **THEN** the dropdown values SHALL include `__empty__`
- **AND** SHALL NOT include a raw empty-string entry

#### Scenario: Nullable column type gates in the synthetic null

- **GIVEN** an ad-hoc filter key whose `DESCRIBE` type is Nullable
- **WHEN** `getTagValues` transforms the response
- **THEN** the dropdown values SHALL include `__null__`
- **AND** this SHALL hold regardless of whether any scanned row was NULL

#### Scenario: Non-Nullable column has no synthetic null

- **GIVEN** an ad-hoc filter key whose `DESCRIBE` type is not Nullable
- **WHEN** `getTagValues` transforms the response
- **THEN** the dropdown values SHALL NOT include `__null__`

#### Scenario: Nullable-valued map key gates in the synthetic null

- **GIVEN** an ad-hoc filter key `attrs['env']` whose base column type is
  `Map(String, Nullable(String))`
- **WHEN** `getTagValues` transforms the response
- **THEN** the dropdown values SHALL include `__null__` (a missing key on a
  Nullable-valued map reads as NULL)

#### Scenario: Non-nullable map key has no synthetic null

- **GIVEN** an ad-hoc filter key `attrs['env']` whose base column type is
  `Map(String, String)`
- **WHEN** `getTagValues` transforms the response
- **THEN** the dropdown values SHALL NOT include `__null__`

### Requirement: Metadata queries carry guardrail query settings

Every query issued through the metadata provider's query runner SHALL carry
the Hydrolix-native circuit breaker `hdx_query_max_execution_time = 10` in
its `querySettings` — this covers ad-hoc value preload, map-key discovery,
and schema/table/column/primary-key/autocomplete lookups. The ad-hoc value
preload and map-key SQL templates SHALL additionally carry
`SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec =
87000` in the statement text (`timeout_overflow_mode` has no Hydrolix
mirror, so it cannot travel on the driver settings channel;
`hdx_query_max_timerange_sec` enforces the bounded time window server-side).
Dashboard, Explore, and alerting queries SHALL NOT receive these injected
settings.

#### Scenario: Value preload target carries guardrails

- **GIVEN** the value dropdown triggers a preload query
- **WHEN** the metadata query runner builds the query target
- **THEN** the target's `querySettings` SHALL include
  `hdx_query_max_execution_time = 10`
- **AND** the statement text SHALL carry
  `SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000`
- **AND** the target's `querySettings` SHALL NOT include
  `timeout_overflow_mode` (driver channel would reject it)

#### Scenario: Map-key discovery carries guardrails

- **GIVEN** `getTagKeys` runs the map-key discovery query for a Map column
- **WHEN** the metadata query runner builds the query target
- **THEN** the target's `querySettings` SHALL include the same breaker
  setting
- **AND** the map-key statement text SHALL carry the same `SETTINGS` suffix
  as the value-preload template

#### Scenario: Dashboard panel queries are untouched

- **GIVEN** a dashboard panel query for the same datasource
- **WHEN** `prepareTarget` assembles its query settings
- **THEN** the guardrail settings SHALL NOT be injected (they may still be
  present only if the operator configured them at datasource level)

#### Scenario: Timeout with break honored yields partial values

- **GIVEN** a preload query whose full-range scan exceeds the execution-time
  cap on an engine that honors SQL-level `timeout_overflow_mode = 'break'`
- **WHEN** the engine hits the time cap
- **THEN** the response SHALL be a successful result computed over the rows
  read so far
- **AND** the dropdown SHALL be populated with those values

#### Scenario: Empty partial result is not an error

- **GIVEN** the cap is so tight that the broken-off scan produced no
  aggregate output (observed on a real cluster with a 1s cap over 90 days)
- **WHEN** `getTagValues` transforms the empty successful response
- **THEN** it SHALL return an empty suggestion list
- **AND** SHALL NOT surface an error to the user (manual value entry
  remains available)

#### Scenario: Slow preload under the cap still populates the dropdown

- **GIVEN** a value source whose preload scan takes just under the
  execution-time cap (≈9.99s against the 10s breaker)
- **WHEN** the user opens the value dropdown
- **THEN** the dropdown SHALL populate with the returned values
- **AND** no client-side layer SHALL abort or error the preload before the
  breaker budget elapses

#### Scenario: Hydrolix breaker bounds the worst case

- **GIVEN** a preload query on an engine that does not honor
  `timeout_overflow_mode` and whose scan exceeds
  `hdx_query_max_execution_time`
- **WHEN** the Hydrolix query head cancels the query
- **THEN** the preload SHALL fail within the breaker budget with the
  cluster's timeout error (no multi-minute hang)
- **AND** the user SHALL still be able to type a filter value manually

### Requirement: Preload time window is bounded to the trailing 24h and rounded

The preload paths (`getTagValues` and `getTagKeysForMap`) SHALL cap the time
range passed to the metadata query: the effective `from` SHALL be
`max(range.from, range.to − 86400s)` and `to` SHALL be unchanged. The
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

### Requirement: Metadata breaker takes the minimum of default and datasource-level values

The metadata query runner SHALL always inject the execution-time breaker
into metadata targets, with a value equal to the minimum of the default (10)
and any positive numeric datasource-level value configured in
`jsonData.querySettings` under `hdx_query_max_execution_time` or its
Hydrolix alias `max_execution_time` (same underlying setting).
Datasource-level values that are missing, non-numeric, or `≤ 0` SHALL be
ignored (`0` means unlimited on Hydrolix and MUST NOT be adopted). The
SQL-level `timeout_overflow_mode` suffix is part of the statement template
and SHALL NOT be affected by datasource-level settings.

#### Scenario: A larger datasource-level timeout cannot loosen the breaker

- **GIVEN** datasource-level `querySettings` contain
  `hdx_query_max_execution_time = 60`
- **WHEN** the metadata query runner builds a query target
- **THEN** the target's `querySettings` SHALL contain the injected breaker
  with value `10`
- **AND** the effective merged value for the metadata query SHALL be `10`
  (target-level wins the merge)

#### Scenario: A smaller datasource-level timeout tightens the breaker

- **GIVEN** datasource-level `querySettings` contain the alias
  `max_execution_time = 5`
- **WHEN** the metadata query runner builds a query target
- **THEN** the injected breaker value SHALL be `5`

#### Scenario: Unlimited is never adopted

- **GIVEN** datasource-level `querySettings` contain
  `hdx_query_max_execution_time = 0`
- **WHEN** the metadata query runner builds a query target
- **THEN** the injected breaker value SHALL be `10`

#### Scenario: Non-numeric datasource-level values are ignored

- **GIVEN** datasource-level `querySettings` contain
  `hdx_query_max_execution_time = '$timeout'`
- **WHEN** the metadata query runner builds a query target
- **THEN** the injected breaker value SHALL be `10`

#### Scenario: Unrelated datasource-level settings do not affect the breaker

- **GIVEN** datasource-level `querySettings` contain only
  `hdx_query_admin_comment = 'x'`
- **WHEN** the metadata query runner builds a query target
- **THEN** the injected breaker value SHALL be `10`
