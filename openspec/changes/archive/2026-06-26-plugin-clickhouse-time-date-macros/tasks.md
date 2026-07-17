# plugin-clickhouse-time-date-macros — implementation tasks

## 1. Time-conversion helpers

- [x] 1.1 Add `pkg/plugin/macros_time.go` exporting (lowercase, package-internal): `timeToDate(t time.Time) string` → `toDate('YYYY-MM-DD')`, `timeToDateTime(t time.Time) string` → `toDateTime(<unix>)`, `timeToDateTime64(t time.Time) string` → `fromUnixTimestamp64Milli(<unixMilli>)`. Verbatim from fork's `macros.go:30-42`.

## 2. The eleven macros

- [x] 2.1 Add `pkg/plugin/macros_clickhouse.go`. Define the macros in this order to match the fork's file layout:
  - `FromTimeFilter` (zero args; returns `timeToDateTime(query.TimeRange.From)`).
  - `ToTimeFilter` (zero args; returns `timeToDateTime(query.TimeRange.To)`).
  - `FromTimeFilterMs` (zero args; returns `timeToDateTime64(query.TimeRange.From)`).
  - `ToTimeFilterMs` (zero args; returns `timeToDateTime64(query.TimeRange.To)`).
  - `TimeFilter` (0 or 1 arg; PK-lookup when omitted; emits `<col> >= toDateTime(...) AND <col> <= toDateTime(...)`).
  - `TimeFilterMs` (0 or 1 arg; PK-lookup when omitted; emits `<col> >= fromUnixTimestamp64Milli(...) AND ...`).
  - `DateFilter` (1 arg required; emits `<col> >= toDate(...) AND <col> <= toDate(...)`).
  - `DateTimeFilter` (2 args required; emits `(<dateCol> ... ) AND (<timeCol> ...)`).
  - `TimeInterval` (0 or 1 arg; PK-lookup when omitted; emits `toStartOfInterval(toDateTime(<col>), INTERVAL N second)` with `N = max(1, floor(query.Interval.Seconds()))`).
  - `TimeIntervalMs` (0 or 1 arg; PK-lookup when omitted; emits `toStartOfInterval(toDateTime64(<col>, 3), INTERVAL N millisecond)`).
  - `IntervalSeconds` (zero args; emits `N`).
- [x] 2.2 Every arg-count violation returns `backend.DownstreamError(fmt.Errorf("%w: expected X argument(s), received %d", sqlutil.ErrorBadArgumentCount, len(args)))`. Wrap shape preserved exactly.
- [x] 2.3 PK-lookup macros invoke `getPK(ctx, query.RawSQL, pos, mdProvider, query.Headers)` — the helper from C7's `metadata.go`. Drop the `//nolint:unused` from getPK since these macros are live callers.
- [x] 2.4 `init()` block in the same file registers all twelve names (eleven distinct funcs; `dt` is an alias for `DateTimeFilter`):
  ```go
  Macros["fromTime"]        = FromTimeFilter
  Macros["toTime"]          = ToTimeFilter
  Macros["fromTime_ms"]     = FromTimeFilterMs
  Macros["toTime_ms"]       = ToTimeFilterMs
  Macros["timeFilter"]      = TimeFilter
  Macros["timeFilter_ms"]   = TimeFilterMs
  Macros["dateFilter"]      = DateFilter
  Macros["dateTimeFilter"]  = DateTimeFilter
  Macros["dt"]              = DateTimeFilter
  Macros["timeInterval"]    = TimeInterval
  Macros["timeInterval_ms"] = TimeIntervalMs
  Macros["interval_s"]      = IntervalSeconds
  ```
- [x] 2.5 Add `var _ = sqlutil.ErrorBadArgumentCount` next to the imports as a build-time guard against the upstream sentinel being renamed/removed.

## 3. Stub alignment (carries the `conditionalAll` baseline)

- [x] 3.1 Update `pkg/plugin/macros_registry.go::Stub` to return `("1=1", nil)` — matches the fork at `0f83082`. The earlier C5 stub returned `""` and would render queries unparseable (`SELECT  FROM t`).
- [x] 3.2 Update `pkg/plugin/interpolator_test.go::TestInterpolate_StubConditionalAll` to expect `"SELECT 1=1 FROM t"`. Same place in the file.

## 4. Tests

- [x] 4.1 Add `pkg/plugin/macros_clickhouse_test.go`. Port from the fork's `macros_test.go:15-232`:
  - `TestTimeToDate` (toDate format).
  - `TestTimeToDateTime` (unix seconds).
  - `TestTimeToDateTime64` (unix millis).
  - `TestMacroFromTimeFilter`, `TestMacroToTimeFilter`, `TestMacroFromTimeFilterMs`, `TestMacroToTimeFilterMs` — stateless, zero-arg.
  - `TestMacroDateFilter`, `TestMacroDateTimeFilter` — required-args.
  - `TestMacroTimeInterval`, `TestMacroTimeIntervalMs` — Interval → seconds / milliseconds floor.
  - `TestMacroIntervalSeconds` — Interval → seconds floor.
- [x] 4.2 Add arg-count error-path coverage for each PK-lookup macro: `len(args) > 1` returns `ErrorBadArgumentCount`.
- [x] 4.3 Add a positive PK-lookup case for `TimeFilter` (and one for `TimeInterval`) by pre-seeding `MetadataProvider.pkCache` for a known `(database, table)`, building SQL that resolves via `cte.GetMacroCTEs`, and asserting the emitted column matches the cached PK.
- [x] 4.4 Confirm the existing C5 test `TestInterpolate_LongerMacroNamesMatchFirst` still passes — it's the regression net for `timeFilter` vs `timeFilter_ms` (and the other length-pair clashes the fork's registry has).

## 5. Quality gates

- [x] 5.1 `go build ./...` clean.
- [x] 5.2 `go vet ./...` clean.
- [x] 5.3 `golangci-lint run --new-from-rev=HEAD` clean.
- [x] 5.4 `go test -race ./...` green.
- [x] 5.5 `npm run typecheck && npm run lint && npm run test:ci` green.
- [x] 5.6 Playwright e2e — runs at coordinated-set verification (C5 + C6 + C7 together produce the full query path).

## 6. Commit

- [x] 6.1 Single commit including code + design + tasks + specs.
- [x] 6.2 Commit message: `pkg/plugin: port ClickHouse time/date macros (C6)`. Body summarises the eleven macros + the `dt` alias + the Stub-aligns-to-fork fix.
