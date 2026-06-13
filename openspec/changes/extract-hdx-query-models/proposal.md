## Why

The plugin imports five data-shape types from `github.com/hydrolix/sqlds/v5`: `sqlds.HDXQuery` and `sqlds.AdHocFilter` (`pkg/api/routes.go`), and `models.PluginSettings`, `models.QuerySetting`, `models.NewPluginSettings` from the fork's `models` sub-package (`pkg/plugin/driver.go`, `pkg/plugin/driver_test.go`, `pkg/plugin/dssuit_test.go`).

These are pure data shapes — datasource configuration JSON, query JSON, validation helpers. They have no sqlds interface conformance and no runtime behaviour. They live in the fork only because they were colocated with the fork's `HydrolixDatasource` type. The shapes are plugin-owned schemas; the fork has no business being their canonical home.

This change moves the shapes into the plugin without changing the sqlds pin. The plugin keeps importing the fork for `HydrolixDatasource`, `Connector`, and macro machinery; the migration to the extension-points sqlds revision is the subsequent change. Shrinking the fork's surface area to "behaviour only" first simplifies every later change in the migration sequence.

This is the first ship-now step in the multi-change retirement of `github.com/hydrolix/sqlds/v5`. It carries no upstream-release dependency and no behaviour drift, so it can land independently of the rest.

## What Changes

- Add `pkg/plugin/models/` package containing `HdxQuery` (renamed from `HDXQuery` for naming parity with the plugin's TypeScript `HdxQuery`), `AdHocFilter`, `PluginSettings`, `QuerySetting`, `NewPluginSettings`, and the validation helpers (`IsValid`, `SetDefaults`, error sentinels).
- Update `pkg/api/routes.go` to consume `models.HdxQuery` and `models.AdHocFilter` from the plugin-local package.
- Update `pkg/plugin/driver.go` (3 sites), `pkg/plugin/driver_test.go`, `pkg/plugin/dssuit_test.go` to consume `PluginSettings`, `QuerySetting`, `NewPluginSettings` from the plugin-local package.
- Go unit-test parity with the fork's existing `models/settings_test.go` coverage of `IsValid` and `SetDefaults`.
- Playwright e2e coverage unchanged.
- `go.mod` unchanged. `github.com/hydrolix/sqlds/v5` stays pinned at its current version; only the unused `models` sub-package import is dropped.

Not breaking for the plugin's HTTP wire format, frontend, or dashboard surfaces. JSON field names are preserved verbatim.

## Capabilities

### New Capabilities

- `hdx-query-models`: Plugin-local data shapes for the Hydrolix datasource — query (`HdxQuery`), ad-hoc filter (`AdHocFilter`), datasource settings (`PluginSettings`, `QuerySetting`), validation helpers, and JSON-unmarshalling constructors. Lives in `pkg/plugin/models/`.

### Modified Capabilities

<!-- None — the data shapes had no spec while they lived in the fork. -->

## Impact

- **Frontend**: none. `src/types.ts`'s existing `HdxQuery` shape is the authoritative TypeScript counterpart; the Go side now matches its name.
- **Backend (Go)**: new package `pkg/plugin/models/`; four files updated to import the new path (`pkg/api/routes.go`, `pkg/plugin/driver.go`, `pkg/plugin/driver_test.go`, `pkg/plugin/dssuit_test.go`).
- **Tests**: Go unit-test parity with the fork's `models/settings_test.go`; no e2e impact.
- **Dependencies**: none added or removed. Fork stays at its current pin; only the `models` sub-package falls out of plugin use.
- **User-visible**: none. JSON wire shapes — including the `HdxQuery` JSON tag names — are byte-for-byte identical.
- **Security**: no surface change. Validation logic moves verbatim.
- **Sequencing**: independent. No upstream dependency, no other change must land first or after.
