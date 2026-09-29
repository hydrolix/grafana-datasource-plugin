## ADDED Requirements

### Requirement: Ad-hoc preload degrades to a keyless form on tables with no primary key

`getTagValues` and `getTagKeysForMap` SHALL issue a keyless form of their
statement — the same statement with the `$__timeFilter(...)` conjunct
absent — when `metadataProvider.primaryKey()` resolves to the empty string
for the ad-hoc target table, instead of returning an empty list or emitting
a predicate against an empty column name. Only the empty string SHALL select
the keyless form. A response that carries errors SHALL reject and SHALL NOT
be memoized; a response with no row (the table is not in `system.tables`)
SHALL resolve to `undefined` and SHALL be memoized, as before this change.
Neither SHALL select the keyless form: for an unresolved primary key
`getTagValues` SHALL return an empty list without issuing a preload query,
and `getTagKeysForMap` SHALL issue the time-filtered statement with the
capped range and leave `$__timeFilter()` to the backend — both exactly as
before this change. `getTagKeys` SHALL resolve the primary key once per call
and pass it to every `getTagKeysForMap` it issues. The keyless form SHALL be
issued with no time
range, so `executeQuery` substitutes `ZERO_TIME_RANGE`, and SHALL NOT apply
the trailing-24h cap. Tables that resolve a non-empty primary key SHALL keep
the time-filtered statement unchanged.

#### Scenario: Value preload on a keyless table omits the time filter

- **GIVEN** an ad-hoc filter on column `status` of a table whose
  `system.tables.primary_key` is the empty string
- **WHEN** `getTagValues` builds the preload SQL
- **THEN** the SQL SHALL NOT contain `$__timeFilter`
- **AND** the SQL SHALL still aggregate with `topK(100)(status)`
- **AND** the SQL SHALL still contain `$__adHocFilter()`

#### Scenario: Keyless value preload populates the dropdown

- **GIVEN** a keyless table containing rows with `status` values `ok` and
  `error`
- **WHEN** the user opens the value dropdown for the `status` ad-hoc key
- **THEN** the returned suggestions SHALL include `ok` and `error`
- **AND** `getTagValues` SHALL NOT return an empty list

#### Scenario: Map-key discovery on a keyless table omits the time filter

- **GIVEN** an ad-hoc target table with no primary key and a Map column
  `attrs`
- **WHEN** `getTagKeysForMap` builds the map-key discovery SQL
- **THEN** the SQL SHALL NOT contain `$__timeFilter`
- **AND** the returned keys SHALL be the `attrs['<key>']` accessors for the
  discovered map keys

#### Scenario: Keyless preloads pass no time range

- **GIVEN** a keyless ad-hoc target table
- **WHEN** either preload path calls `executeQuery`
- **THEN** it SHALL pass no time range argument
- **AND** the resulting query target's range SHALL be `ZERO_TIME_RANGE`

#### Scenario: Unresolved primary key issues no preload

- **GIVEN** the primary-key lookup for the target table returns a response
  carrying `errors` (for example a missing grant on `system.tables` or a
  tripped metadata breaker)
- **WHEN** `getTagValues` runs for a key on that table
- **THEN** it SHALL return an empty list
- **AND** no value preload query SHALL be issued
- **AND** `metadataProvider.primaryKey()` SHALL NOT memoize a value for the
  table, so the next call re-issues the lookup

#### Scenario: Unresolved primary key keeps the backend-resolved map-key statement

- **GIVEN** the primary-key lookup for a target table with a Map column
  returns a response carrying `errors`
- **WHEN** `getTagKeys` runs map-key discovery for that column
- **THEN** the discovery SQL SHALL contain `$__timeFilter()`
- **AND** the range passed to `executeQuery` SHALL be the capped preload
  range
- **AND** the discovered `attrs['<key>']` accessors SHALL be returned

#### Scenario: No-row primary key response is memoized as unresolved

- **GIVEN** `system.tables` returns no row for the target table
- **WHEN** `metadataProvider.primaryKey()` is called twice for it
- **THEN** both calls SHALL resolve to `undefined`
- **AND** the lookup SHALL be issued once

#### Scenario: Primary key is resolved once for map-key discovery

- **GIVEN** an ad-hoc target table with three Map columns
- **WHEN** `getTagKeys` runs map-key discovery for all of them
- **THEN** `metadataProvider.primaryKey()` SHALL be invoked once
- **AND** every `getTagKeysForMap` statement SHALL be built from that
  single resolved value

#### Scenario: Keyed table is unaffected

- **GIVEN** an ad-hoc target table whose primary key is `timestamp`
- **WHEN** `getTagValues` builds the preload SQL
- **THEN** the SQL SHALL contain `$__timeFilter(timestamp)`
- **AND** the range passed to `executeQuery` SHALL be the capped preload
  range

### Requirement: Preload template slots are filled literally

The statement builders SHALL insert every slot value — column, table, time
filter, and ad-hoc condition — into the preload templates literally, so
that `$`-prefixed sequences in the inserted text (`$'`, `` $` ``, `$&`,
`$$`) are never interpreted as replacement patterns.

#### Scenario: Dollar sequence in the ad-hoc condition survives intact

- **GIVEN** an ad-hoc condition variable whose value is
  `currency = '$' AND amount > 0`
- **WHEN** `getColumnValuesStatement` builds the preload SQL
- **THEN** the SQL SHALL contain `AND currency = '$' AND amount > 0`
  verbatim
- **AND** the `SETTINGS` suffix SHALL appear exactly once, at the end

#### Scenario: Dollar sequence in a map-key column survives intact

- **GIVEN** an ad-hoc key `attrs['$ref']`
- **WHEN** `getColumnValuesStatement` builds the preload SQL
- **THEN** the aggregated expression SHALL be `attrs['$ref']` verbatim

## MODIFIED Requirements

### Requirement: Ad-hoc value preload uses bounded-memory topK aggregation

The ad-hoc filter value preload query (`AD_HOC_VALUE_QUERY`) SHALL compute
the candidate values with the `topK` aggregate function bounded to 100
values, and SHALL NOT use an unbounded `GROUP BY <column>` with a
post-aggregation `ORDER BY count` sort. The query SHALL remain a single
statement over the target table, filtered by `$__adHocFilter()` and the
optional ad-hoc condition variable. It SHALL additionally be filtered by
`$__timeFilter(<timeColumn>)` when the target table has a non-empty primary
key to serve as `<timeColumn>`; when the primary key is the empty string,
the time conjunct SHALL be absent and no substitute ordering or time
predicate SHALL be introduced in its place. Both forms SHALL be produced
from the same template, so the `topK` arity, the `AS value` alias, the
`$__adHocFilter()` conjunct, and the condition handling cannot drift
between them.

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

#### Scenario: Keyless table keeps the topK shape without a time conjunct

- **GIVEN** an ad-hoc filter on a table with no primary key
- **WHEN** `getTagValues` builds the preload SQL
- **THEN** the SQL SHALL aggregate with `topK(100)(<column>)`
- **AND** SHALL NOT contain `$__timeFilter`, `ORDER BY`, or `GROUP BY`

#### Scenario: Keyed statement is byte-identical to the pre-change template

- **GIVEN** an ad-hoc filter on column `status` of table `sample.log` whose
  primary key is `ts`, with no condition variable
- **WHEN** `getColumnValuesStatement` builds the preload SQL
- **THEN** the SQL SHALL equal
  `SELECT arrayJoin(topK(100)(status)) AS value FROM sample.log WHERE $__timeFilter(ts) AND $__adHocFilter()  SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000`

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
The keyless form SHALL carry the identical suffix: `hdx_query_max_timerange_sec`
derives the covered range from the WHERE-clause filter on the primary
column, so a statement with no such filter is not measured by it, and
varying the suffix would only create a second template to keep in sync.
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

#### Scenario: Keyless form carries the same guardrails

- **GIVEN** a preload or map-key discovery query on a table with no primary
  key
- **WHEN** the metadata query runner builds the query target
- **THEN** the target's `querySettings` SHALL include
  `hdx_query_max_execution_time = 10`
- **AND** the statement text SHALL carry
  `SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec = 87000`

#### Scenario: Dashboard panel queries are untouched

- **GIVEN** a dashboard panel query for the same datasource
- **WHEN** `prepareTarget` assembles its query settings
- **THEN** the guardrail settings SHALL NOT be injected (they may still be
  present only if the operator configured them at datasource level)

#### Scenario: Timeout with break honored yields partial values

- **GIVEN** a preload query whose full scan exceeds the execution-time cap
  on an engine that honors SQL-level `timeout_overflow_mode = 'break'`
- **WHEN** the engine hits the time cap
- **THEN** the response SHALL be a successful result computed over the rows
  read so far
- **AND** the dropdown SHALL be populated with those values

#### Scenario: Keyless full-table scan is bounded by the breaker

- **GIVEN** a keyless table large enough that the unfiltered preload cannot
  complete within the execution-time cap
- **WHEN** the value dropdown is opened
- **THEN** the query SHALL be broken off within the breaker budget
- **AND** the result SHALL be the partial aggregate or an empty list, never
  a multi-minute hang

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
range passed to the metadata query whenever they issue a time-filtered
statement: the effective `from` SHALL be `max(range.from, range.to −
86400s)` and `to` SHALL be unchanged. The metadata query target SHALL carry
`round = "5m"` so the plugin backend's existing round mechanism snaps both
endpoints to 5-minute boundaries before macro expansion, making the
interpolated SQL text stable within a rounding window. The time-filtered
form SHALL keep the plain `$__timeFilter(${timeColumn})` filter (no cap
arithmetic in SQL). When the statement is the keyless form, the preload
paths SHALL NOT resolve, cap, or pass a time range at all — there is no
window to bound and no time macro to expand.

#### Scenario: Long dashboard range is capped

- **GIVEN** a dashboard time range spanning 90 days and a keyed target table
- **WHEN** the value dropdown triggers a preload query
- **THEN** the range passed to the metadata query SHALL have
  `from = to − 86400s`
- **AND** `to` SHALL equal the dashboard range's end

#### Scenario: Short dashboard range is untouched

- **GIVEN** a dashboard time range spanning 6 hours and a keyed target table
- **WHEN** the value dropdown triggers a preload query
- **THEN** the range passed to the metadata query SHALL equal the dashboard
  range

#### Scenario: Metadata target requests 5-minute rounding

- **GIVEN** any preload or map-key discovery query
- **WHEN** the metadata query runner builds the query target
- **THEN** the target's `round` SHALL be `"5m"`

#### Scenario: Executed SQL is stable across opens within a rounding window

- **GIVEN** a dashboard with a `now`-relative time range and a keyed target
  table
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

#### Scenario: Keyless preload skips the cap entirely

- **GIVEN** a dashboard time range spanning 90 days and a target table with
  no primary key
- **WHEN** the value dropdown triggers a preload query
- **THEN** no capped range SHALL be computed
- **AND** the query target's range SHALL be `ZERO_TIME_RANGE`
