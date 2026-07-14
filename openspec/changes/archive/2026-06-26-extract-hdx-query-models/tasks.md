# extract-hdx-query-models — implementation tasks

## 1. Create `pkg/plugin/models` package

- [x] 1.1 Create directory `pkg/plugin/models/`.
- [x] 1.2 Add `pkg/plugin/models/query.go` with `HdxQuery` (renamed from the fork's `HDXQuery`) and `AdHocFilter`. JSON tags identical to the fork's `interpolator.go` lines 309-328 at `hydrolix/sqlds/v5@v5.0.1`. Non-JSON fields (`TimeRange`, `Interval`, `Headers`) keep the `json:"-"` tag.
- [x] 1.3 Add `pkg/plugin/models/settings.go` with `PluginSettings`, `QuerySetting`, `NewPluginSettings`, `parseBool`, `parseUint`, and error sentinels. Content ported verbatim from `hydrolix/sqlds/v5@v5.0.1`'s `models/settings.go` modulo the package name being `package models` (already matches).
- [x] 1.4 Confirm the package imports only Grafana plugin SDK and stdlib. `grep -r 'sqlds' pkg/plugin/models` must return zero matches.

## 2. Update import sites

- [x] 2.1 *Deferred to C5*: `pkg/api/routes.go`'s `sqlds.HDXQuery` / `sqlds.AdHocFilter` references stay until C5 lifts the interpolator (whose signature requires `*sqlds.HDXQuery` at the current pin). C5 swaps both together.
- [x] 2.2 `pkg/plugin/driver.go`: import-path swap from `github.com/hydrolix/sqlds/v5/models` to `github.com/hydrolix/plugin/pkg/plugin/models`. Type references stay identical (same `PluginSettings`, `QuerySetting`, `NewPluginSettings` names).
- [x] 2.3 `pkg/plugin/driver_test.go`: import-path swap (line 14).
- [x] 2.4 `pkg/plugin/dssuit_test.go`: import-path swap (line 11).

## 3. Tests

- [x] 3.1 Add `pkg/plugin/models/settings_test.go` ported verbatim from `hydrolix/sqlds/v5@v5.0.1`'s `models/settings_test.go`. Covers `NewPluginSettings` happy path, type-coercion via `parseBool` / `parseUint`, invalid-JSON error, mandatory-field validation, `SetDefaults` behaviour.
- [x] 3.2 Add `pkg/plugin/models/query_test.go` with a small JSON round-trip test asserting `HdxQuery` and `AdHocFilter` marshal and unmarshal without losing fields. The wire format is dashboard-visible; even one missed JSON tag would break panels.

## 4. Quality gates

- [x] 4.1 `go vet ./...` — clean.
- [x] 4.2 `golangci-lint run` — clean.
- [x] 4.3 `go test -race ./...` — green.
- [x] 4.4 `npm run typecheck && npm run lint && npm test -- --ci` — green (no frontend impact expected, but run as a guard).

## 5. Commit

- [x] 5.1 Single commit containing the new `pkg/plugin/models/` package, the four import-site updates, the change's `tasks.md` + `specs/`, and the migration plan's tick on section 1's tasks.
- [x] 5.2 Commit message: `pkg/plugin/models: extract HdxQuery, AdHocFilter, PluginSettings from fork`
