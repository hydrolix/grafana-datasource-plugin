## ADDED Requirements

### Requirement: Map-key discovery uses bounded-memory topKArray aggregation

Map-key discovery (`AD_HOC_MAP_KEY_QUERY`) SHALL compute a `Map` column's
candidate keys with `topKArray(100)(mapKeys(<column>))` and SHALL NOT select
every distinct key. Keys SHALL be offered in the order the query returns
them: approximate frequency order, most common first. The time conjunct,
`$__adHocFilter()` and guardrail settings SHALL stay as before.

#### Scenario: High-cardinality map produces a bounded query

- **GIVEN** a `Map` column `attributes` on the ad-hoc target table
- **WHEN** `getTagKeysForMap` builds the discovery SQL
- **THEN** the SQL SHALL aggregate with `topKArray(100)(mapKeys(attributes))`
- **AND** SHALL NOT contain `distinct`

#### Scenario: Keys arrive capped and most common first

- **GIVEN** a `Map` column with more than 100 distinct keys in the trailing
  window, where key `hot` occurs on every row and key `warm` on every other
  row
- **WHEN** the ad-hoc key dropdown is populated
- **THEN** the discovery query SHALL return exactly 100 keys
- **AND** `attributes['hot']` SHALL be offered before `attributes['warm']`

### Requirement: Preload statements do not alias their result column

Neither the map-key discovery nor the value preload statement SHALL alias its
result column, because a SELECT alias shadows a table column of the same name
inside `WHERE`. The response SHALL be read by column position.

#### Scenario: A column named key does not break discovery

- **GIVEN** an ad-hoc target table whose primary key column is named `key`
- **WHEN** map-key discovery runs with `$__timeFilter()` on that column
- **THEN** the query SHALL succeed and return the map's keys

#### Scenario: A column named value does not break value preload

- **GIVEN** an ad-hoc target table with a column named `value`
- **WHEN** the value preload runs with an ad-hoc filter on `value`
- **THEN** the preload SQL SHALL NOT contain `AS value`
