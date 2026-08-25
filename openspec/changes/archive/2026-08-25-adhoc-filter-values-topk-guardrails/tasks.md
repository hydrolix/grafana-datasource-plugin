# Tasks: adhoc-filter-values-topk-guardrails

## 1. Spike — verify query shape and settings (design D1/D2 preconditions)

      _Verified on the real cluster 2026-08-24: flat
      `arrayJoin(topK(100)(col))` form is valid; driver-level
      `hdx_query_max_execution_time` + SQL-level
      `SETTINGS timeout_overflow_mode='break'` returns partial results
      (values at 3s over 90 days; empty-but-successful result at 1s)._

- [x] 1.1 On the dev ClickHouse stack, verify the flat single-column
      template (confirmed on Hydrolix, including the `SETTINGS` suffix);
      fall back to the design D1 subquery form if stock ClickHouse rejects
      it, and lock the template shape
- [x] 1.2 Verify `$__timeFilter` / `$__adHocFilter()` expansion of the
      locked template (including the trailing `SETTINGS` clause) through the
      `/interpolate` backend resource; if resolution fails, apply the D1
      fallback (explicit argument or outer-query filter)
- [ ] 1.3 Close the remaining cluster checks: verify
      `hdx_query_max_timerange_sec = 87000` in the `SETTINGS` clause (a
      capped >24h-range preload must pass; a deliberately uncapped >24h+slack
      range must be cancelled); record the exact rejection error for
      `timeout_overflow_mode` as a URL parameter, and alias precedence when
      both `hdx_query_max_execution_time` and `max_execution_time` are
      supplied with different values

      _Deliberately left open at archive time (2026-08-25): these require the
      production Hydrolix cluster, which no CI or dev-stack path can reach —
      stock ClickHouse accepts `hdx_*` settings via `custom_settings_prefixes`
      but does not enforce them. The design's load-bearing cluster behavior
      (flat topK form, driver-channel breaker + SQL-level `break` returning
      partial results) was already confirmed on the real cluster on
      2026-08-24; what remains is confirmatory. Track it on the ticket, not
      here._
- [x] 1.4 On the dev ClickHouse stack, check whether stock ClickHouse
      accepts `hdx_` settings on both channels — `hdx_query_max_execution_time`
      as a session setting and `hdx_query_max_timerange_sec` in the SQL
      `SETTINGS` clause — or needs `custom_settings_prefixes='hdx_'` in the
      dev config, so guardrail injection doesn't break the dev stack

## 2. Query template and constants

- [x] 2.1 Replace `AD_HOC_VALUE_QUERY` in `src/constants.ts` with the
      verified flat single-column topK template ending in
      `SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec
      = 87000`; add the same `SETTINGS` suffix to `AD_HOC_MAP_KEY_QUERY`;
      add a named constant for the topK size (100)
- [x] 2.2 Add the guardrail constants to `src/constants.ts`: breaker
      (`hdx_query_max_execution_time = 10`), lookback cap (86400s), round
      interval (`"5m"`), and the timerange setting value derived as
      cap + 2×round-interval (= 87000) so the numbers cannot drift
- [x] 2.3 Update `getColumnValuesStatement` in `src/ast.ts` for the new
      template placeholders; remove the stale commented-out variants

## 3. Response handling in getTagValues

- [x] 3.1 In `getTagValues` (`src/datasource.ts`), gate `SYNTHETIC_NULL` on
      the column type from the already-fetched `tableKeys` metadata (type is
      Nullable per the existing `NULLABLE_TYPES` handling), replacing the
      old null-entries-in-list derivation; response parsing stays field-0
      (`getValuesFromResponse` unchanged)
- [x] 3.2 Keep `SYNTHETIC_EMPTY` derivation from `''` unchanged and confirm
      an empty successful response yields an empty suggestion list without
      an error
- [x] 3.3 Cap the preload range in `getTagValues` and `getTagKeysForMap`:
      pass `{from: max(range.from, range.to − 86400s), to: range.to}` to
      `executeQuery` (template keeps plain `$__timeFilter`)

## 4. Guardrail injection in the metadata query runner

- [x] 4.1 In `getQueryRunner` (`src/editor/metadataProvider.ts`), inject the
      breaker default into the metadata target's `querySettings`
- [x] 4.2 Suppress injection when
      `instanceSettings.jsonData.querySettings` already contain
      `hdx_query_max_execution_time` **or** its alias `max_execution_time`
      (design D3 — alias pair treated as one setting)
- [x] 4.3 Set `round: "5m"` on metadata query targets in `getQueryRunner`
      (replacing today's `round: ""`) so the backend's existing
      `roundTimeRange` path snaps the range before macro expansion

## 5. Unit tests (frontend)

- [x] 5.1 `src/ast.test.ts` (or equivalent): `getColumnValuesStatement`
      renders topK template for plain, `arrayJoin(...)`, and `map['key']`
      columns; no `GROUP BY` / `ORDER BY count` in output
- [x] 5.2 `src/datasource.test.ts`: `getTagValues` maps `''` →
      `SYNTHETIC_EMPTY`; Nullable column type → `SYNTHETIC_NULL` appended
      (even when no returned value is NULL); non-Nullable type → no
      `SYNTHETIC_NULL`; empty response → empty list, no error
- [x] 5.3 metadataProvider tests: metadata targets carry the breaker and no
      `timeout_overflow_mode` in `querySettings`; the rendered value-preload
      and map-key SQL carry the `SETTINGS timeout_overflow_mode = 'break'`
      suffix; DS-level `hdx_query_max_execution_time` suppresses injection;
      DS-level `max_execution_time` (alias) also suppresses it; unrelated
      DS-level settings suppress nothing
- [x] 5.4 Unit tests for the time window: 90-day range → `from = to −
      86400s`; 6-hour range → untouched; metadata targets carry
      `round: "5m"`; rendered templates carry both `SETTINGS` entries
- [x] 5.5 Run `npm run typecheck`, `npm run lint`, `npm run test:ci` (verify
      test count ≠ 0)

## 6. E2E (behavior-affecting change)

- [x] 6.1 Seed a ClickHouse fixture with a skewed-frequency column
      (dominant value + long tail, plus empty-string rows, a Nullable
      column, and a non-Nullable column), pinned time range spanning more
      than 24h with one distinctive value occurring only outside the
      trailing-24h window
- [x] 6.2 Playwright test via the `e2e-dev` skill: open the ad-hoc value
      dropdown, assert the dominant value is present, `__empty__` appears,
      `__null__` appears for the Nullable column and is absent for the
      non-Nullable column, the outside-window value is absent from
      suggestions but works as a typed filter, and the dropdown populates
      within the guardrail budget
- [x] 6.3 Rebuild `dist/` cleanly (`build-plugin` skill) before the e2e run

## 7. Docs and Go-side sanity

- [x] 7.1 Update datasource config help text / README section on
      `querySettings` to document the metadata breaker default, the alias
      override (`hdx_query_max_execution_time` / `max_execution_time` are
      the same setting; either suppresses injection), that a DS-level
      override applies to all queries, the SQL-level `timeout_overflow_mode`
      / `hdx_query_max_timerange_sec` behavior, and the suggestion-window
      semantics (dropdown values come from the trailing 24h of the range)
- [x] 7.2 Run `go vet ./...`, `golangci-lint run`, `go test -race ./...`
      (no backend code change expected — confirm green)

## 8. Review adjustments (approved 2026-08-24)

- [x] 8.1 Gate `__null__` for map-access keys in `getTagValues`: resolve the
      base map column's type from `tableKeys` (strip the `['…']` suffix)
      and append `SYNTHETIC_NULL` iff the type is a
      `Map(String, Nullable(…))` variant; plain columns keep the
      `NULLABLE_TYPES` check
- [x] 8.2 Replace breaker suppression with min-wins injection in
      `getQueryRunner`: always inject, value = min(10, positive numeric
      DS-level value under `hdx_query_max_execution_time` or
      `max_execution_time`); ignore missing / non-numeric / ≤ 0 values
      (0 = unlimited must never be adopted)
- [x] 8.3 Update unit tests to the new semantics: map key on
      `Map(String, Nullable(String))` → `__null__`, on
      `Map(String, String)` → none; DS `60` → injected `10`, alias `5` →
      injected `5`, `0` → `10`, non-numeric → `10`, unrelated settings →
      `10`
- [x] 8.4 Update the README override wording: DS-level settings can lower
      but never raise the metadata timeout (minimal value wins)
- [x] 8.5 Re-run `npm run typecheck`, `npm run lint`, `npm run test:ci`

## 9. E2E runtime verification of guardrails (approved 2026-08-25, design D7)

- [x] 9.1 `tests/helpers.ts`: add a preload interceptor that (a) captures the
      full `/api/ds/query` POST body and (b) rewrites the outgoing `rawSql`
      to append a SQL comment with a fresh UUID per request
      (`route.fetch({postData})` + fulfill), returning captured
      body↔UUID pairs; add a `system.query_log` lookup helper (ClickHouse
      HTTP, `SYSTEM FLUSH LOGS` + poll, match by UUID, exclude the lookup
      query itself). Prefer a `-- e2e:<uuid>` trailing comment; fall back to
      `/* e2e:<uuid> */` if the backend AST/macro path chokes on `--`
- [x] 9.2 e2e — guardrail transmission: open the value dropdown, assert the
      intercepted preload payload carries
      `querySettings: hdx_query_max_execution_time = "10"`, no
      `timeout_overflow_mode` in `querySettings`, `round: "5m"`, `from`/`to`
      capped to the trailing 24h, and the
      `SETTINGS timeout_overflow_mode = 'break', hdx_query_max_timerange_sec
      = 87000` suffix in `rawSql`
- [x] 9.3 e2e — rounding stability: dashboard with a `now`-relative range;
      open the dropdown twice a few seconds apart (waiting past the next
      5-minute boundary first when the opens would straddle one); fetch both
      executed queries from `query_log` by their UUIDs; assert the SQL is
      identical after stripping the UUID comments, and (nice-to-have) that
      the `toDateTime(...)` literals are multiples of 300
- [x] 9.4 e2e — slow-source tolerance: seed a dedicated slow source (e.g.
      100 in-window rows + `CREATE VIEW e2e.adhoc_slow AS SELECT … WHERE
      sleepEachRow(0.0999) = 0`, ≈9.99s per preload, just under the 10s
      breaker); separate test with a raised timeout asserting the dropdown
      populates with the expected values, elapsed ≥ ~9.9s (the preload
      genuinely waited), and no error UI; keep the main spec's 8s-budget
      assertion untouched on the fast fixture
- [x] 9.5 Run the full ad-hoc e2e suite green (fast + new specs) via the
      `e2e-dev` skill against a clean `dist/`
