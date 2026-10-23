## MODIFIED Requirements

### Requirement: `MetadataProvider` caches PK + key lookups for one hour

The plugin SHALL define `MetadataProvider` in `pkg/plugin/metadata.go` with two TTL-cached lookups: `(database, table) → primary_key` and `cte_name → {column_name: column_type}`. Both caches SHALL use `jellydator/ttlcache/v3` with a one-hour TTL. Schema queries SHALL route through `metadataDS.QueryData(...)` so they participate in OAuth-keyed pooling (C4) and the TTL connection cache (C3).

The primary-key lookup SHALL treat the empty string as a legitimate result. An empty result frame — the table is absent from `system.tables` — SHALL yield `ErrPrimaryKeyNotFound`. A result frame whose `primary_key` cell is the empty string — the table exists but declares no primary key — SHALL yield `("", nil)`, and `GetPK` SHALL store that result in `pkCache` exactly as it stores any non-empty key, so a keyless table is looked up once per TTL. `GetPK` SHALL NOT store anything when the lookup returns an error, so a transient failure is retried on the next call and can never be mistaken for a keyless table. Deciding whether `""` is usable as a column is the consumer's responsibility, not the lookup's.

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

#### Scenario: Empty primary_key cell is a value, not an error

- **GIVEN** a fake `metadataDS` that returns a frame whose single cell is the empty string for the PK query
- **WHEN** `QueryPK` is called
- **THEN** the result SHALL be `("", nil)`
- **AND** the returned error SHALL NOT wrap `ErrPrimaryKeyNotFound`

#### Scenario: Empty primary key is stored like any other result

- **GIVEN** a fake `metadataDS` that returns a frame whose single cell is the empty string for the PK query
- **WHEN** `GetPK(ctx, headers, "db", "keyless")` is called twice in succession
- **THEN** `metadataDS.QueryData` SHALL be invoked exactly once
- **AND** both returns SHALL be `("", nil)`

#### Scenario: A failed lookup is not stored

- **GIVEN** a fake `metadataDS` that returns a generic `backend.DownstreamError` for the PK query
- **WHEN** `GetPK(ctx, headers, "db", "tbl")` is called twice in succession
- **THEN** `metadataDS.QueryData` SHALL be invoked twice
- **AND** both returns SHALL carry that error
- **AND** `errors.Is(err, ErrPrimaryKeyEmpty)` SHALL be false for both
