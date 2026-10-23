## 1. Backend: `""` is a value; the macro path raises the error

- [x] 1.1 Revert `QueryPK` and `GetPK` in `pkg/plugin/metadata.go` to their HEAD behavior: the empty cell yields `("", nil)` and is stored in `pkCache` like any other result; the empty frame still yields `ErrPrimaryKeyNotFound`; errors are never stored.
- [x] 1.2 Make `ErrPrimaryKeyEmpty` a plain `errors.New` sentinel with no error source, documenting why the source must live on the wrapper (SDK `ErrorWithSource.Is` compares only the source).
- [x] 1.3 In `getPK` (`pkg/plugin/metadata.go`), when `GetPK` returns `""`, return `backend.DownstreamError(fmt.Errorf("%w: %s.%s; …", ErrPrimaryKeyEmpty, db, table))` with a hint that the time column must be passed as the macro's argument — wording valid for all four PK-lookup macros.
- [x] 1.4 Revert `resolveColumnArg` in `pkg/plugin/macros_clickhouse.go` to HEAD's one-line delegation to `getPK`; remove the unreachable `column == ""` block and the duplicated hint.

## 2. Frontend: single template with a `${timeFilter}` slot

- [x] 2.1 In `src/constants.ts`, remove `AD_HOC_VALUE_QUERY_NO_TIME_FILTER`, `AD_HOC_MAP_KEY_QUERY_NO_TIME_FILTER` and `AD_HOC_QUERY_BREAKER_SETTINGS`; restore the single `AD_HOC_QUERY_GUARDRAIL_SETTINGS` suffix and give `AD_HOC_VALUE_QUERY` / `AD_HOC_MAP_KEY_QUERY` a `${timeFilter}` slot in place of the hardcoded conjunct.
- [x] 2.2 In `src/ast.ts`, make `getColumnValuesStatement` and `getColumnKeysForMapStatement` take a required `timeColumn: string` and fill the slot with `$__timeFilter(<col>) AND ` / `$__timeFilter() AND ` when non-empty, `""` otherwise; the keyed output must be byte-identical to HEAD.
- [x] 2.3 Fill every slot (`${column}`, `${table}`, `${timeColumn}`, `${timeFilter}`, `${condition}`) with a function replacer so `$`-sequences in inserted text are literal.
- [x] 2.4 Update the constants comment: the suffix is shared because `hdx_query_max_timerange_sec` derives its range from the primary-column predicate and is inert without one.

## 3. Frontend: preload paths

- [x] 3.1 In `src/editor/metadataProvider.ts` `primaryKey()`, return `Promise<string | undefined>`: reject (not memoized) on a response carrying `errors`; resolve and memoize `undefined` on no row, as at HEAD; `""` stays the memoized keyless value. Use `fillSlots` for `AD_HOC_KEY_QUERY` too.
- [x] 3.2 Remove `asTimeColumn` from `src/datasource.ts`; `getTagValues` builds the statement from the resolved string and passes `adHocPreloadRange(...)` only when it is non-empty. A rejected lookup returns `[]` as before the change.
- [x] 3.3 In `getTagKeys`, resolve the primary key once (alongside `tableKeys`) and pass it into `getTagKeysForMap`, which takes `timeColumn: string | undefined`: only `""` selects the keyless statement with no range; `undefined` (lookup failed) keeps the backend-resolved keyed statement and capped range, as at HEAD. `getTagValues` resolves it in the same `Promise.all` as `tableKeys`.
- [x] 3.4 Keep the `adHocPreloadRange` doc comment accurate for the final shape (keyless preloads skip it; `ZERO_TIME_RANGE` is reached by design because the statement carries no time macro).
- [x] 3.5 Update `.claude/CLAUDE.md`: `ZERO_TIME_RANGE` is valid for any statement that carries no time macro — the `system.*` / `DESCRIBE` lookups and the keyless ad-hoc preloads — and `adHocPreloadRange()` applies to keyed tables only.

## 4. Go unit tests

- [x] 4.1 `QueryPK` returns `("", nil)` for a frame whose single cell is the empty string, and `ErrPrimaryKeyNotFound` for an empty frame.
- [x] 4.2 `GetPK` called twice for a keyless table invokes `metadataDS.QueryData` exactly once and returns `("", nil)` both times.
- [x] 4.3 `GetPK` called twice when `metadataDS` returns a generic `backend.DownstreamError` invokes `QueryData` twice, returns that error both times, and `errors.Is(err, ErrPrimaryKeyEmpty)` is false.
- [x] 4.4 Each of `TimeFilter`, `TimeFilterMs`, `TimeInterval`, `TimeIntervalMs` on a keyless table returns an empty fragment; `errors.Is(err, ErrPrimaryKeyEmpty)` and `backend.IsDownstreamError(err)` are true; the message names `db.table` and the macro-argument hint.
- [x] 4.5 A zero-arg macro over a table whose PK lookup fails with a generic downstream error propagates it, `errors.Is(err, ErrPrimaryKeyEmpty)` is false, and the message has no hint.
- [x] 4.6 Replace `macroPosFor` with the existing parse → `GetMacroCTEs` pattern used elsewhere in the test file, or make the existing sites use the helper — no fifth copy.
- [x] 4.7 Run the Go suite with `-race`.

## 5. Frontend unit tests

- [x] 5.1 `getColumnValuesStatement` with `""` emits no `$__timeFilter`, keeps `topK(100)`, `$__adHocFilter()` and the full guardrail suffix (both settings), and no `GROUP BY` / `ORDER BY`.
- [x] 5.2 `getColumnValuesStatement` with `"ts"` emits the HEAD statement byte-for-byte (exact-string assertion against the pre-change template).
- [x] 5.3 `getColumnKeysForMapStatement` covers the same two branches, keyed output byte-identical to HEAD.
- [x] 5.4 A condition containing `$'` and a map-key column containing `$ref` are inserted verbatim; the `SETTINGS` suffix appears exactly once.
- [x] 5.5 `getTagValues` on a keyless table returns the response values, issues a query with no `$__timeFilter`, and the request range is `ZERO_TIME_RANGE`.
- [x] 5.6 `getTagValues` on a keyless table still applies the synthetic `__empty__` / `__null__` gating.
- [x] 5.7 `getTagValues` when the PK lookup rejects returns `[]` and issues no preload query.
- [x] 5.8 `metadataProvider.primaryKey()` rejects and does not memoize a response carrying `errors` (a second call re-issues the lookup); a no-row response memoizes `undefined` (one lookup); an explicit `""` cell memoizes `""`. `getTagKeys` on a rejected lookup issues the keyed `$__timeFilter()` statement with the capped range.
- [x] 5.9 `getTagKeys` with three Map columns invokes `primaryKey()` once; each `getTagKeysForMap` is keyless (no range) for `""` and capped for `"ts"`.
- [x] 5.10 Keyed-table preload still receives the capped trailing-24h range and `round = "5m"`.
- [x] 5.11 Fold `setupKeylessMock` into the existing preload mock builders in `src/datasource.test.ts` (one parameterised helper) and drop the `undefined`-shape test cases.

## 6. E2E

- [x] 6.1 Add a keyless table to `testdata/containers/initdb.sql` — `ENGINE = MergeTree() ORDER BY tuple()`, which leaves `system.tables.primary_key` empty — seeded with a small fixed row set including a Map column and a low-cardinality string column.
- [x] 6.2 Update `tests/adHocKeyless.spec.ts`: assert the keyless statements carry the full guardrail suffix (including `hdx_query_max_timerange_sec`), use `captureSqls` from `tests/helpers.ts` instead of a hand-rolled body parser, and keep the `[data-value=""]` react-select entry point.
- [x] 6.3 Assert map-key discovery on the keyless table returns the `attrs['<key>']` accessors instead of erroring.
- [x] 6.4 Keep the existing keyed-table ad-hoc specs green with pinned time ranges (no reliance on the system clock).

## 7. Verification

- [x] 7.1 Run the pre-PR gates: `npm run typecheck`, `npm run lint`, `npm run test:ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`, `npm run build`.
- [x] 7.2 Run the e2e suite via `e2e-dev` against a ClickHouse container that has the keyless fixture (reset the volume or apply the DDL to the running container first).
- [x] 7.3 Confirm no behavior change for keyed tables by diffing the interpolated SQL for an existing ad-hoc dashboard before and after.
- [x] 7.4 Record that `hdx_query_max_timerange_sec` on a predicate-free statement has been checked against the Hydrolix documentation only; the dev stack runs stock ClickHouse and cannot exercise `hdx_*` settings, so the first run against a real Hydrolix query head is the proving signal.
