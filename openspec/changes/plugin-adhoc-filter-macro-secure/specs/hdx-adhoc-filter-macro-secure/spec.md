# hdx-adhoc-filter-macro-secure

## ADDED Requirements

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

## MODIFIED Requirements

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
