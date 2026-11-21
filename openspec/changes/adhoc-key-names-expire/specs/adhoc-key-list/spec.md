# adhoc-key-list Specification (delta)

## ADDED Requirements

### Requirement: The ad hoc filter key list for a table expires one minute after it was read

The frontend metadata provider SHALL reuse a table's ad hoc filter key list for
less than one minute (`AD_HOC_KEYS_TTL_MS`), counted from the moment the DESCRIBE
that produced it was issued. A call after that SHALL issue a new DESCRIBE. Reads
within the minute SHALL NOT extend it. Entries SHALL be kept per table.

#### Scenario: A name published while the page is open appears after the TTL

- **GIVEN** the key list for table `T` was read at time `t0`, without `userAgent`
- **AND** `userAgent` has since been published on `T`
- **WHEN** the key list for `T` is requested at `t0 + AD_HOC_KEYS_TTL_MS - 1`
- **THEN** the cached list SHALL be returned and no DESCRIBE SHALL be issued
- **WHEN** the key list for `T` is requested at `t0 + AD_HOC_KEYS_TTL_MS`
- **THEN** a DESCRIBE SHALL be issued and the list SHALL contain `userAgent`

#### Scenario: Each table expires on its own schedule

- **GIVEN** the key list for `A` was read at `t0` and for `B` at `t0 + AD_HOC_KEYS_TTL_MS - 1`
- **WHEN** both are requested at `t0 + AD_HOC_KEYS_TTL_MS`
- **THEN** only `A` SHALL be read again
