## Context

The interpolation entry point is `HdxInterpolator.Interpolate` (`pkg/plugin/interpolator.go`), constructed as `NewHdxInterpolator(NewMetadataProvider(wrapper), Macros)` in `pkg/plugin/hdx_sqlds.go`. `Macros` (`pkg/plugin/macros_registry.go`) maps macro names to `MacroFunc`s; dispatch handles regex site matching (only real `$__` call sites, not string literals), longest-name-first, and escaped `$$__macro`. Several macros call the `MetadataProvider` — `AdHocFilterMacro` → `GetKeys` → `QueryKeys`, and the time macros → `GetPK` → `QueryPK`.

Current tests call macro functions directly (`macros_adhoc_test.go`, `macros_clickhouse_test.go`), and `pkg/api/routes_test.go` drives `/interpolate` with a `stubInterpolator` — the real pipeline is never run end-to-end. Reusable scaffolding: `nopMetadataDS` (panics if a schema query escapes the cache), `preseededProvider(cte, schema)` (seeds `keyCache`), and the `pkCache.Set(...)` pattern for PK lookups.

## Goals / Non-Goals

**Goals:**
- Exercise `interpolate()` on realistic SQL through the real `Macros` registry with a stubbed metadata layer, asserting the rewritten SQL.
- Cover the dispatch behaviors (unknown macro left in place, escaped macro), the adHoc+time combination, the WITH-CTE adHoc path, and a PK-triggering time macro.
- Cover the `/interpolate` route against the real interpolator, not a stub.

**Non-Goals:**
- Any production code change; this is test-only.
- New e2e/Playwright coverage (macros remain isolated there — separate gap).
- Hitting a real ClickHouse; the metadata layer is stubbed via pre-seeded caches so `nopMetadataDS` is never queried.

## Decisions

**D1 — Drive the pipeline via a real interpolator with pre-seeded caches.** Build `NewHdxInterpolator(p, Macros)` where `p = NewMetadataProvider(nopMetadataDS{})` with `keyCache`/`pkCache` pre-seeded for the fixtures. Rationale: exercises real dispatch + macro + metadata-resolution glue while staying hermetic and fast; `nopMetadataDS` panicking on a cache miss doubles as an assertion that the seeded key is the one actually looked up.
- _Alternative considered:_ use `fakeMetadataDS` returning canned frames. Reasonable, but pre-seeding the cache is simpler and pins the exact lookup key (important for the WITH-CTE case, where the key is the resolved subquery string).

**D2 — Assert on substrings / normalized fragments, not brittle whole-SQL equality where a macro embeds time bounds.** Time macros interpolate concrete `TimeRange` values; assert the stable structural fragments (column comparisons, `status = 'active'`, resolved PK column) rather than exact timestamps. Rationale: keeps tests robust to formatting while still proving the seam. Pin exact strings only where output is deterministic (adHoc conditions, unknown-macro passthrough, escaped-macro).

**D3 — Route integration test wires the real interpolator.** In `pkg/api`, register `Routes(ds)` (or invoke the `/interpolate` handler) with `ds.Interpolator` set to a real `HdxInterpolator.Interpolate` over a stubbed metadata layer, then issue an HTTP request with a macro-bearing query and assert the response body. Rationale: covers the route↔interpolator seam the existing stub test cannot. If wiring a full `*sqlds.SQLDatasource` in `pkg/api` is impractical from that package, drive the exported handler with the real interpolator injected directly.
- _Alternative considered:_ leave route coverage to the stub test only. Rejected — the audit specifically flagged the route↔real-pipeline seam as untested.

## Risks / Trade-offs

- **[Time-macro assertions are flaky if they pin exact timestamps]** → Pin time ranges explicitly in the fixture and assert structural fragments / resolved column names, not rendered instants (D2). Proving signal: repeated `go test -race` runs are stable.
- **[A pre-seeded cache key drifts from what the macro actually computes (e.g. the resolved WITH-CTE subquery string)]** → Derive the key in-test from `cte.GetMacroCTEs` (as the resolve-adhoc-with-cte test does) rather than hardcoding, so the seed always matches the lookup. Proving signal: `nopMetadataDS` never panics (a miss would).
- **[Cross-package wiring in `pkg/api` pulls in more than a test should]** → Prefer injecting a real `HdxInterpolator.Interpolate` into the handler over standing up a full datasource; keep the test within what `pkg/api` already imports. Proving signal: the route test compiles without new non-test dependencies.
