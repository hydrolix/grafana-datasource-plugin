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

- [ ] 4.1 Advance the `go.mod` `replace` pin to a fork revision carrying `interpolator-func-field`
- [ ] 4.2 `go vet ./...` and `go test -race ./...` green in the dev container
- [ ] 4.3 `npm run build` clean dist + Playwright e2e exercising the `/interpolate` path
