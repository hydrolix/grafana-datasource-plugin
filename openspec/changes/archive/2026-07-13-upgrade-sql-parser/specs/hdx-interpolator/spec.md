## ADDED Requirements

### Requirement: SQL parser is pinned at v0.5.2 or later

The plugin SHALL depend on `github.com/hydrolix/clickhouse-sql-parser` at v0.5.2 or later. Because that version removes `String()` from the `Expr` interface and all AST nodes, the plugin SHALL serialize AST nodes to SQL text via `parser.Format(node)` rather than `node.String()`.

#### Scenario: AST serialization uses parser.Format

- **GIVEN** the plugin's Go backend serializing a parsed `TableIdentifier` or `FROM` expression to text
- **WHEN** the serialization runs
- **THEN** it SHALL call `parser.Format(node)`
- **AND** SHALL NOT call `node.String()` (which no longer exists on AST nodes)

## MODIFIED Requirements

### Requirement: `cte.GetMacroCTEs` extracts CTE associations from the AST

The plugin SHALL define `cte.GetMacroCTEs(ast []parser.Expr) (map[MacroId]CTE, error)` in `pkg/plugin/cte/`. The function returns one CTE entry per macro call site, capturing the surrounding `FROM` expression and resolved table / database. The table, database, and CTE strings SHALL be produced via `parser.Format` on the corresponding AST nodes, yielding text equivalent to the pre-upgrade `String()` serialization.

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
