# hdx-interpolator Specification

## Purpose
TBD - created by archiving change plugin-hdx-interpolator. Update Purpose after archive.
## Requirements
### Requirement: `HdxInterpolator` implements `sqlds.Interpolator`

The plugin SHALL define `HdxInterpolator` in `pkg/plugin/interpolator.go` whose
`Interpolate` method satisfies the `sqlds.Interpolator` *func type*
(`func(ctx, *sqlutil.Query, json.RawMessage) (string, error)`). The signature
SHALL NOT carry a `*SQLDatasource` parameter — the datasource and metadata the
interpolator needs are captured via `md`/`macros` at construction. A
compile-time assertion (`var _ sqlds.Interpolator = (&HdxInterpolator{}).Interpolate`)
SHALL guard the signature. The implementation SHALL be safe for concurrent use
across queries.

#### Scenario: Func-type conformance

- **GIVEN** the `(&HdxInterpolator{}).Interpolate` method value
- **WHEN** assigned to a `sqlds.Interpolator` variable or to `SQLDatasource.Interpolator`
- **THEN** the assignment SHALL compile without error
- **AND** the signature SHALL omit the `*SQLDatasource` parameter

#### Scenario: Concurrent dispatch is race-free

- **GIVEN** a `*HdxInterpolator` shared across many goroutines
- **WHEN** each goroutine calls `Interpolate(...)` with independent queries
- **THEN** all calls SHALL complete without data races (under `go test -race`)

### Requirement: `Macros` package-level registry

The plugin SHALL define `Macros map[string]MacroFunc` as a package-level variable in `pkg/plugin/macros_registry.go`. Subsequent changes (C6, C7) populate it via `init()` blocks; the map is read-only after init.

#### Scenario: Registry is populated at startup

- **GIVEN** the `plugin` package
- **WHEN** the package's `init()` functions have run
- **THEN** `Macros["conditionalAll"]` SHALL be set to the `Stub` function
- **AND** other entries SHALL be added by C6 and C7

### Requirement: `MacroFunc` signature matches the plugin's macro contract

The plugin SHALL define `MacroFunc` as `func(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, md *MetadataProvider) (string, error)`.

#### Scenario: Macros conform to the signature

- **GIVEN** a macro function defined as `func(ctx, *models.HdxQuery, []string, parser.Pos, *MetadataProvider) (string, error)`
- **WHEN** assigned to a `MacroFunc` variable
- **THEN** the assignment SHALL compile without error

### Requirement: `cte.GetMacroCTEs` extracts CTE associations from the AST

The plugin SHALL define `cte.GetMacroCTEs(ast []parser.Expr) (map[MacroId]CTE, error)` in `pkg/plugin/cte/`. The function returns one CTE entry per macro call site, capturing the surrounding `FROM` expression and resolved table / database.

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

### Requirement: `cte.MacroPositions` reports macro call sites

The plugin SHALL define `cte.MacroPositions(input string) ([]parser.Pos, error)` that returns the byte positions of `$__`-prefixed identifiers in the parsed AST.

#### Scenario: Positions returned for valid SQL

- **GIVEN** SQL `SELECT $__timeFilter, $__dateFilter FROM events`
- **WHEN** `MacroPositions` is invoked
- **THEN** the result SHALL contain two positions

#### Scenario: Error on malformed SQL

- **GIVEN** SQL that does not parse (`SELEC FRO :: bad`)
- **WHEN** `MacroPositions` is invoked
- **THEN** the function SHALL return a non-nil error

### Requirement: Macro dispatch in `Interpolate`

`HdxInterpolator.Interpolate` SHALL parse `query.RawSQL`, find macro call sites via `cte.MacroPositions`, dispatch each registered macro at its AST position, and replace the call site's bytes with the macro's return value. Macros SHALL be applied in reverse byte order so earlier offsets remain valid as later regions change length. Macro keys SHALL be sorted by length descending so longer macro names (e.g. `timeFilter_ms`) match before shorter prefix-overlapping names (e.g. `timeFilter`).

#### Scenario: Registered macro is dispatched

- **GIVEN** a `HdxInterpolator` with macro `upper` returning `"UPPER(" + args[0] + ")"`
- **WHEN** `Interpolate` is invoked on `SELECT $__upper(name) FROM t`
- **THEN** the output SHALL contain `"UPPER(name)"`
- **AND** the output SHALL NOT contain `"$__upper"`

#### Scenario: Unknown macro is left in place

- **GIVEN** an empty macro registry
- **WHEN** `Interpolate` is invoked on `SELECT $__unknownMacro() FROM t`
- **THEN** the output SHALL equal the input (the macro call site is preserved)

#### Scenario: Longer macro names match first

- **GIVEN** a registry with both `timeFilter` and `timeFilter_ms`
- **WHEN** `Interpolate` is invoked on `SELECT $__timeFilter_ms() FROM t`
- **THEN** the dispatched macro SHALL be `timeFilter_ms`, not `timeFilter`

#### Scenario: Escaped macros strip one leading `$`

- **GIVEN** SQL `SELECT $$__conditionalAll() FROM t`
- **WHEN** `Interpolate` is invoked
- **THEN** the output SHALL contain `$__conditionalAll()`
- **AND** the macro SHALL NOT be expanded

### Requirement: `Round` time-range adjustment runs before macro dispatch

When `query.Round` is set to a non-zero duration string, the interpolator SHALL round `query.TimeRange.From` and `query.TimeRange.To` to multiples of that duration before any macro dispatches.

#### Scenario: Round applied before macro reads TimeRange

- **GIVEN** a query with `Round = "1m"` and `TimeRange.From = 12:00:30 UTC`
- **WHEN** a macro that reads `query.TimeRange.From` runs
- **THEN** the macro SHALL observe the rounded value (`12:00:00 UTC`)

#### Scenario: Invalid or sub-second `Round` is a no-op

- **GIVEN** `Round = "500ms"` or `Round = "not-a-duration"`
- **WHEN** the interpolator runs
- **THEN** `query.TimeRange` SHALL be unchanged

### Requirement: Unbalanced macro parentheses surface a typed error

When a macro identifier in the SQL is followed by an unbalanced argument list (open paren without close), the interpolator SHALL return `ErrParseMacroArgs` so `sqlds.handleQuery` classifies the failure as a downstream error.

#### Scenario: Unbalanced parens produce the typed error

- **GIVEN** SQL containing `$__foo(missingClose`
- **WHEN** `Interpolate` runs against a registry that has `foo`
- **THEN** the returned error SHALL wrap `ErrParseMacroArgs`

### Requirement: `MacroCTEs` HTTP handler returns the plugin-local CTE list

The plugin's `/macroCTE` HTTP route SHALL invoke `cte.GetMacroCTEs` on the parsed AST and return the resulting list as `Response[[]cte.CTE]`. The handler restores the full implementation that was temporarily stubbed in C2.

#### Scenario: Valid SQL yields the CTE map

- **GIVEN** a POST to `/macroCTE` with body `{"data":{"query":"SELECT $__timeFilter FROM mydb.events"}}`
- **WHEN** the handler runs
- **THEN** the response SHALL have `error: false`
- **AND** the `data` field SHALL contain one CTE entry with `Table: "events"`, `Database: "mydb"`

#### Scenario: Malformed SQL produces a wrapped error

- **GIVEN** a POST to `/macroCTE` with malformed SQL
- **WHEN** the handler runs
- **THEN** the response SHALL have `error: true`
- **AND** the `errorMessage` SHALL describe the parse failure

### Requirement: `MetadataProvider` placeholder for C7

The plugin SHALL define `MetadataProvider` as an empty struct in `pkg/plugin/metadata.go` with a constructor `NewMetadataProvider() *MetadataProvider`. C7 (`plugin-adhoc-filter-macro-secure`) replaces this file with the TTL-cached implementation. The placeholder exists so `HdxInterpolator` and the macro signature compile in C5 alone.

#### Scenario: Constructor returns a non-nil value

- **GIVEN** the C5 plugin build
- **WHEN** `NewMetadataProvider()` is invoked
- **THEN** the returned `*MetadataProvider` SHALL be non-nil
- **AND** the `getPK` helper SHALL return `ErrMetadataProviderUnavailable` when called

### Requirement: `Interpolate` routes runtime fields from `sqlutil.Query`

When invoked as the `sqlds.Interpolator` func, the implementation SHALL overlay `query.RawSQL`, `query.TimeRange`, and `query.Interval` from the upstream `*sqlutil.Query` onto the unmarshalled `models.HdxQuery`, so the sqlds-derived runtime context takes precedence over anything carried in `rawJSON`.

#### Scenario: Runtime fields override JSON content

- **GIVEN** a `rawJSON` payload containing one `RawSQL` and a `*sqlutil.Query` containing a different `RawSQL`
- **WHEN** `Interpolate` runs
- **THEN** the macro dispatch SHALL operate on the `*sqlutil.Query`'s `RawSQL`, not the JSON's

