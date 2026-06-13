# plugin-hdx-interpolator — implementation tasks

## 1. CTE / AST visitors sub-package

- [ ] 1.1 Create `pkg/plugin/cte/` sub-package.
- [ ] 1.2 Add `pkg/plugin/cte/cte.go` with `MacroId`, `CTE`, `GetMacroCTEs`, `MacroPositions`, and the AST visitors (`macroVisitor`, `tableVisitor`, `queryVisitor`). Port from the fork's `interpolator.go:240-265` modulo type-name capitalisation.
- [ ] 1.3 Add `pkg/plugin/cte/cte_test.go` with happy-path + edge cases: simple macro, bare table, multiple macros per SELECT, `MacroPositions` over valid + invalid SQL.

## 2. Macros registry and `Stub`

- [ ] 2.1 Add `pkg/plugin/macros_registry.go` with `MacroFunc` type, `Macros map[string]MacroFunc`, and the `Stub` no-op.
- [ ] 2.2 Register `Macros["conditionalAll"] = Stub` in an `init()` block (parity with the fork).

## 3. `HdxInterpolator`

- [ ] 3.1 Add `pkg/plugin/interpolator.go`. Define `HdxInterpolator` with `md *MetadataProvider`, `macros map[string]MacroFunc`.
- [ ] 3.2 Implement `Interpolate(ctx, *sqlds.SQLDatasource, *sqlutil.Query, json.RawMessage) (string, error)` — the sqlds-extension interface. Unmarshal rawJSON into `models.HdxQuery`, overlay runtime fields, dispatch to `interpolate`.
- [ ] 3.3 Implement `interpolate(ctx, *models.HdxQuery) (string, error)` — the dispatch routine. Port from the fork's `Interpolator.Interpolate`. Round time range when `Round` is set; sort macro keys by length descending; collect macro matches via `getMacroMatches`; apply replacements in reverse byte order.
- [ ] 3.4 Add helpers: `getMacroMatches`, `parseArgs`, `roundTimeRange`. Port verbatim from the fork.
- [ ] 3.5 Define `ErrParseMacroArgs` for unbalanced-paren cases.

## 4. `MetadataProvider` stub

- [ ] 4.1 Add `pkg/plugin/metadata.go` with a placeholder `MetadataProvider` struct, `NewMetadataProvider`, `ErrMetadataProviderUnavailable`, and the stubbed `getPK` (returns the typed error).
- [ ] 4.2 Document that C7 replaces this file with the real TTL-cached implementation.
- [ ] 4.3 Add `//nolint:unused` on `getPK` — referenced by C6's macros that ship into the same package; the lint suppression is intentional for the C5 standalone build.

## 5. `models.HdxQuery.WithSQL`

- [ ] 5.1 Add `WithSQL(rawSQL string) *HdxQuery` method to `pkg/plugin/models/query.go`. The interpolator's macro-expansion loop calls it to feed each successive macro the partially-rewritten SQL.

## 6. Wire `ds.Interpolator` in the wrapper

- [ ] 6.1 Update `pkg/plugin/hdx_sqlds.go` so `NewHdxSqlDatasource` constructs `ds.Interpolator = NewHdxInterpolator(NewMetadataProvider(), Macros)`.

## 7. Restore `MacroCTEs` HTTP handler

- [ ] 7.1 Update `pkg/api/routes.go`'s `MacroCTEs` to use `cte.GetMacroCTEs` and `cte.CTE`.
- [ ] 7.2 Restore the `maps` / `slices` imports.

## 8. Tests

- [ ] 8.1 Add `pkg/plugin/interpolator_test.go` covering: `parseArgs` (no bracket, empty, single, multiple, trim, nested, unbalanced), `roundTimeRange` (invalid, sub-second, 1m), no-macros pass-through, unknown-macro-in-place, registered-macro dispatch, escaped-macro, `Stub` (conditionalAll), Round-before-macros, longer-keys-first, interface conformance, `ErrParseMacroArgs`.
- [ ] 8.2 `cte_test.go` from task 1.3 covers `cte.*`.

## 9. Quality gates

- [ ] 9.1 `go build ./...` — clean.
- [ ] 9.2 `go vet ./...` — clean.
- [ ] 9.3 `golangci-lint run` — no new issues vs C2.
- [ ] 9.4 `go test -race ./...` — green.
- [ ] 9.5 `npm run typecheck && npm test -- --ci` — green.
- [ ] 9.6 Playwright e2e — `/macroCTE` is now functional again; macroFunctions specs still fail until C6 + C7 land. Full e2e verification deferred to end of coordinated set.

## 10. Commit

- [ ] 10.1 Single commit with all of the above + this change's `specs/` and `tasks.md`.
- [ ] 10.2 Commit message: `pkg/plugin: port AST interpolator + CTE extraction (C5)`. Note: `Macros` registry is empty except for `conditionalAll`; C6 + C7 populate the rest.
