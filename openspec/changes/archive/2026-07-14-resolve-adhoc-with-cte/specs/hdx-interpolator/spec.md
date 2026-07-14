## MODIFIED Requirements

### Requirement: `cte.GetMacroCTEs` extracts CTE associations from the AST

The plugin SHALL define `cte.GetMacroCTEs(ast []parser.Expr) (map[MacroId]CTE, error)` in `pkg/plugin/cte/`. The function returns one CTE entry per macro call site, capturing the surrounding `FROM` expression and resolved table / database. The table, database, and CTE strings SHALL be produced via `parser.Format` on the corresponding AST nodes, yielding text equivalent to the pre-upgrade `String()` serialization. When the `FROM` expression is a bare identifier that matches a WITH-clause alias in scope (nearest enclosing scope winning on shadowing), the entry's `CTE` field SHALL be the alias's defining subquery, parenthesized, rather than the alias name. A bare identifier that matches no in-scope WITH alias SHALL keep its identifier form.

#### Scenario: Macro inside a simple SELECT

- **GIVEN** the SQL `SELECT $__timeFilter FROM mydb.events`
- **WHEN** `GetMacroCTEs` is invoked on the parsed AST
- **THEN** the return map SHALL contain exactly one entry
- **AND** the entry's `Macro` field SHALL be `"$__timeFilter"`
- **AND** the entry's `Table` field SHALL be `"events"`
- **AND** the entry's `Database` field SHALL be `"mydb"`

#### Scenario: Macro with bare table reference (no database qualifier)

- **GIVEN** the SQL `SELECT $__timeFilter FROM events`
- **WHEN** `GetMacroCTEs` is invoked
- **THEN** the entry's `Database` field SHALL be empty

#### Scenario: FROM references a WITH alias

- **GIVEN** the SQL `WITH x AS (SELECT a FROM events) SELECT $__adHocFilter() FROM x`
- **WHEN** `GetMacroCTEs` is invoked
- **THEN** the entry's `CTE` field SHALL be the parenthesized subquery `(SELECT a FROM events)`
- **AND** SHALL NOT be the bare identifier `x`

#### Scenario: Shadowed alias resolves to the nearest scope

- **GIVEN** SQL where an alias name is defined in both an outer and an inner `WITH` clause and the macro sits in the inner SELECT selecting from that alias
- **WHEN** `GetMacroCTEs` is invoked
- **THEN** the entry's `CTE` field SHALL be the inner alias's defining subquery
