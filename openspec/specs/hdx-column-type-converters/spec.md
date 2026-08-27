# hdx-column-type-converters Specification

## Purpose

Owns the mapping from ClickHouse column types to Grafana data frame fields: which types the converter registry (`pkg/converters`) recognises, the frame field type each produces, how SQL `NULL` is represented, and the contracts a converter must honour — rejecting unexpected scan types rather than substituting a zero value, never aliasing the reused scan buffer, and matching type names deterministically regardless of registry iteration order. Rendering follows the *declared column type* rather than the value, matching ClickHouse's own `toString()`, so an IPv4-mapped address in an `IPv6` column keeps its `::ffff:` prefix.

Also owns which of those types are offered as ad-hoc filter keys (via the frontend supported-type list) and the requirement that a filter naming an eligible key produces valid SQL rather than being silently dropped to `1=1`.

## Requirements

### Requirement: UUID columns convert to string frame fields

The converter registry SHALL map a `UUID` column to a `data.FieldTypeString`
frame field holding the canonical textual form of the value.

#### Scenario: Non-nullable UUID column is selected

- **WHEN** a query selects a `UUID` column
- **THEN** the resulting frame field type is `FieldTypeString`
- **AND** each value is the column's canonical `8-4-4-4-12` hyphenated text

#### Scenario: UUID query no longer fails to scan

- **WHEN** a query selects a `UUID` column over either the native or the HTTP protocol
- **THEN** frame construction succeeds without a scan error

### Requirement: Nullable UUID columns preserve SQL NULL

The converter registry SHALL map a `Nullable(UUID)` column to a
`data.FieldTypeNullableString` frame field, and SHALL represent SQL `NULL` as a
nil `*string`.

#### Scenario: Populated nullable UUID value

- **WHEN** a `Nullable(UUID)` column holds a value
- **THEN** the frame field's concrete value at that row is the hyphenated text

#### Scenario: NULL nullable UUID value

- **WHEN** a `Nullable(UUID)` column holds SQL `NULL`
- **THEN** the frame value at that row is nil rather than an empty or placeholder string

### Requirement: IPv4 and IPv6 columns convert to string frame fields

The converter registry SHALL map `IPv4` and `IPv6` columns to
`data.FieldTypeString` frame fields, rendering each value according to its
declared column type in the same textual form ClickHouse's own `toString()`
produces.

#### Scenario: IPv4 column is selected

- **WHEN** a query selects an `IPv4` column holding `1.2.3.4`
- **THEN** the frame field type is `FieldTypeString` and the value is `1.2.3.4`

#### Scenario: IPv6 column is selected

- **WHEN** a query selects an `IPv6` column holding `2001:db8::1`
- **THEN** the frame field type is `FieldTypeString` and the value is `2001:db8::1`

#### Scenario: IPv4-mapped address in an IPv6 column

- **WHEN** an `IPv6` column holds a 16-byte IPv4-mapped address such as `::ffff:1.2.3.4`
- **THEN** the frame value keeps the IPv6 form `::ffff:1.2.3.4`, matching what ClickHouse's `toString()` returns for the same value
- **AND** it is NOT collapsed to the dotted-quad form `net.IP.String()` would produce

#### Scenario: IPv4-mapped bytes in an IPv4 column

- **WHEN** the same IPv4-mapped 16-byte value reaches the converter for an `IPv4` column
- **THEN** the frame value is the dotted-quad form `1.2.3.4`, because rendering follows the column type rather than the value

#### Scenario: Rendering is verified against the server rather than a fixture

- **WHEN** an integration test selects an `IPv6` column alongside `toString()` of the same column
- **THEN** the converted frame value equals the value ClickHouse rendered, over both the native and the HTTP protocol

#### Scenario: Value of an unrenderable length

- **WHEN** a value that is neither 4 nor 16 bytes reaches an IP converter
- **THEN** it returns a non-nil error and a nil value rather than a partial or blank rendering

### Requirement: Nullable IPv4 and IPv6 columns preserve SQL NULL

The converter registry SHALL map `Nullable(IPv4)` and `Nullable(IPv6)` columns to
`data.FieldTypeNullableString` frame fields, and SHALL represent SQL `NULL` as a
nil `*string`.

#### Scenario: Populated nullable IP value

- **WHEN** a `Nullable(IPv4)` or `Nullable(IPv6)` column holds an address
- **THEN** the frame field's concrete value at that row is that address in text form

#### Scenario: NULL nullable IP value

- **WHEN** a `Nullable(IPv4)` or `Nullable(IPv6)` column holds SQL `NULL`
- **THEN** the frame value at that row is nil rather than `<nil>` or an empty string

### Requirement: IP converters reject unexpected scan types

The IP converters SHALL return an error when handed a value that is not the
expected `*net.IP` (non-nullable) or `**net.IP` (nullable) scan target, rather
than substituting a zero value.

#### Scenario: Converter receives a mismatched type

- **WHEN** an IP converter's `ConverterFunc` is called with a value of any other type
- **THEN** it returns a non-nil error naming the expected and actual types
- **AND** it does not return a blank string

### Requirement: Converted IP values do not alias the reused scan buffer

The nullable IP converter SHALL return a freshly allocated `*string` per row, as
required by the `sqlutil.FrameConverter` aliasing contract.

#### Scenario: Multiple rows of a nullable IP column

- **WHEN** a query returns several rows from a `Nullable(IPv4)` column with distinct addresses
- **THEN** each frame row holds its own address rather than every row collapsing to the last value

### Requirement: UUID and IP type matching is deterministic

Converter selection SHALL match on exact ClickHouse type name for `UUID`,
`IPv4`, `IPv6` and their `Nullable(...)` forms, and MUST NOT be reachable by any
other registry entry's type regex.

#### Scenario: Registry entry lookup

- **WHEN** the registry is searched for `UUID`, `IPv4`, `IPv6`, `Nullable(UUID)`, `Nullable(IPv4)`, or `Nullable(IPv6)`
- **THEN** exactly one entry matches each name, independent of registry iteration order

### Requirement: UUID and IP columns are eligible ad-hoc filter keys

The frontend supported-type list SHALL include `UUID`, `IPv4`, and `IPv6` so that
columns of those types — and of their derived `Nullable(...)`, `Array(...)`, and
`Map(String, ...)` forms — are offered as ad-hoc filter keys.

#### Scenario: DESCRIBE output contains UUID and IP columns

- **WHEN** ad-hoc filter keys are derived from a table describe that includes `UUID`, `IPv4`, `IPv6`, and `Nullable(UUID)` columns
- **THEN** each of those columns appears in the returned key list with its ClickHouse type

#### Scenario: Derived wrapper type lists stay consistent

- **WHEN** the array and map type lists are derived from the supported-type list
- **THEN** they contain one entry per supported type and per nullable type, including the UUID and IP entries

### Requirement: Ad-hoc filters on UUID and IP columns produce valid SQL

An ad-hoc filter applied to a `UUID`, `IPv4`, or `IPv6` column SHALL generate a
condition that ClickHouse accepts and that matches the intended rows.

#### Scenario: Equality filter on a UUID column

- **WHEN** an ad-hoc filter applies `=` with a UUID literal to a `UUID` column
- **THEN** the generated condition executes without a ClickHouse type error and selects the matching rows

#### Scenario: Equality filter on an IP column

- **WHEN** an ad-hoc filter applies `=` with an address literal to an `IPv4` or `IPv6` column
- **THEN** the generated condition executes without a ClickHouse type error and selects the matching rows

#### Scenario: Equality filter using a dotted-quad literal

- **WHEN** an ad-hoc filter applies `=` with `1.2.3.4` to an `IPv6` column holding the IPv4-mapped address `::ffff:1.2.3.4`
- **THEN** the row matches, because ClickHouse parses the literal into an IPv6 value before comparing
- **AND** the padded `::ffff:1.2.3.4` form the panel displays matches under `=` as well, so a value copied out of a panel cell is valid ad-hoc filter input

#### Scenario: Negated equality filter on an IP column

- **WHEN** an ad-hoc filter applies `!=` with an address literal to an `IPv4` column
- **THEN** the generated condition selects every row except the matching one

#### Scenario: Multi-value filter on an IP column

- **WHEN** an ad-hoc filter applies the multi-value operator `=|` with two address literals to an `IPv4` column
- **THEN** the generated `IN (...)` condition selects the rows matching either literal

#### Scenario: Negated multi-value filter on an IP column

- **WHEN** an ad-hoc filter applies the negated multi-value operator `!=|` with two address literals to an `IPv4` column
- **THEN** the generated `NOT IN (...)` condition excludes the rows matching either literal and retains the rest

#### Scenario: NULL sentinel filter on a nullable UUID or IP column

- **WHEN** an ad-hoc filter applies `=` or `!=` with the `__null__` sentinel to a `Nullable(UUID)`, `Nullable(IPv4)`, or `Nullable(IPv6)` column
- **THEN** the generated condition is a plain `IS NULL` / `IS NOT NULL` test
- **AND** it does NOT additionally compare the column against the literal `'__null__'`, which no UUID or IP column can parse

#### Scenario: Regex and LIKE operators accept the displayed value

- **WHEN** an ad-hoc filter applies `=~` or `!~` to a `UUID`, `IPv4`, or `IPv6` column
- **THEN** the generated condition wraps the column in `toString(...)`
- **AND** because the plugin renders values in the same form `toString(...)` produces, the text shown in a panel cell matches under `=~` as well as under `=`

#### Scenario: A literal in a different textual form does not match under `=~`

- **WHEN** an ad-hoc filter applies `=~` with the dotted-quad literal `1.2.3.4` to an `IPv6` column rendered as `::ffff:1.2.3.4`
- **THEN** no rows match, because `=~` compares rendered text and the pattern carries no wildcards
- **AND** the same literal under `=` does match, because `=` parses the literal before comparing

#### Scenario: Negated text operator excludes the matching row

- **WHEN** an ad-hoc filter applies `!~` with the rendered form of a value to an `IPv6` column
- **THEN** the generated `toString(...) NOT LIKE` condition excludes the row holding that value and retains the rest
- **AND** the retained rows are identified positively, so a condition that lost its negation is distinguishable from one that applied it

### Requirement: Ad-hoc filters on UUID and IP columns are not silently dropped

The ad-hoc filter macro SHALL apply a filter whose key names a `UUID`, `IPv4`, or
`IPv6` column rather than skipping it and falling back to `1=1`.

#### Scenario: Filter key resolves against the table's DESCRIBE key set

- **WHEN** a dashboard applies an ad-hoc filter on a UUID or IP column and the panel query contains `$__adHocFilter()`
- **THEN** the filter reaches the backend attached to the query
- **AND** the emitted condition restricts the result set rather than resolving to `1=1`

