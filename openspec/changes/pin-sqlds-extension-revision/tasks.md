# pin-sqlds-extension-revision — implementation tasks

## 1. `go.mod` pin via `replace` directive

- [ ] 1.1 Update `require` line in `go.mod` to reference the upstream module path: `github.com/grafana/sqlds/v5 v5.0.0-20260613103402-ef925e15e15e`.
- [ ] 1.2 Add `replace github.com/grafana/sqlds/v5 => github.com/hydrolix/sqlds/v5 v5.0.0-20260613103402-ef925e15e15e` directive with a comment explaining it is removed in C8.
- [ ] 1.3 Run `go mod tidy` to regenerate `go.sum`.

## 2. Bulk import-path swap

- [ ] 2.1 `find pkg -name '*.go' -exec sed -i '' 's|github.com/hydrolix/sqlds/v5|github.com/grafana/sqlds/v5|g' {} +` over `pkg/plugin/` and `pkg/api/`.
- [ ] 2.2 Confirm `grep -rn 'hydrolix/sqlds/v5' pkg/` returns zero matches.

## 3. Wrapper type `pkg/plugin/hdx_sqlds.go`

- [ ] 3.1 Define `HdxSqlDatasource` embedding `*sqlds.SQLDatasource`.
- [ ] 3.2 Define `NewHdxSqlDatasource(driver sqlds.Driver) *HdxSqlDatasource` that constructs via `sqlds.NewDatasource(driver)`, sets `EnableMultipleConnections = true`.
- [ ] 3.3 Override `NewDatasource(ctx, settings) (instancemgmt.Instance, error)` on `*HdxSqlDatasource` so the returned instance is the wrapper, not the embedded pointer.

## 4. Rewrite `pkg/plugin/datasource.go`

- [ ] 4.1 Replace fork-specific construction (`&sqlds.HydrolixDatasource{Connector: conn}`, `ds.RegisterRoutes(...)`) with `ds := NewHdxSqlDatasource(NewHydrolix())`.
- [ ] 4.2 Move HTTP route registration into a local `registerRoutes` helper that calls `api.Routes(ds.SQLDatasource)` and assigns `httpadapter.New(mux)` to `ds.CallResourceHandler`.
- [ ] 4.3 Return `ds.NewDatasource(ctx, settings)` (the overridden wrapper method).

## 5. Update `pkg/api/routes.go`

- [ ] 5.1 Import `github.com/grafana/grafana-plugin-sdk-go/data/sqlutil` and `github.com/hydrolix/plugin/pkg/plugin/models`.
- [ ] 5.2 Swap function signatures: `Routes` and `Interpolate` take `*sqlds.SQLDatasource`.
- [ ] 5.3 Rewrite `Interpolate` to call `ds.Interpolator.Interpolate(ctx, ds, *sqlutil.Query, json.RawMessage)`. Fall back to `sqlds.DefaultInterpolator{}` when `ds.Interpolator == nil`. Pass Hydrolix-specific fields via the marshalled `models.HdxQuery`.
- [ ] 5.4 Swap `QueryData.Filters` from `[]sqlds.AdHocFilter` to `[]models.AdHocFilter` (already plugin-local from C1).
- [ ] 5.5 Stub `MacroCTEs` to return an empty CTE list with a comment that C5 restores the full implementation.
- [ ] 5.6 Remove the now-unused `maps` and `slices` imports.

## 6. Update `pkg/plugin/driver.go`

- [ ] 6.1 Pin `Hydrolix.Settings` return value: `ForwardHeaders: false` regardless of `pluginSettings.CredentialsType`.

## 7. Update test files

- [ ] 7.1 `pkg/plugin/datasource_test.go`: change `case *sqlds.HydrolixDatasource:` to `case *plugin.HdxSqlDatasource:`. Remove the now-unused `sqlds` import.
- [ ] 7.2 `pkg/plugin/driver_test.go`'s `TestSettings_ForwardHeaders`: update the `forwardOAuth` case's `wantForward` from `true` to `false`. Add a comment explaining the new C2 invariant.

## 8. Quality gates

- [ ] 8.1 `go build ./...` — clean.
- [ ] 8.2 `go vet ./...` — clean.
- [ ] 8.3 `golangci-lint run ./pkg/plugin/... ./pkg/api/...` — no NEW issues introduced (pre-existing issues in `pkg/plugin/driver.go`, `pkg/plugin/driver_conv_test.go`, `pkg/api/routes.go` not in scope).
- [ ] 8.4 `go test -race ./...` — all packages green.
- [ ] 8.5 `npm run typecheck && npm run lint && npm test -- --ci` — green (no frontend impact).
- [ ] 8.6 Playwright e2e — deferred to the end of the C2-C7 coordinated set; in isolation the macroCTE handler is stubbed and the interpolator is no-op, so e2e would surface migration intermediates as test failures.

## 9. Commit

- [ ] 9.1 Single commit containing the `go.mod` pin, import-path swap, wrapper, datasource.go rewrite, routes.go update with stubbed MacroCTEs, driver.go ForwardHeaders pin, and test updates.
- [ ] 9.2 Commit message: `pkg/plugin/hdx_sqlds: pin grafana/sqlds@ef925e1 + add HdxSqlDatasource wrapper (C2)`. Mention macroCTE stub and interpolator passthrough as behaviour caveats restored in C5.
