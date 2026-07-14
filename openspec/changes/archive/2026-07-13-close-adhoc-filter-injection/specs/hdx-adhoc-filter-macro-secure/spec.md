## ADDED Requirements

### Requirement: Scalar/map filter operators are allowlisted

`buildFilterCondition` SHALL accept only an explicit allowlist of operators for scalar and map (non-Array) columns: `=`, `!=`, `<`, `<=`, `>`, `>=`, `=|`, `!=|`, `=~`, `!~`. Any operator outside this set SHALL cause the function to return a non-nil error (wrapping a descriptive message), and the macro SHALL NOT emit a condition built from that operator. The operator string SHALL NOT reach the emitted SQL except as one of the allowlisted comparison tokens.

#### Scenario: Injected operator is rejected

- **GIVEN** a filter `{Key: "status", Operator: "= 'x' OR 1=1 -- ", Value: "x"}` against a `String` column
- **WHEN** `buildFilterCondition` runs
- **THEN** it SHALL return a non-nil error
- **AND** SHALL NOT return a condition containing `OR 1=1`

#### Scenario: All supported operators produce a condition

- **GIVEN** a filter with a non-empty, non-NULL value against a `String` column
- **WHEN** `buildFilterCondition` runs once for each allowlisted operator
- **THEN** each call SHALL return a nil error and a non-empty condition

#### Scenario: Unknown operator on a scalar column errors like the Array path

- **GIVEN** a filter `{Key: "n", Operator: "BETWEEN", Value: "1"}` against an `Int64` column
- **WHEN** `buildFilterCondition` runs
- **THEN** it SHALL return a non-nil error
- **AND** the error SHALL name the unsupported operator

### Requirement: Map filter keys are validated and quoted

When a filter key matches the map-subscript form `column['subscript']`, `AdHocFilterMacro` SHALL validate the base `column` against the resolved CTE schema (as today) and SHALL rebuild the key used in SQL from trusted pieces: the base column emitted as a backtick-quoted identifier and the subscript emitted as a single-quoted escaped literal (via `escape`). The raw `filter.Key` SHALL NOT be interpolated into the emitted condition for map columns. A filter whose base column is not in the schema SHALL be dropped, as today.

#### Scenario: Injected subscript cannot break out

- **GIVEN** a filter `{Key: "attrs['a'] OR 1=1 --']", Operator: "=", Value: "v"}` where `attrs` is a `Map(String, String)` column in the schema
- **WHEN** `AdHocFilterMacro` runs
- **THEN** the emitted condition SHALL be a single map-access comparison over the backtick-quoted base column
- **AND** the entire injected subscript SHALL appear only inside a single-quoted, escaped map-key literal, with the embedded quote backslash-escaped so it cannot terminate the literal

#### Scenario: Honest map filter emits quoted key

- **GIVEN** a filter `{Key: "attrs['env']", Operator: "=", Value: "prod"}` where `attrs` is a `Map(String, String)` column
- **WHEN** `AdHocFilterMacro` runs
- **THEN** the emitted condition SHALL reference the key as `` `attrs`['env'] `` with the column backtick-quoted and the subscript single-quoted

#### Scenario: Subscript with a quote is escaped, not injected

- **GIVEN** a filter `{Key: "attrs['a'']', Operator: "=", Value: "v"}` where `attrs` is a `Map(String, String)` column
- **WHEN** `AdHocFilterMacro` runs
- **THEN** the subscript SHALL be emitted with the single quote backslash-escaped inside the literal
- **AND** the literal SHALL NOT be terminated early
