# Ad-hoc filter value preload: topK + query guardrails

Jira: [HDX-11854](https://hydrolix.atlassian.net/browse/HDX-11854)

## Why

On dashboards with a 30–90 day time range, the ad-hoc filter value dropdown
for high-cardinality fields (e.g. `clientIP`) on raw tables loads extremely
slowly or never: the preload query aggregates every distinct value in the
range (`GROUP BY … ORDER BY count DESC`) before its `LIMIT 100` applies, and
nothing bounds its execution time, rows read, or memory. Grafana never passes
the user's typed text to `getTagValues`, so the one preload query is the only
chance to return useful values — it must be fast and degrade gracefully.

## What Changes

- Replace the ad-hoc value preload query shape (`AD_HOC_VALUE_QUERY` in
  `src/constants.ts`) with a `topK`-based query, keeping approximate
  popularity ordering with bounded aggregation memory.
- Preserve the synthetic `__empty__` / `__null__` dropdown entries under
  the new query shape (`__null__` becomes type-gated: offered for Nullable
  columns and Nullable-valued map keys).
- Guard all frontend metadata queries issued through the metadata provider
  (ad-hoc value preload, map-key discovery, schema and autocomplete lookups)
  with a Hydrolix-native execution-time circuit breaker, plus "break"
  overflow semantics embedded in the preload SQL so partial results are
  returned where the engine supports it.
- Bound the preload time window to the trailing 24h of the dashboard range,
  rounded to 5-minute boundaries via the plugin's existing `round`
  mechanism (repeatable SQL text), with a Hydrolix-native timerange setting as
  server-side enforcement.
- Datasource-level `querySettings` can tighten (never loosen) the metadata
  execution-time guardrail — the minimal value wins; existing mechanism, no
  new config UI.
- Non-breaking: no `plugin.json` changes, no query model changes, no backend
  changes; existing dashboards keep working on Grafana ≥ 10.
- Test coverage: frontend unit tests plus Playwright e2e for the dropdown
  behavior and for runtime guardrail application (intercepted request
  payload, rounded-SQL stability via UUID-tagged `query_log` lookup,
  slow-source tolerance just under the breaker budget).

## Capabilities

### New Capabilities

- `adhoc-value-preload`: how the plugin populates ad-hoc filter key/value
  dropdowns — the value preload query shape (topK, LIMIT semantics, array and
  map column handling, synthetic empty/null entries) and the guardrail
  settings applied to metadata queries so they return partial results within
  bounded time instead of hanging.

### Modified Capabilities

None — existing specs (`hdx-adhoc-filter-macro-secure`, `hdx-query-models`,
etc.) cover the backend macro and models; their requirements do not change.

## Impact

- `src/constants.ts` — `AD_HOC_VALUE_QUERY` template; new guardrail default
  constants.
- `src/ast.ts` — `getColumnValuesStatement` (template substitution only).
- `src/datasource.ts` — `getTagValues` response handling for synthetic
  empty/null entries.
- `src/editor/metadataProvider.ts` — `getQueryRunner` gains guardrail
  `querySettings` on metadata targets.
- No Go backend changes: guardrails ride the existing
  `querySettings` → session settings path in `driver.go`.
- Risk: `topK` results are approximate and aggregate functions skip NULLs —
  both handled in design; dropdown remains best-effort by definition.
