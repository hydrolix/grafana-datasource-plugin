## Why

The fork's `macros.go` at `0f83082` defines eleven ClickHouse-specific time/date/interval macros that the plugin's dashboards expand at query time: `$__fromTimeFilter`, `$__toTimeFilter`, `$__fromTimeFilterMs`, `$__toTimeFilterMs`, `$__timeFilter`, `$__timeFilterMs`, `$__dateFilter`, `$__dateTimeFilter`, `$__timeInterval`, `$__timeIntervalMs`, `$__intervalSeconds`. They emit ClickHouse SQL fragments (`fromUnixTimestamp64Milli(...)`, `toStartOfInterval(toDateTime(...), INTERVAL N second)`, column-range comparisons like `col >= X AND col <= Y`, etc.).

After C2 pins the plugin to sqlds at `ef925e1`, those macros no longer ship inside sqlds. C5 establishes the `MacroFunc` signature and the `Macros` registry; this change populates the registry with the time/date/interval macros. Until C6 lands alongside C5, panel queries that use any time macro fail expansion.

Five of the macros (`TimeFilter`, `TimeFilterMs`, `TimeInterval`, `TimeIntervalMs`, and `AdHocFilter`) need primary-key lookup when the user omits the column argument — they call into `MetadataProvider` (defined in C7) via the `getPK` helper. That coupling makes C6 depend on C7 at the file level: both ship in the same merge unit. The remaining six macros (`FromTimeFilter`, `ToTimeFilter`, `FromTimeFilterMs`, `ToTimeFilterMs`, `DateFilter`, `DateTimeFilter`, `IntervalSeconds`) are stateless and use neither `MetadataProvider` nor `getPK`; they ship as part of this change unconditionally.

## What Changes

- Add `pkg/plugin/macros_clickhouse.go` containing the eleven `MacroFunc` implementations ported verbatim from the fork's `macros.go:45-189`:
  - **Stateless** (no `*MetadataProvider` use): `FromTimeFilter`, `ToTimeFilter`, `FromTimeFilterMs`, `ToTimeFilterMs`, `DateFilter`, `DateTimeFilter`, `IntervalSeconds`.
  - **PK-lookup-on-omitted-column** (resolve a column via `getPK` from C7 when the macro argument is empty): `TimeFilter`, `TimeFilterMs`, `TimeInterval`, `TimeIntervalMs`.
- Add `pkg/plugin/macros_time.go` with the `timeToDate`, `timeToDateTime`, `timeToDateTime64` helpers (also ported from the fork).
- Add `init()` block that registers all eleven macros into the package-level `Macros` map established by C5: `Macros["fromTimeFilter"] = FromTimeFilter`, etc.
- Add `pkg/plugin/macros_clickhouse_test.go` with the fork's `macros_test.go` cases for the eleven macros ported verbatim. Includes time-range cases (epoch boundaries, sub-second intervals), arg-count error paths, column-omitted-with-PK-lookup paths.
- No changes to `pkg/plugin/hdx_sqlds.go` from C5 — wiring already happens via the package-level `Macros` registry.
- Playwright e2e coverage unchanged in isolation; runs at the end of the C2-C7 merge window.

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics. Emitted SQL fragments are byte-for-byte identical to the fork.

## Capabilities

### New Capabilities

- `hdx-clickhouse-time-date-macros`: Plugin-owned ClickHouse time/date/interval macros (`$__fromTimeFilter`, `$__toTimeFilter`, `$__fromTimeFilterMs`, `$__toTimeFilterMs`, `$__timeFilter`, `$__timeFilterMs`, `$__dateFilter`, `$__dateTimeFilter`, `$__timeInterval`, `$__timeIntervalMs`, `$__intervalSeconds`). Registered into the `hdx-interpolator` macro registry via `init()`.

### Modified Capabilities

<!-- C5's hdx-interpolator's Macros registry receives entries from this change; the registry surface itself is unchanged. -->

## Impact

- **Frontend**: none.
- **Backend (Go)**: new files `pkg/plugin/macros_clickhouse.go`, `pkg/plugin/macros_time.go`, paired `_test.go` files.
- **Tests**: ported macro tests for the eleven macros; existing `Interpolator` tests from C5 exercise the dispatch path.
- **Dependencies**: none added or removed beyond what C5 brought in (`clickhouse-sql-parser` for `parser.Pos`).
- **User-visible**: none. Existing dashboards using time macros expand to identical SQL.
- **Security**: no surface change. Stateless macros emit fully-typed ClickHouse expressions (`fromUnixTimestamp64Milli(<number>)`) with no user-supplied strings interpolated; PK-lookup macros emit `<column> >= X AND <column> <= Y` where the column is from the macro argument or from `MetadataProvider`'s schema lookup (no user-supplied free-form string).
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2) and `plugin-hdx-interpolator` (C5) for the dispatch infrastructure, and on `plugin-adhoc-filter-macro-secure` (C7) for the `MetadataProvider` type and `getPK` helper that the PK-lookup macros call into. Ships in the same coordinated merge window as C2-C7.
