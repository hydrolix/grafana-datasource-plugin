# Tasks

## 1. Adapt the interpolator to the func field

- [x] 1.1 Drop the `*sqlds.SQLDatasource` parameter from `HdxInterpolator.Interpolate`
- [x] 1.2 Add the compile-time `var _ sqlds.Interpolator = (&HdxInterpolator{}).Interpolate` assertion
- [x] 1.3 Install the method value `NewHdxInterpolator(...).Interpolate` on `ds.Interpolator` in `NewHdxSqlDatasource`

## 2. Adapt the HTTP handler

- [x] 2.1 Call `ds.Interpolator(ctx, *sqlutil.Query, json.RawMessage)` directly in `pkg/api/routes.go`
- [x] 2.2 Replace the `sqlds.DefaultInterpolator{}` nil-fallback with an explicit "interpolator not configured" error

## 3. Tests

- [x] 3.1 Update the conformance call in `pkg/plugin/interpolator_test.go`
- [x] 3.2 Update the `routes_test.go` stub to a method value; rename the nil-fallback test to assert the error path

## 4. Integration & verification

> **Superseded by `retire-hydrolix-sqlds-fork`.** The func-interpolator code
> (§1-§3) is committed (`a2d9d6f`) and compiles against upstream
> `grafana/sqlds@v5.2.0` unchanged — verified in that change's design D2/D3
> (`Interpolator` is the identical func-typed surface upstream). The original
> plan advanced the *fork* `replace` pin to an `interpolator-func-field`
> revision; the migration now resolves straight to upstream, so that step is
> dropped and the remaining verification runs as part of the swap change's
> quality gates. This change archives with the migration sequence
> (`sqlds-migration-plan` §11.2), after the swap lands.

- [ ] 4.1 ~~Advance the `go.mod` `replace` pin to a fork revision carrying `interpolator-func-field`~~ — **dropped**; resolution moves to upstream `v5.2.0` via `retire-hydrolix-sqlds-fork` (its tasks §1).
- [ ] 4.2 `go vet ./...` and `go test -race ./...` green in the dev container — performed by `retire-hydrolix-sqlds-fork` (its tasks §4.2-4.3) against `v5.2.0`.
- [ ] 4.3 `npm run build` clean dist + Playwright e2e exercising the `/interpolate` path — performed by `retire-hydrolix-sqlds-fork` (its tasks §4.4-4.5).
