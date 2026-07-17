## Context

`$__adHocFilter` is the only macro in the Hydrolix plugin whose output interpolates user-supplied values (the Grafana ad-hoc filter UI lets users type the value to filter on). Every other macro interpolates time ranges (numeric) and column identifiers (SQL identifiers, not values). This asymmetry makes the ad-hoc filter macro the locus of the catalog-review security finding.

At `0f83082`, `buildArrayCondition` (`macros.go:256+`) emits:

```go
fmt.Sprintf("has(%s, $$%s$$)", key, v) // v is filter.Value, user-supplied
```

`$$…$$` is ClickHouse's "dollar-quoted literal" syntax: anything between the opening and closing `$$` is taken verbatim, with the closing `$$` terminating the literal. A value containing `$$` (`"foo$$bar"`) terminates the literal early; downstream tokens become SQL. Example: `has(arr, $$foo$$ OR 1=1 --$$)` parses as `has(arr, 'foo') OR 1=1 --…` — classic injection.

`buildFilterCondition` (`macros.go:283+`) has the same pattern in multiple branches for non-array column types.

The fix is the standard SQL pattern: emit single-quoted literals with backslash escapes. ClickHouse's literal grammar accepts `'` → `\'`, `\` → `\\`, `\n` → `\n`, NUL → `\0`. The plugin's `escape` helper handles every character ClickHouse treats specially inside single-quoted literals.

`MetadataProvider` (the fork's `MetaDataProvider`) is the macro's support layer: it caches the primary key per `(database, table)` and the column-type map per CTE name. Both caches use `jellydator/ttlcache/v3` (1-hour TTL). Schema queries are routed through the datasource (`ds.QueryData(...)`), so they participate in the OAuth-keyed pooling (C4) and TTL connection cache (C3).

## Goals / Non-Goals

**Goals:**
- Port `$__adHocFilter` and its `buildArrayCondition` / `buildFilterCondition` helpers into the plugin.
- **Replace `$$…$$` dollar-quoted literals with single-quoted escaped literals at every user-supplied value interpolation site.**
- Port `MetadataProvider` (TTL-cached PK + column-type lookups) into the plugin.
- Register `adHocFilter` into the `Macros` registry via `init()`.
- Pass `MetadataProvider` to the macro through the existing `MacroFunc` signature (C5).

**Non-Goals:**
- Reworking the filter-condition logic. The macro emits the same shapes (`<col> IN (...)`, `has(<col>, ...)`, `NOT LIKE`, etc.) as the fork; only the value-literal encoding changes.
- Adopting `?`-style driver-level parameter binding (which would obviate escape rules entirely). Macro outputs are spliced into a SQL string and re-parsed; the binding boundary is upstream of the macro, not below it. Driver binding is a future cleanup.
- Adding new filter operators or new column-type handling. Surface preserved.
- Changing the `MetadataProvider` cache shapes or TTLs. The fork uses 1-hour for both caches; this change matches.

## Decisions

### D1. `escape(s string) string` — ClickHouse single-quoted-literal escape

```go
// pkg/plugin/macros_adhoc.go
// escape returns s with characters that have special meaning inside a
// ClickHouse single-quoted literal replaced by their escape sequences:
//   '  -> \'
//   \  -> \\
//   \n -> \n  (newline character; emitted as the two characters '\' 'n')
//   \r -> \r
//   \t -> \t
//   \0 -> \0  (NUL byte)
// Every other byte is preserved verbatim. The result is safe to surround
// with single quotes to form a ClickHouse string literal.
func escape(s string) string {
    var b strings.Builder
    b.Grow(len(s) + 4) // common case: a few extra chars for escapes
    for i := 0; i < len(s); i++ {
        switch s[i] {
        case '\'':
            b.WriteString(`\'`)
        case '\\':
            b.WriteString(`\\`)
        case '\n':
            b.WriteString(`\n`)
        case '\r':
            b.WriteString(`\r`)
        case '\t':
            b.WriteString(`\t`)
        case 0:
            b.WriteString(`\0`)
        default:
            b.WriteByte(s[i])
        }
    }
    return b.String()
}
```

**Why byte-by-byte iteration, not `strings.ReplaceAll` chained.** Chained replacement creates intermediate strings (allocates O(n) per replace). Byte-by-byte with `strings.Builder` is one pass, one allocation amortised. Macro evaluation runs on the query hot path; matter of microseconds across many filters.

**Why escape `\n`, `\r`, `\t`, `\0` and not just `'` and `\`.** ClickHouse's literal grammar allows raw newlines, tabs, etc. inside single-quoted literals, but downstream SQL pretty-printers / loggers / proxy layers sometimes barf on multi-line literals or invisible control chars. Conservative escape (every special char) keeps the emitted SQL clean for the whole pipeline. The "cost" is a few escape sequences in legitimate values; SQL evaluation produces the same output.

**Why no Unicode-aware processing.** ClickHouse accepts arbitrary UTF-8 inside single-quoted literals as-is. The escape function operates on bytes, not runes; UTF-8 multi-byte sequences pass through verbatim. Test coverage includes Unicode values to confirm.

**Why a function, not a type method on a `ValueEscaper` struct.** A standalone function is the simpler shape. No state, no construction.

### D2. Every value interpolation goes through `escape`; no exceptions

`buildArrayCondition` and `buildFilterCondition` together have 6+ value-interpolation sites at `0f83082`. Each one becomes:

```go
// Before
fmt.Sprintf("has(%s, $$%s$$)", key, v)
// After
fmt.Sprintf("has(%s, '%s')", key, escape(v))
```

Identifier interpolation (`key` here is a column name) does not go through `escape` — column names are SQL identifiers, not value literals. ClickHouse's identifier grammar enforces its own constraints; passing through unchanged is correct.

**Why a separate helper for values, no helper for identifiers.** Identifiers and values have different grammars; conflating them in one helper would risk misuse. The macro's argument shape distinguishes them: `filter.Key` is the identifier (column), `filter.Value` / `filter.Values` are the values. Type discipline is the safest pattern.

**Why preserve identifier passthrough.** Column names in the ad-hoc filter UI come from Grafana's variable-substitution layer, which has already constrained them to schema-known columns. The macro's own `if slices.Contains(keyNames, column)` gate (against the metadata-provider's known keys) prevents free-form-string injection through the column path.

### D3. `MetadataProvider` holds the wrapper, not just settings

```go
// pkg/plugin/metadata.go
type MetadataProvider struct {
    ds       *HdxSqlDatasource
    pkCache  *ttlcache.Cache[string, string]
    keyCache *ttlcache.Cache[string, map[string]string]
}

func NewMetadataProvider(ds *HdxSqlDatasource) *MetadataProvider {
    pkCache := ttlcache.New[string, string](
        ttlcache.WithTTL[string, string](time.Hour),
    )
    keyCache := ttlcache.New[string, map[string]string](
        ttlcache.WithTTL[string, map[string]string](time.Hour),
    )
    return &MetadataProvider{ds: ds, pkCache: pkCache, keyCache: keyCache}
}
```

The provider needs to issue schema queries (`DESCRIBE table`, `SELECT primary_key FROM system.tables WHERE ...`). The cleanest way is to call `ds.QueryData(...)` — the same pipeline panel queries go through, so OAuth keying (C4) and connection caching (C3) apply identically.

**Why pass the wrapper rather than a function.** The provider needs both settings (default database for `getDefaultDatabase`) and the query execution path. The wrapper carries both: settings via `ds.settings`, queries via `ds.QueryData`. A function-based interface would force two function values to be passed; one wrapper pointer is simpler.

**Why no back-pointer guard against nil `ds`.** The constructor mandates a non-nil wrapper. Tests can pass a fake wrapper. Defensive nil checks at every method are noise that hides logic errors.

**Why two caches, not one with a discriminated key.** The two caches have different value types (`string` for PK, `map[string]string` for keys). Combining them would require an interface value or a tagged union; both are heavier than two typed caches.

### D4. `getPK` helper composes AST walk + PK lookup

```go
// pkg/plugin/metadata.go
func getPK(ctx context.Context, rawSQL string, pos parser.Pos, mdProvider *MetadataProvider, headers http.Header) (string, error) {
    // 1. Parse rawSQL into AST.
    // 2. Find the table reference at pos. Yields (database, table) — database may be empty.
    // 3. Call mdProvider.GetPK(ctx, headers, database, table).
    // 4. Return the PK column name, or error.
}
```

Used by C6's PK-lookup macros (`TimeFilter`, `TimeFilterMs`, `TimeInterval`, `TimeIntervalMs`) and by `AdHocFilterMacro` when the macro's CTE argument is omitted.

**Why colocate with `MetadataProvider`.** `getPK` is metadata-machinery, not a macro. C6's macros call it; the macros file in C6 imports nothing from `metadata.go` directly — the helper sits in `package plugin` and gets called by name.

**Why a free function, not a method on `MetadataProvider`.** The function takes `*MetadataProvider` as a parameter, signalling that it composes the provider's operations rather than being one of them. Methods on `MetadataProvider` are the primitive operations (`GetPK`, `GetKeys`, `getDefaultDatabase`); `getPK` (lowercase) is the helper that uses them. Naming convention preserves the layer separation.

**Why lowercase `getPK`.** Internal to the package; no external consumer needs it.

### D5. `MetadataProvider` caches with 1-hour TTL, no eviction-callback close

Unlike `TTLConnectionCache` (C3), `MetadataProvider`'s caches hold strings and string maps — pure data, no `*sql.DB` to close. Eviction just discards the entry; no `OnEviction` callback needed.

```go
pkCache := ttlcache.New[string, string](
    ttlcache.WithTTL[string, string](time.Hour),
)
// no OnEviction
go pkCache.Start()
```

**Why one hour.** Matches the connection cache TTL. A schema change in upstream Hydrolix takes effect for the plugin within an hour of the change — acceptable lag, matches the fork.

**Why not invalidate on schema-change signal.** No signal exists. ClickHouse's `system.tables` reflects schema state, but the plugin doesn't subscribe to its mutation stream. Time-based eviction is the simplest correctness guarantee.

**Why no `Dispose` method.** `MetadataProvider`'s lifecycle is tied to `HdxSqlDatasource`. When the SDK calls `ds.Dispose()`, the wrapper's `Dispose` (promoted from `*sqlds.SQLDatasource`) terminates. Adding a separate `MetadataProvider.Dispose()` requires plumbing through the wrapper. The sweep goroutine in `ttlcache` exits when the process exits — fine for short-lived test runs and acceptable for long-running production where datasource instances are also long-lived. If a memory profile surfaces leaked sweep goroutines, add `Dispose`.

### D6. `HdxSqlDatasource` parses `models.PluginSettings` once at construction

The wrapper grows a field:

```go
type HdxSqlDatasource struct {
    *sqlds.SQLDatasource
    Settings *models.PluginSettings
}

func NewHdxSqlDatasource(driver sqlds.Driver, settings backend.DataSourceInstanceSettings) *HdxSqlDatasource {
    parsed, _ := models.NewPluginSettings(context.Background(), settings) // best-effort
    ds := sqlds.NewDatasource(driver)
    ds.EnableMultipleConnections = true
    ds.ConnectionCacheFactory = func() sqlds.ConnectionCache {
        return NewTTLConnectionCache(settings.UID, time.Hour)
    }
    wrapper := &HdxSqlDatasource{SQLDatasource: ds, Settings: parsed}
    mdProvider := NewMetadataProvider(wrapper)
    ds.Interpolator = NewHdxInterpolator(mdProvider, Macros)
    return wrapper
}
```

**Why cache the parsed settings.** `MetadataProvider.getDefaultDatabase` reads `settings.DefaultDatabase`. Re-parsing per call is wasteful; parse once at construction.

**Why best-effort parse (`_, _ := ...`).** Parsing can fail (malformed JSON in `settings.JSONData`); if it does, the datasource instantiation should still succeed so the user sees the error on `CheckHealth` or first query, not as a silent instantiation failure. The macro that uses settings handles `wrapper.Settings == nil` by returning an error.

**Why store as a pointer.** `nil` indicates "settings parse failed". A zero-value struct could be confused for "successfully parsed settings with all-empty fields". Pointer makes the failure state explicit.

### D7. CTE resolution: argument first, AST-position second

The macro's argument shape is `$__adHocFilter(<cte_name>)`. If `<cte_name>` is provided, use it directly. If omitted, parse the SQL, run `GetMacroCTEs`, find the CTE whose `MacroPos == pos`, use that name.

```go
var cte = ""
if len(params) == 1 {
    cte = params[0]
}
if cte == "" {
    expr, err := parser.NewParser(query.RawSQL).ParseStmts()
    if err != nil {
        return "", err
    }
    macroCTEs, err := GetMacroCTEs(expr)
    if err != nil {
        return "", err
    }
    for _, macroCTE := range macroCTEs {
        if macroCTE.MacroPos == pos {
            cte = macroCTE.CTE
            break
        }
    }
}
if cte == "" {
    return "", fmt.Errorf("cannot apply ad hoc filters: unable to resolve tableName for ad hoc filter at index %d", pos)
}
```

**Why argument-first.** Dashboard authors who explicitly specify a CTE name shortcut the AST walk and avoid ambiguity when the macro sits inside nested CTEs.

**Why fall back to AST resolution.** Most dashboards omit the argument; the macro is meant to be drop-in. AST-position lookup is the convenience.

**Why error on no-resolution.** Returning `1=1` on a failed resolution would silently disable the filter, which is worse than failing loudly. Operators investigating a "filters aren't applied" issue should see the error in the panel.

### D8. Test-corpus: porting from the fork + new escape-correctness cases

The fork's `macros_test.go` covers ad-hoc filter resolution paths (~400 LOC) and the fork's `metadata_test.go` covers cache behaviour (~120 LOC). Both port verbatim modulo type renames.

New tests this change adds (no fork equivalent):
- **Escape correctness**: filter value `O'Reilly` produces `'O\'Reilly'`; value containing `\` produces `\\`; value `$$payload$$` is fully escaped (no `$$` reaches the wire); value with multi-byte UTF-8 passes through unchanged; value with NUL byte produces `\0`; value with newline produces `\n`.
- **Round-trip property**: for a random sample of strings, the emission's parse-output equals the original input under ClickHouse's literal-parsing rules (fuzzed via `testing/quick`).
- **Multi-filter joining**: three filters, one with quoted value, one with `IN` list, one with array `has`, produce a single AND-joined expression with all values correctly escaped.

**Why fuzz the escape function.** The escape rules are small and obvious for ASCII; less obvious for byte sequences that happen to be valid UTF-8. A property-based test catches escape misses far more thoroughly than a finite-case test.

**Why one e2e test for the quote case.** Playwright tests are expensive to run; a single representative test (filter with `O'Reilly`) covers the integration path from frontend → backend → wire SQL. Unit tests cover the rest of the escape grammar more cheaply.

## Risks / Trade-offs

- **[Behaviour drift between fork's filter-condition shapes and migrated implementation]** → Mitigation: ported tests assert the same WHERE-clause text byte-for-byte (modulo the literal encoding). Any drift fails a test. Confirmed by running ported tests against migrated code.
- **[Escape misses a ClickHouse-special character]** → Mitigation: D8's fuzz test; the property-based test exercises the round-trip across thousands of random inputs. Risk surfaces immediately, not in production.
- **[Existing dashboards depended on `$$…$$` quirks (e.g., embedding SQL in a filter value)]** → Acceptable: dollar-quoting was a vulnerability, not a documented feature. A dashboard using it was already broken in subtle ways. The migration replaces the emission, not the macro's input shape, so any dashboard that relied on the macro's documented contract continues to work.
- **[`MetadataProvider`'s schema-query failure cascades the macro into an error]** → Mitigation: tests assert that `GetKeys` failure → macro returns `error` not `"1=1"`. Dashboard surfaces the error to the operator who can act on it (auth issue, table renamed, etc.). The fork behaves identically.
- **[1-hour cache TTL means schema changes take effect with up-to-1-hour lag]** → Acceptable: matches fork, matches connection cache. If a deployment surfaces a need for faster invalidation, add a manual-flush HTTP resource — out of scope here.
- **[`HdxSqlDatasource.Settings == nil` propagates as an opaque error in the macro]** → Mitigation: explicit nil-check in `getDefaultDatabase` with `backend.PluginError` wrapping; the message names the failure: "plugin settings not parsed; check datasource configuration". Surfacing in the panel guides the operator.
- **[Sweep goroutines leak across many datasource instances in tests]** → Mitigation: defer adding `Dispose` until profiling shows a problem; documented in D5.

## Migration Plan

- **Forward**: ships in the C2-C7 coordinated merge window. Sequence inside its PR commit (or PR if stacked):
  1. Add `pkg/plugin/metadata.go` with `MetadataProvider`, `getPK`, and schema-query primitives. Ports from the fork modulo `*HydrolixDatasource` → `*HdxSqlDatasource`, `models.HdxQuery` rename, etc.
  2. Add `pkg/plugin/macros_adhoc.go` with `AdHocFilterMacro`, `buildArrayCondition`, `buildFilterCondition`, and `escape`. Every `$$%s$$` site swaps to `'%s'` with `escape(value)`.
  3. Add `init()` block registering `Macros["adHocFilter"] = AdHocFilterMacro`.
  4. Add `pkg/plugin/metadata_test.go` and `pkg/plugin/macros_adhoc_test.go` ported from the fork + the new escape-correctness tests (D8).
  5. Update `pkg/plugin/hdx_sqlds.go` (from C2 + C3 + C5) to: parse settings on construction (D6); construct `MetadataProvider`; pass `MetadataProvider` to `NewHdxInterpolator`.
  6. Add the one new Playwright e2e for the quote-in-filter-value case.
  7. Run quality gates: `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`. E2E via the `grafana-plugin-e2e` skill — this is the change where the e2e suite begins to make sense to run end-to-end (C5 + C6 + C7 together produce a working query path).
- **Rollback**: revert this change's commit/PR. Without it, the ad-hoc filter macro is unregistered; `$__adHocFilter` calls fail interpolation. Rollback requires reverting C5 too.
- **Sequencing**: depends on C2 (wrapper), C3 (`ttlcache` direct dep), C5 (interpolator + `Macros` registry + `GetMacroCTEs`), and C6 (shared macro patterns). Ships in the same coordinated merge window.

## Open Questions

- Should the escape function also handle ClickHouse's `\xHH` hex escapes for arbitrary bytes (e.g., NUL → `\x00` rather than `\0`)? Defer — `\0` is the canonical ClickHouse syntax for NUL; both `\0` and `\x00` parse the same. The simpler escape table is easier to audit.
- Should `MetadataProvider` expose a `Flush()` method for an HTTP-triggered manual cache invalidation? Defer; add when a deployment surfaces a need.
- Should `escape` be moved to `pkg/plugin/models/` (alongside the `HdxQuery` shape) for future reuse by other macros? Defer — the only consumer today is the ad-hoc filter macro; premature relocation hides the security-critical helper deeper than it needs to be.
- Should the macro emit `JOIN` clauses for cross-table ad-hoc filters when the dashboard's `RawSQL` joins multiple tables? The fork doesn't. Defer — feature request territory, not migration territory.
