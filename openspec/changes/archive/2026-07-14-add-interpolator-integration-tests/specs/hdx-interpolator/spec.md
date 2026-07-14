## ADDED Requirements

### Requirement: Interpolation pipeline is covered by integration tests

The plugin SHALL have Go integration tests that drive the real `HdxInterpolator` over the real `Macros` registry with a stubbed metadata layer (pre-seeded caches), feeding raw SQL through interpolation and asserting the rewritten SQL. The `/interpolate` HTTP route SHALL additionally be covered by a test that wires the real `HdxInterpolator` (not a stub) behind the handler. These tests codify existing behavior at the integration seam; they introduce no behavioral change.

#### Scenario: Combined ad-hoc filter and time macro in one query

- **GIVEN** a real interpolator over the `Macros` registry with the CTE schema and primary key pre-seeded
- **WHEN** raw SQL containing both `$__adHocFilter()` and a time macro (e.g. `$__timeFilter(ts)`) is interpolated with one filter and a fixed time range
- **THEN** the rewritten SQL SHALL contain the filter condition and the time-bounded comparison for the resolved column

#### Scenario: Ad-hoc filter over a WITH-CTE FROM

- **GIVEN** a real interpolator with the resolved CTE-subquery schema pre-seeded
- **WHEN** `WITH x AS (SELECT … ) SELECT … FROM x WHERE $__adHocFilter()` is interpolated with a filter on a CTE column
- **THEN** the rewritten SQL SHALL contain the filter condition built from that column

#### Scenario: Time macro resolves the primary key from cache

- **GIVEN** a real interpolator whose `pkCache` is pre-seeded for the FROM table
- **WHEN** a time macro without an explicit column argument is interpolated
- **THEN** the rewritten SQL SHALL reference the cached primary-key column

#### Scenario: Unknown macro is left in place

- **WHEN** raw SQL containing a macro not in the registry is interpolated
- **THEN** the macro text SHALL remain unchanged in the output

#### Scenario: Escaped macro strips one dollar

- **WHEN** raw SQL containing `$$__<macro>` is interpolated
- **THEN** the output SHALL contain `$__<macro>` and the macro SHALL NOT be dispatched

#### Scenario: `/interpolate` route over the real interpolator

- **GIVEN** the `/interpolate` handler wired to a real `HdxInterpolator` with a stubbed metadata layer
- **WHEN** a macro-bearing query is POSTed
- **THEN** the response SHALL contain the interpolated SQL produced by the real macro pipeline

#### Scenario: Ad-hoc filter injection is neutralized end to end

- **GIVEN** a real interpolator (and the `/interpolate` route wired to it) with the FROM schema pre-seeded
- **WHEN** an ad-hoc filter carrying an injected operator is interpolated
- **THEN** the pipeline SHALL return an error rather than SQL that carries the injection
- **AND** WHEN an ad-hoc filter carries a value or Map subscript crafted to break out of its literal, the rewritten SQL SHALL keep it wholly inside a quoted/backtick-quoted context with the quotes escaped
