# hdx-adhoc-filter-macro-secure

## RENAMED Requirements

- FROM: `### Requirement: `MetadataProvider` caches PK + key lookups for one hour`
- TO: `### Requirement: `MetadataProvider` caches PK lookups for one hour and key lookups for one minute`

## MODIFIED Requirements

### Requirement: `MetadataProvider` caches PK lookups for one hour and key lookups for one minute

The plugin SHALL define `MetadataProvider` in `pkg/plugin/metadata.go` with two TTL-cached lookups: `(database, table) → primary_key` and `cte_name → {column_name: column_type}`. Both caches SHALL use `jellydator/ttlcache/v3`. The primary-key cache SHALL use a one-hour TTL. The column-type cache SHALL use a one-minute TTL (`adHocKeysTTL`) counted from the schema query that filled the entry, and a cache hit SHALL NOT extend an entry's expiry, so that a column name published on the table is applied by `$__adHocFilter` within a minute however often the map is read. Schema queries SHALL route through `metadataDS.QueryData(...)` so they participate in OAuth-keyed pooling (C4) and the TTL connection cache (C3).

#### Scenario: PK cache hit avoids schema query

- **GIVEN** a `MetadataProvider` whose `pkCache` already contains `("db", "tbl") → "id"`
- **WHEN** `GetPK(ctx, headers, "db", "tbl")` is called
- **THEN** the result SHALL be `("id", nil)`
- **AND** the underlying `metadataDS.QueryData` SHALL NOT be invoked

#### Scenario: PK cache miss invokes schema query and stores result

- **GIVEN** a `MetadataProvider` whose `pkCache` is empty and a fake `metadataDS` that returns a frame with `[id]` for the PK query
- **WHEN** `GetPK(ctx, headers, "db", "tbl")` is called twice in succession
- **THEN** `metadataDS.QueryData` SHALL be invoked exactly once
- **AND** both returns SHALL be `("id", nil)`

#### Scenario: PK not found is a typed error

- **GIVEN** a fake `metadataDS` that returns an empty frame for the PK query
- **WHEN** `QueryPK` is called
- **THEN** the returned error SHALL wrap `ErrPrimaryKeyNotFound`

#### Scenario: A key-map entry lives at most a minute

- **GIVEN** a `MetadataProvider` built by `NewMetadataProvider`
- **WHEN** `GetKeys` fills an entry
- **THEN** the entry's TTL SHALL be at most one minute

#### Scenario: Reading a key-map entry does not extend it

- **GIVEN** a cached column-type map for `hydro.logs`
- **WHEN** `GetKeys` is served from that entry
- **THEN** the entry's expiry SHALL be unchanged

#### Scenario: A filter on a newly published name applies on a busy dashboard

- **GIVEN** a column-type map for `hydro.logs` cached without `userAgent`
- **AND** `userAgent` is then published on `hydro.logs`
- **WHEN** `$__adHocFilter` runs with a filter on `userAgent` repeatedly, more often than the TTL
- **THEN** within the TTL after the map was filled, the result SHALL contain a condition on `userAgent` instead of `1=1`
