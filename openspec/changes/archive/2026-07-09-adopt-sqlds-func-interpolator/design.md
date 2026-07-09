# Design

## Decisions

### D1: Install a method value, not a struct that implements an interface

`HdxInterpolator` keeps its fields (`md`, `macros`) and its package-private
dispatch, but its `Interpolate` method now matches the `sqlds.Interpolator`
func type. `NewHdxSqlDatasource` assigns the method value
`NewHdxInterpolator(...).Interpolate` to `ds.Interpolator`.

Rationale: the extension is a func field now. A method value carries the
`md`/`macros` the interpolator closes over with zero extra surface — no
interface, no exported wrapper. A compile-time
`var _ sqlds.Interpolator = (&HdxInterpolator{}).Interpolate` assertion locks
the signature so a future fork change breaks at build here, not at the wiring
site.

### D2: The `/interpolate` handler returns an error on a nil field

`sqlds.DefaultInterpolator` no longer exists, so the handler's previous
nil-fallback is gone. `NewHdxSqlDatasource` always installs the Hydrolix
interpolator, so in production `ds.Interpolator` is never nil; a nil field
means the datasource was not constructed through that path. The handler detects
nil and returns "interpolator not configured".

Rationale: honest failure over silent degradation. Alternative considered:
reach for an sqlds-provided default — rejected, the default is now unexported
*and* would skip every Hydrolix macro silently, producing wrong SQL rather
than an error. The handler's existing `recover()` remains a backstop.

### D3: `rawJSON` round-trip is unchanged

The handler already marshals `models.HdxQuery` (carrying `Filters`, `Round`,
`TimeRange`, `Interval`, `Headers`) into `rawJSON`, and the func signature
still takes `rawJSON`. So nothing changes in how Hydrolix-specific fields
travel to the interpolator.

## Risks

- [The `go.mod` fork pin must advance in lockstep, or the plugin won't build]
  → Mitigation: this change is gated on the fork landing
  `interpolator-func-field`; the compile-time assertion catches signature drift
  at build time.
- [A full build needs the dev container — host hits the go 1.25.9 toolchain
  floor in the fork's deps] → Mitigation: documented; the sqlds package plus
  the edited plugin files were confirmed to compile in a combined worktree,
  blocked only by that unrelated toolchain floor. Verification completes in the
  container (`grafana-plugin-build`).
