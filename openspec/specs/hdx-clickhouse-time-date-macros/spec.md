# hdx-clickhouse-time-date-macros Specification

## Purpose
TBD - created by archiving change plugin-clickhouse-time-date-macros. Update Purpose after archive.
## Requirements
### Requirement: Time-conversion helpers

The plugin SHALL define three package-internal time-conversion helpers in `pkg/plugin/macros_time.go`:

- `timeToDate(t time.Time) string` — returns `toDate('YYYY-MM-DD')`.
- `timeToDateTime(t time.Time) string` — returns `toDateTime(<unix>)`.
- `timeToDateTime64(t time.Time) string` — returns `fromUnixTimestamp64Milli(<unixMilli>)`.

Output strings match the fork's byte-for-byte.

#### Scenario: timeToDate emits ISO date

- **GIVEN** `t = 2014-11-12T11:45:26.371Z`
- **WHEN** `timeToDate(t)` is called
- **THEN** the return SHALL be `"toDate('2014-11-12')"`

#### Scenario: timeToDateTime emits unix seconds

- **GIVEN** `t = time.Unix(1708430068, 0)`
- **WHEN** `timeToDateTime(t)` is called
- **THEN** the return SHALL be `"toDateTime(1708430068)"`

#### Scenario: timeToDateTime64 emits unix millis

- **GIVEN** `t = time.UnixMilli(1708430068123)`
- **WHEN** `timeToDateTime64(t)` is called
- **THEN** the return SHALL be `"fromUnixTimestamp64Milli(1708430068123)"`

### Requirement: Stateless time-range macros

The plugin SHALL define four stateless macros that read `query.TimeRange` and emit a single ClickHouse expression. They take zero arguments; passing any argument SHALL be tolerated (the macros ignore `args`).

- `FromTimeFilter` → `timeToDateTime(query.TimeRange.From)`
- `ToTimeFilter` → `timeToDateTime(query.TimeRange.To)`
- `FromTimeFilterMs` → `timeToDateTime64(query.TimeRange.From)`
- `ToTimeFilterMs` → `timeToDateTime64(query.TimeRange.To)`

#### Scenario: FromTimeFilter emits `toDateTime(<unix>)`

- **GIVEN** `query.TimeRange.From = 2014-11-12T11:45:26.371Z`
- **WHEN** `FromTimeFilter(ctx, query, [], 0, nil)` is called
- **THEN** the return SHALL be `("toDateTime(1415792726)", nil)`

#### Scenario: ToTimeFilterMs emits `fromUnixTimestamp64Milli(<unixMilli>)`

- **GIVEN** `query.TimeRange.To = 2015-11-12T11:45:26.371Z`
- **WHEN** `ToTimeFilterMs(ctx, query, [], 0, nil)` is called
- **THEN** the return SHALL be `("fromUnixTimestamp64Milli(1447328726371)", nil)`

### Requirement: Date and DateTime filter macros take explicit columns

`DateFilter` SHALL require exactly one argument (the date column). `DateTimeFilter` SHALL require exactly two arguments (date column, time column). Both emit AND-joined per-column comparisons against the time range.

#### Scenario: DateFilter emits AND-joined date comparisons

- **GIVEN** a time range from `2014-11-12` to `2015-11-12`
- **WHEN** `DateFilter(ctx, query, ["dateCol"], 0, nil)` is called
- **THEN** the return SHALL be `"dateCol >= toDate('2014-11-12') AND dateCol <= toDate('2015-11-12')"`

#### Scenario: DateTimeFilter combines date and time predicates

- **GIVEN** the same time range
- **WHEN** `DateTimeFilter(ctx, query, ["dateCol", "timeCol"], 0, nil)` is called
- **THEN** the return SHALL be `"(dateCol >= toDate('2014-11-12') AND dateCol <= toDate('2015-11-12')) AND (timeCol >= toDateTime(1415792726) AND timeCol <= toDateTime(1447328726))"`

#### Scenario: DateFilter with wrong argument count is a downstream error

- **GIVEN** `args = []` (or any length ≠ 1)
- **WHEN** `DateFilter` is invoked
- **THEN** the returned error SHALL wrap `sqlutil.ErrorBadArgumentCount`

### Requirement: PK-lookup macros resolve the column via `getPK` when omitted

The plugin SHALL define four macros that accept zero or one argument: `TimeFilter`, `TimeFilterMs`, `TimeInterval`, `TimeIntervalMs`. When `len(args) == 1 && args[0] != ""`, the macro SHALL use that column. Otherwise the macro SHALL call `getPK(ctx, query.RawSQL, pos, mdProvider, query.Headers)` to resolve the primary key for the table at `pos`. When `getPK` returns an error, the macro SHALL propagate it.

The table at `pos` is the containing SELECT's FROM — its left table, for a JOIN — followed through WITH aliases and inline subqueries, chained or nested, to the real table underneath. Names resolve as ClickHouse's old analyzer resolves them (Hydrolix runs with `enable_analyzer=0`): a SELECT's main body sees all of its WITH aliases and the scopes around it, with the nearest definition winning and an alias winning over a same-named table; an alias's body sees only the aliases defined before it in its WITH plus the scopes around that WITH, never itself or a later sibling, so such a name means a real table there; with a name repeated in one WITH, the last definition wins. When an alias or subquery on that path has no single source table (a JOIN, table function, UNION / EXCEPT / INTERSECT, or no FROM), `getPK` SHALL fail with a downstream error wrapping `cte.ErrNoSingleSource` that names the alias (or "subquery"), before any metadata query. When the primary key is a single column and some alias or subquery on the path does not select it under its own name (through `*`, `t.*`, `db.t.*` or a `COLUMNS(...)` matcher that picks it, unless an EXCEPT drops it; the column itself; or an expression over it aliased back to it), `getPK` SHALL fail with a downstream error wrapping `ErrPKNotExposed` that names the innermost such layer.

#### Scenario: TimeFilter with explicit column

- **GIVEN** `args = ["ts"]` and time range from `2014-11-12` to `2015-11-12`
- **WHEN** `TimeFilter(ctx, query, ["ts"], 0, mdProvider)` is called
- **THEN** the return SHALL be `"ts >= toDateTime(1415792726) AND ts <= toDateTime(1447328726)"`
- **AND** `mdProvider`'s schema cache SHALL NOT be consulted

#### Scenario: TimeFilter resolves column from MetadataProvider PK cache

- **GIVEN** SQL `SELECT $__timeFilter FROM mydb.events` with `pkCache["mydb_events"] = "primary_ts"`
- **WHEN** `TimeFilter` runs at the position of the macro
- **THEN** the return SHALL contain `"primary_ts >= toDateTime(...)" AND "primary_ts <= toDateTime(...)"`

#### Scenario: PK lookup follows a WITH alias to its table

- **GIVEN** SQL `WITH errors AS (SELECT * FROM sample.logs WHERE status >= 500) SELECT $__timeInterval() AS t, count() FROM errors WHERE $__timeFilter() GROUP BY t`
- **WHEN** either macro runs without a column argument
- **THEN** the primary-key lookup SHALL target `database='sample' AND table ='logs'`
- **AND** the macro SHALL expand on that table's primary key

#### Scenario: Alias body without a single source table

- **GIVEN** SQL `WITH e AS (SELECT * FROM a JOIN b ON a.k = b.k) SELECT $__timeFilter() FROM e`
- **WHEN** `TimeFilter` runs without a column argument
- **THEN** the error SHALL wrap `cte.ErrNoSingleSource`, name `` `e` ``, and be a `backend.DownstreamError`
- **AND** no metadata query SHALL be issued

#### Scenario: Alias that does not select the primary-key column

- **GIVEN** SQL `WITH e AS (SELECT timestamp AS ts, v FROM sample.logs) SELECT $__timeFilter() FROM e` and primary key `timestamp`
- **WHEN** `TimeFilter` runs without a column argument
- **THEN** the error SHALL wrap `ErrPKNotExposed`, name `` `timestamp` `` and `` `e` ``, and be a `backend.DownstreamError`

#### Scenario: Too many arguments produces a typed error

- **GIVEN** `args = ["a", "b"]`
- **WHEN** any of `TimeFilter` / `TimeFilterMs` / `TimeInterval` / `TimeIntervalMs` is called
- **THEN** the returned error SHALL wrap `sqlutil.ErrorBadArgumentCount`
- **AND** the error SHALL be a `backend.DownstreamError`

### Requirement: Interval macros floor sub-second intervals to 1

`TimeInterval` SHALL emit `toStartOfInterval(toDateTime(<col>), INTERVAL N second)` where `N = max(1, floor(query.Interval.Seconds()))`. `TimeIntervalMs` SHALL emit the millisecond-precision form: `toStartOfInterval(toDateTime64(<col>, 3), INTERVAL N millisecond)` with `N = max(1, query.Interval.Milliseconds())`. `IntervalSeconds` SHALL emit just the integer `N = max(1, floor(query.Interval.Seconds()))`.

#### Scenario: TimeInterval with 20-second Interval

- **GIVEN** `query.Interval = 20 * time.Second` and `args = ["col"]`
- **WHEN** `TimeInterval` runs
- **THEN** the return SHALL be `"toStartOfInterval(toDateTime(col), INTERVAL 20 second)"`

#### Scenario: TimeIntervalMs with 20-second Interval

- **GIVEN** `query.Interval = 20 * time.Second` and `args = ["col"]`
- **WHEN** `TimeIntervalMs` runs
- **THEN** the return SHALL be `"toStartOfInterval(toDateTime64(col, 3), INTERVAL 20000 millisecond)"`

#### Scenario: IntervalSeconds returns floored seconds

- **GIVEN** `query.Interval = 20 * time.Second`
- **WHEN** `IntervalSeconds` runs
- **THEN** the return SHALL be `"20"`

#### Scenario: Sub-second interval floors to 1

- **GIVEN** `query.Interval = 500 * time.Millisecond`
- **WHEN** `IntervalSeconds` runs
- **THEN** the return SHALL be `"1"`

### Requirement: Registry populated via `init()` with fork-faithful names

The package SHALL run an `init()` block in `pkg/plugin/macros_clickhouse.go` that registers the twelve dashboard-visible names (eleven distinct functions; `dt` is an alias for `DateTimeFilter`):

| Registered name      | Function          |
| -------------------- | ----------------- |
| `fromTime`           | FromTimeFilter    |
| `toTime`             | ToTimeFilter      |
| `fromTime_ms`        | FromTimeFilterMs  |
| `toTime_ms`          | ToTimeFilterMs    |
| `timeFilter`         | TimeFilter        |
| `timeFilter_ms`      | TimeFilterMs      |
| `dateFilter`         | DateFilter        |
| `dateTimeFilter`     | DateTimeFilter    |
| `dt`                 | DateTimeFilter    |
| `timeInterval`       | TimeInterval      |
| `timeInterval_ms`    | TimeIntervalMs    |
| `interval_s`         | IntervalSeconds   |

#### Scenario: Registry contains every fork-known name

- **GIVEN** the C6 plugin build
- **WHEN** `init()` has run
- **THEN** every name in the table above SHALL be present in `Macros`
- **AND** every name SHALL map to the function listed alongside it

### Requirement: `Stub` returns `1=1` matching the fork's behaviour

The `Stub` function in `pkg/plugin/macros_registry.go` SHALL return `("1=1", nil)`. The earlier C5 stub returned `""`, which left invalid SQL (`SELECT  FROM t`) when expanded; the fork returns `"1=1"` so the rewritten query stays parseable until a real `conditionalAll` lands.

#### Scenario: Stub returns 1=1

- **GIVEN** the C6 plugin build
- **WHEN** `Stub(ctx, query, args, pos, mdProvider)` is called with any arguments
- **THEN** the return SHALL be `("1=1", nil)`

