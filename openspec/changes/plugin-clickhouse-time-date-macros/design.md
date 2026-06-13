## Context

The fork at `0f83082` carries eleven ClickHouse-specific macros in `macros.go:45-189`. They share three structural properties:

1. **They consume `query.TimeRange.From` / `query.TimeRange.To`** — the macros are how dashboards inject Grafana's time-picker selection into SQL. Without them, dashboards are useless against Hydrolix.
2. **They emit ClickHouse-specific SQL** — `fromUnixTimestamp64Milli`, `toStartOfInterval(toDateTime(...), INTERVAL N second)`, etc. Not portable to other dialects; correct only when the target is ClickHouse / Hydrolix.
3. **Five of them resolve the column argument via PK lookup when omitted.** `$__timeFilter` and `$__timeInterval` (plus their `Ms` variants) call `getPK(ctx, query.RawSQL, pos, mdProvider, query.Headers)` when no column argument is given; the helper walks `mdProvider`'s schema cache to find the primary-key column for the table referenced at `pos`. This makes those macros depend on `MetadataProvider` (defined in C7).

The macros' `MacroFunc` signature (C5's `func(ctx, *models.HdxQuery, []string, parser.Pos, *MetadataProvider) (string, error)`) carries `*MetadataProvider` even for macros that don't use it (they `_`-ignore it). Mixed-signature support is not needed — every macro takes the same parameters; the unused ones are free.

`sqlutil.ErrorBadArgumentCount` is the upstream error sentinel for "wrong number of arguments to a macro". The fork's macros wrap it with `backend.DownstreamError` so Grafana surfaces a user-visible error rather than treating the failure as a plugin bug.

## Goals / Non-Goals

**Goals:**
- Port the eleven time/date/interval macros from the fork verbatim, registered into the `Macros` registry established by C5.
- Port the time-conversion helpers (`timeToDate`, `timeToDateTime`, `timeToDateTime64`) from the fork verbatim.
- Port the fork's macro tests verbatim; preserve the existing regression net.

**Non-Goals:**
- Defining `MetadataProvider` or `getPK`. Both live in C7.
- Adding new macros. The eleven existing macros are the surface; expanding the surface is a future change.
- Optimising macro output. ClickHouse-side `toStartOfInterval` is fast and well-supported; no rewrite needed.
- Adding macro-level caching (e.g., memoising PK lookups across macro calls in the same `Interpolate`). The fork doesn't, and `MetadataProvider` (C7) carries its own AST/schema cache.

## Decisions

### D1. One file per concern: `macros_clickhouse.go` for the macros, `macros_time.go` for the helpers

```go
// pkg/plugin/macros_time.go
func timeToDate(t time.Time) string     { … }
func timeToDateTime(t time.Time) string { return fmt.Sprintf("fromUnixTimestamp(%d)", t.Unix()) }
func timeToDateTime64(t time.Time) string { return fmt.Sprintf("fromUnixTimestamp64Milli(%d)", t.UnixMilli()) }

// pkg/plugin/macros_clickhouse.go
func FromTimeFilter(_ context.Context, query *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
    return timeToDateTime(query.TimeRange.From), nil
}
// … nine more macros …

func init() {
    Macros["fromTimeFilter"]   = FromTimeFilter
    Macros["toTimeFilter"]     = ToTimeFilter
    Macros["fromTimeFilterMs"] = FromTimeFilterMs
    Macros["toTimeFilterMs"]   = ToTimeFilterMs
    Macros["timeFilter"]       = TimeFilter
    Macros["timeFilterMs"]     = TimeFilterMs
    Macros["dateFilter"]       = DateFilter
    Macros["dateTimeFilter"]   = DateTimeFilter
    Macros["timeInterval"]     = TimeInterval
    Macros["timeIntervalMs"]   = TimeIntervalMs
    Macros["intervalSeconds"]  = IntervalSeconds
}
```

**Why two files.** The helpers (`timeToDate`, `timeToDateTime`, `timeToDateTime64`) are small, stateless, and reusable across macros. The macros themselves are larger and have a clear test surface. Separation keeps each file focused; the test files (`macros_clickhouse_test.go`, paired) target one concern.

**Why one `init()` rather than per-macro `init()`s.** A single `init()` reads like a registration manifest — every macro and its registered name visible in one place. Per-macro `init()`s would scatter the registry population.

**Why register by camelCase string names.** The dashboard's `RawSQL` contains `$__fromTimeFilter(...)`. The interpolator's `getMacroMatches` strips the `$__` prefix and looks up by the remaining name. The fork uses camelCase; this change matches verbatim.

### D2. PK-lookup macros call `getPK` from C7

```go
func TimeFilter(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, mdProvider *MetadataProvider) (string, error) {
    if len(args) > 1 {
        return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
    }
    column := ""
    if len(args) == 1 && args[0] != "" {
        column = args[0]
    } else {
        pk, err := getPK(ctx, query.RawSQL, pos, mdProvider, query.Headers)
        if err != nil {
            return "", err
        }
        column = pk
    }
    return fmt.Sprintf("%s >= %s AND %s <= %s", column, timeToDateTime(query.TimeRange.From), column, timeToDateTime(query.TimeRange.To)), nil
}
```

`getPK` is declared in C7's `metadata.go`. Both files exist in `package plugin`, so the cross-file reference compiles without an explicit import.

**Why `getPK` and not direct `mdProvider.PrimaryKey(...)` access.** The fork wraps the call in `getPK(ctx, sql, pos, mdProvider, headers)` because the PK lookup depends on which table is at `pos` in `sql`, with `headers` carrying auth/org-id context for the underlying schema query. The helper composes the AST walk (find the table at `pos`) + the metadata lookup + the auth flow. C6's macros stay clean by calling the helper.

**Why no fallback when `mdProvider` is nil.** If `mdProvider == nil`, `getPK` returns an error. The macro propagates. Dashboards lacking a metadata provider can't use omitted-column variants — they must pass the column explicitly. This matches the fork.

### D3. Argument-count validation: exact-match upstream behaviour

The fork's macros validate `len(args)` and return `backend.DownstreamError(fmt.Errorf("%w: expected X argument(s), received Y", sqlutil.ErrorBadArgumentCount, len(args)))`. This change uses the identical pattern.

**Why `backend.DownstreamError`.** Grafana classifies plugin errors as "plugin" (the plugin itself failed) vs "downstream" (the data source returned an error). Macro arg-count mismatches are user errors (wrong dashboard SQL); marking them downstream makes Grafana display the error in the panel rather than as a plugin crash.

**Why exact `sqlutil.ErrorBadArgumentCount` wrapping.** The upstream `handleQuery` (`sqlds@ef925e1`) checks `errors.Is(err, sqlutil.ErrorBadArgumentCount)` to classify the error path; wrapping preserves that classification chain. Replacing with a custom error sentinel would break upstream's error-type matching.

### D4. Stateless-macro signature still takes `*MetadataProvider`

`FromTimeFilter`, `ToTimeFilter`, etc. take `*MetadataProvider` as their fifth parameter but `_`-ignore it.

**Why not split the signature into `MacroFunc` (no `*MetadataProvider`) and `MacroFuncMD` (with)?** The dispatch loop in `HdxInterpolator.Interpolate` (C5) calls every macro with the same five arguments. Two signatures would force a type-switch at dispatch, which buys nothing: every macro's signature is fixed at compile time. The `_`-ignored parameter is free.

**Why not pass `nil` for `*MetadataProvider` to stateless macros to save a struct allocation?** `*MetadataProvider` is a pointer; passing `nil` vs a real pointer is the same size. No allocation saved.

### D5. Test corpus ports the fork's `macros_test.go` for the eleven macros verbatim

The fork's `macros_test.go` (1004 LOC at `0f83082`) tests:
- Time-range conversion at epoch boundaries (UTC, sub-second precision, year-2050 edge).
- Sub-second `Interval` values (returns 1 second floor).
- Arg-count error paths (0, 1, 2, many).
- Column-omitted-with-PK-lookup paths (mocks `MetadataProvider`).
- Column-quoted-vs-bare paths (the macros are not auth/escape-aware; they pass the column through unchanged — which is correct for non-user-supplied schema columns and also matches the fork).

This change ports the ~600 LOC of those tests covering the eleven time/date/interval macros (the adHoc-filter test cases live with C7). Mock for `MetadataProvider` is the same mock C7 defines.

**Why port rather than rewrite.** Same reason as the interpolator porting in C5: preserve the regression net during the move. Rewriting tests during a code-move muddies behaviour-drift signals.

**Why one test file per macros file.** `macros_clickhouse_test.go` lives next to `macros_clickhouse.go`. Standard Go convention.

### D6. No security-relevant escape changes in this change

The eleven macros emit:
- Pure numeric expressions (`fromUnixTimestamp64Milli(1234567890)`, `<integer>`).
- Column-name interpolations that the user supplied as macro arguments (`<col_name> >= X AND <col_name> <= Y`). Column names are SQL identifiers, not values; the fork passes them through unchanged. ClickHouse rejects malformed identifiers at parse time, so a malicious argument (`x OR 1=1`) becomes a parse error, not a SQL injection.

The catalog-review finding ("`$$…$$` dollar-quoted literals are injectable") applies to the **value-interpolation** path inside the ad-hoc filter macro, which lives in C7. This change introduces no new value-interpolation surfaces.

**Why call this out explicitly.** Reviewers seeing eleven macros copied verbatim from a fork flagged for security issues will reasonably ask if the issue applies here. The answer is no: the time/date/interval macros do not interpolate user-supplied values, only user-supplied column names that pass through ClickHouse's parse step.

### D7. Macro names are stable; renames are out of scope

The dashboard `RawSQL` invokes macros as `$__fromTimeFilter`, `$__timeInterval`, etc. The registered names (`"fromTimeFilter"`, `"timeInterval"`) are dashboard contract. Renaming would break every existing dashboard.

**Why preserve names verbatim.** Behaviour preservation is the migration's first invariant. A future cleanup that renames macros would need a dashboard-migration step (rewrite stored queries); not in scope.

## Risks / Trade-offs

- **[Time-range conversion drifts between fork and migrated macros]** → Mitigation: ported tests exercise epoch-boundary, sub-second, year-2050 cases. Any drift fails a test loudly. Confirmed by running the ported test suite against the migrated code.
- **[Macro-name drift between dashboard and registry]** → Mitigation: integration test in `interpolator_test.go` (from C5) feeds a dashboard-style query (`SELECT ... WHERE $__timeFilter(ts)`) and asserts the output contains the expected ClickHouse expression. A typo in either side fails the test.
- **[`getPK` returns ambiguous results when the SQL has multiple FROM tables]** → Existing fork behaviour; ported verbatim. The macro's behaviour on ambiguity depends on `MetadataProvider`'s implementation (C7) — the macros themselves just propagate the error. Documented in C7's risk section.
- **[`sqlutil.ErrorBadArgumentCount` is renamed/removed upstream in a future SDK release]** → Mitigation: a build-time constant assertion (`var _ = sqlutil.ErrorBadArgumentCount`) in the macros file fails compilation if the symbol moves. Loud, not silent.

## Migration Plan

- **Forward**: ships in the C2-C7 coordinated merge window. Sequence inside its PR commit (or PR if stacked):
  1. Add `pkg/plugin/macros_time.go` with `timeToDate`, `timeToDateTime`, `timeToDateTime64`.
  2. Add `pkg/plugin/macros_clickhouse.go` with the eleven macros + the `init()` registration block.
  3. Add `pkg/plugin/macros_clickhouse_test.go` ported from the fork's `macros_test.go` cases for the eleven macros.
  4. Run quality gates: `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`.
- **Rollback**: revert this change's commit/PR. The `Macros` registry has zero entries; every panel query using a time macro fails interpolation. Rollback requires reverting C5 and the macros together.
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2), `plugin-hdx-interpolator` (C5) for the dispatch path, and `plugin-adhoc-filter-macro-secure` (C7) for `MetadataProvider` + `getPK`. Ships in the same coordinated merge window.

## Open Questions

- Should `IntervalSeconds`' minimum-1-second floor be documented as user-visible behaviour (e.g., in the plugin's docs)? The fork doesn't. Defer; revisit if a deployment surfaces sub-second-interval needs.
- Should the macros adopt typed argument structs (e.g., `type TimeFilterArgs struct { Column string }`) instead of `[]string`? Defer — `[]string` is upstream's macro shape and matches the fork. Typing buys clarity at the cost of more boilerplate per macro; weigh in a future cleanup change.
- Should `timeToDate` / `timeToDateTime` / `timeToDateTime64` move to `pkg/plugin/models/` so future non-macro code can reuse them? Defer — they have no current consumer outside the macros.
