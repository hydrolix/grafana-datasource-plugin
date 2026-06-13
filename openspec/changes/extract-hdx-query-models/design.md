## Context

The plugin currently imports five data shapes from `github.com/hydrolix/sqlds/v5 v5.0.1`:

- `sqlds.HDXQuery` — top-level type in the fork's main package (defined in `interpolator.go`), used by `pkg/api/routes.go:63` when handling the `/interpolate` resource.
- `sqlds.AdHocFilter` — top-level struct in the fork's main package (also `interpolator.go`), used by `pkg/api/routes.go:164` inside the interpolate-request JSON envelope. Pure JSON shape; macro-side helpers (`buildArrayCondition`, `buildFilterCondition`) consume it but live elsewhere and stay with the fork until C7 ports the macro.
- `models.PluginSettings`, `models.QuerySetting`, `models.NewPluginSettings` — from the fork's `models/settings.go`, used by `pkg/plugin/driver.go:69,234,252,259,265,279,282,309`, `pkg/plugin/driver_test.go:18,235,279,282,285,334`, and `pkg/plugin/dssuit_test.go:54`.

`PluginSettings` carries the JSON wire format Grafana sends in `DataSourceInstanceSettings.JSONData` (host, port, protocol, credentials type, query settings, etc.). `HdxQuery` carries the per-query JSON Grafana sends in `DataQuery.JSON`. `AdHocFilter` is the per-filter shape inside the interpolate request. None have runtime behaviour beyond unmarshalling and validation.

The fork's tip at the extension-points revision (`ef925e1`) no longer carries Hydrolix-specific code. The data shapes have to move somewhere; the plugin owns them.

## Goals / Non-Goals

**Goals:**
- Move `PluginSettings`, `QuerySetting`, `NewPluginSettings`, `HdxQuery`, and their validation helpers into `pkg/plugin/models/` with byte-for-byte JSON parity.
- Drop the plugin's import of `github.com/hydrolix/sqlds/v5/models` and the `sqlds.HDXQuery` reference in `pkg/api/routes.go`.
- Land independently of every other change in the sqlds-retirement sequence — no `go.mod` swap, no extension-point wiring.

**Non-Goals:**
- Renaming JSON tags. Wire format is frozen by deployed dashboards.
- Tightening validation rules. `IsValid` / `SetDefaults` move verbatim; tightening is a future change.
- Splitting the package further (e.g. `models/query/`, `models/settings/`). One flat package is enough at this size.
- Touching the fork. The fork keeps its `models` sub-package; the plugin just stops importing it.

## Decisions

### D1. One flat package, `pkg/plugin/models/`

Single package directory containing `query.go` (the `HdxQuery` shape + filter sub-shapes), `settings.go` (the `PluginSettings`, `QuerySetting`, constructors, validation), and their respective `_test.go` files.

**Why one package, not two.** The shapes are small (combined ~250 LOC of types + validation). Splitting them into `models/query/` and `models/settings/` adds two import paths, two package comments, and zero clarity. The fork itself groups them under one path (`hydrolix/sqlds/v5/models` for settings; main package for `HDXQuery`) only because of where `HDXQuery` happened to be defined. The plugin can pick the tidier layout.

**Why a sub-package of `pkg/plugin/` and not flat in `pkg/plugin/`.** Both `pkg/api/` and `pkg/plugin/` need these shapes. Flat placement would force `pkg/api/` → `pkg/plugin/` (HTTP layer importing the implementation layer — wrong direction). A `models/` sub-package keeps the dependency direction tidy: both depend on `models`, not on each other.

### D2. Rename `HDXQuery` → `HdxQuery` for cross-language parity

The plugin's TypeScript already uses `HdxQuery` (`src/types.ts`). The Go side adopts the same casing during the move.

**Why now.** This is the cheapest moment to make the rename — every import site is being updated anyway. A future rename would touch the same files twice.

**Why `HdxQuery` over the Go-idiomatic `HDXQuery`.** Cross-language search hits matter more day-to-day than capitalisation correctness. `grep -rn HdxQuery .` finding both TypeScript and Go references is concretely useful; `HDXQuery` matching only Go is mildly annoying. The Go style guide prefers initialism caps, but it's a guideline not a rule; consistency across the project boundary wins here.

**JSON wire format is unchanged.** Field tags (`json:"datasource"`, etc.) stay as the fork wrote them. Only the Go type identifier changes.

### D3. `models` package has no dependency on `sqlds`

The package imports only the Grafana plugin SDK (`backend`, `data/sqlutil`) and the Go standard library. It does not import `github.com/hydrolix/sqlds/v5` or its eventual replacement.

**Why no sqlds import.** The shapes are wire-format data. Wire-format types should depend only on the SDK that defines the wire (the Grafana plugin SDK). Pulling sqlds into the models package would couple the data layer to the query-execution library — backwards.

Practically: `HdxQuery` embeds `sqlutil.Query` from the SDK, not `sqlds.Query` from the fork. The two are the same type today (sqlds re-exports the SDK's `sqlutil.Query`), so this is a zero-behaviour rewire — just dropping a layer of indirection.

### D4. Validation logic moves verbatim

`PluginSettings.IsValid` and `PluginSettings.SetDefaults` move byte-for-byte from `hydrolix/sqlds@v5.0.1`'s `models/settings.go`. Error sentinels (`ErrorMessageInvalidHost`, etc.) move with them.

**Why verbatim, not a cleanup pass.** Validation is the only non-trivial logic in the package. Changing it during a code-move muddies the diff and risks behaviour drift that's hard to isolate during review. Cleanups (e.g. typed validation errors, structured violation reporting) are a future change.

### D5. Existing fork `models/settings_test.go` ports to the plugin

The fork's `models/settings_test.go` test surface (207 LOC at the fork's current pin) ports into `pkg/plugin/models/settings_test.go` with the same cases. Test names and assertions move verbatim modulo the package name.

**Why port rather than rewrite.** Same reason as D4: preserve the regression net during the move. A test-suite rewrite is a separate change.

## Risks / Trade-offs

- **[Subtle behaviour drift between fork's validation and the moved validation]** → Mitigation: ported tests are the same assertions. Any drift fails the test suite loudly. Confirmed by running both the fork's and the moved test suites against the moved code during the PR.
- **[`HdxQuery` rename touches more files than expected]** → Mitigation: `grep -rn 'HDXQuery'` before and after the change; expect zero matches in plugin code after. CI's `go vet ./...` and `golangci-lint run` catch any missed call sites.
- **[Future merge conflicts with the in-flight `retire-sqlds-fork` change]** → Mitigation: `extract-hdx-query-models` lands first (it's narrower); the larger change rebases on top. The retire-sqlds-fork design's D5 (which contemplated a similar `models/` move under a different name) is superseded by this change.

## Migration Plan

- **Forward**: single PR. Sequence inside the PR:
  1. Create `pkg/plugin/models/query.go`, `pkg/plugin/models/settings.go`, paired `_test.go` files. Content copied from the fork at the plugin's current pin (`v5.0.1`), with `HDXQuery` → `HdxQuery` rename applied.
  2. Update imports in `pkg/api/routes.go`, `pkg/plugin/driver.go`, `pkg/plugin/driver_test.go`, `pkg/plugin/dssuit_test.go`.
  3. Run `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`. All must pass.
  4. Playwright e2e via the `grafana-plugin-e2e` skill — should be a no-op since wire formats are unchanged, but run as a guard.
- **Rollback**: revert the PR. No data, dashboard, or downstream consumer is affected. The fork's `models` package still exists at the pinned version; the import paths just go back to where they were.
- **Sequencing**: independent. Can ship before `pin-sqlds-extension-revision` (the next change in the migration sequence) so that change's diff is smaller and easier to review.

## Open Questions

- Should `HdxQuery` carry a method receiver style helper (e.g. `WithSQL(rawSQL string) HdxQuery`) for the macro-rewrite flow, mirroring the fork's `(q *HDXQuery) WithSQL` in `interpolator.go`? Defer to the change that introduces the plugin-side interpolator (`plugin-hdx-interpolator`). That change has visibility into whether the helper is wanted; this change is just the data-shape move.
