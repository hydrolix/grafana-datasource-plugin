## Why

The `interpolator-func-field` change in the `hydrolix/sqlds` fork reshapes the
pluggable interpolator extension from an `Interpolator` *interface*
(`Interpolate(ctx, *SQLDatasource, *sqlutil.Query, json.RawMessage)`) into a
`func` field on `SQLDatasource`
(`func(ctx, *sqlutil.Query, json.RawMessage) (string, error)`), and removes the
exported `DefaultInterpolator`.

The plugin consumes this extension in two places — `HdxInterpolator`
(`hdx-interpolator`) and the `/interpolate` HTTP handler
(`hdx-sqlds-wrapper`) — both written against the old interface. Neither
compiles against the new fork surface.

This change adapts the plugin to the func-field shape: drop the redundant
`*SQLDatasource` parameter, install the interpolator as a method value, and
replace the `DefaultInterpolator` nil-fallback in the HTTP handler with an
explicit error — the Hydrolix interpolator is always wired by
`NewHdxSqlDatasource`, so a nil field now signals a construction bug, not a
degraded-but-valid path.

## What Changes

- `pkg/plugin/interpolator.go`: `HdxInterpolator.Interpolate` drops the
  `*sqlds.SQLDatasource` parameter; add a compile-time
  `var _ sqlds.Interpolator = (&HdxInterpolator{}).Interpolate` assertion.
- `pkg/plugin/hdx_sqlds.go`: install the interpolator as a method value
  (`NewHdxInterpolator(...).Interpolate`) on `ds.Interpolator`.
- `pkg/api/routes.go`: call `ds.Interpolator(ctx, *sqlutil.Query, json.RawMessage)`
  directly; a nil field returns an "interpolator not configured" error instead
  of `sqlds.DefaultInterpolator{}`.
- `pkg/plugin/interpolator_test.go`, `pkg/api/routes_test.go`: updated to the
  func signature; the stub becomes a method value; the nil-fallback test
  becomes a nil-error test.
- `go.mod`: ~~the fork `replace` pin must advance to a revision carrying
  `interpolator-func-field`~~ — **superseded**. The migration resolves straight
  to upstream `grafana/sqlds@v5.2.0` (whose `Interpolator` surface is identical
  to this func-typed shape) via `retire-hydrolix-sqlds-fork`; no intermediate
  fork-pin advance is taken.
- **BREAKING** at the fork-API boundary only; no Grafana-facing behavior change.

## Capabilities

- `hdx-interpolator`
- `hdx-sqlds-wrapper`
