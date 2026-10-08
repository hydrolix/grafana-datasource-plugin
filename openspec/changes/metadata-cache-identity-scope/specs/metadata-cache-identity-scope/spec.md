# metadata-cache-identity-scope Specification (delta)

## ADDED Requirements

### Requirement: A schema-metadata cache entry belongs to the identity that fetched it

`MetadataProvider`'s primary-key and CTE-column caches SHALL key each entry on
the forwarded identity in addition to the table or CTE reference. Where the
caller forwards no identity, the key SHALL be the unscoped key, so that a mode
authenticating with one credential keeps one shared entry.

The identity SHALL be the forwarded token's subject claim, so that an entry
survives a token refresh and never spans two people. Where the subject cannot
be read, a digest of the token SHALL be used instead.

#### Scenario: A second user's lookup reaches the cluster

- **GIVEN** a datasource in a forwarding mode
- **AND** user A has looked up the primary key of table `T`
- **WHEN** user B looks up the primary key of table `T`
- **THEN** the lookup SHALL reach the cluster with user B's credential
- **AND** SHALL NOT be served from user A's cache entry

#### Scenario: A mode with no forwarded identity is unchanged

- **GIVEN** a datasource authenticating with a service account
- **AND** a cached primary key for table `T`
- **WHEN** another query looks up the primary key of table `T`
- **THEN** it SHALL be served from the cache

#### Scenario: A refreshed token keeps its cache entries

- **GIVEN** a cached entry for a user
- **WHEN** that user's forwarded token is refreshed
- **THEN** the entry SHALL still be served to them

### Requirement: Cache log lines do not carry the key

Log lines about cache hits and misses SHALL name the table or CTE reference
rather than the cache key, which carries an identity.

#### Scenario: A cache hit is logged by table

- **WHEN** a primary-key cache hit is logged
- **THEN** the line SHALL name the table
