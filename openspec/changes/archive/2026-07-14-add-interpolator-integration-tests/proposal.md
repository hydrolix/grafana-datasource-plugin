## Why

A coverage audit found the SQL interpolation pipeline is exercised only by isolated unit tests — each macro function is called directly, and the `/interpolate` HTTP route is tested with a stub interpolator. Nothing drives the real `HdxInterpolator.Interpolate` through the actual `Macros` registry on a raw SQL string, so regressions in macro dispatch (site matching, longest-name-first, escaped-macro handling), macro-to-CTE association, and metadata resolution would slip past the unit tests. This is a test-only change closing that seam.

## What Changes

- Add Go integration tests in `pkg/plugin` that construct a real `HdxInterpolator` over the real `Macros` registry with a stubbed metadata layer (pre-seeded caches), feed raw SQL through `interpolate()`, and assert the fully rewritten SQL.
- Add an HTTP-route integration test in `pkg/api` that wires the **real** `HdxInterpolator` behind the `/interpolate` handler (not the existing stub) and asserts the response for a macro-bearing query.
- Reuse existing scaffolding (`nopMetadataDS`, `preseededProvider`, the `routes_test.go` request helpers); no new test dependencies.
- Test-only: no production code changes, no wire-format or API changes. Non-breaking.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `hdx-interpolator`: adds a requirement that the end-to-end interpolation pipeline (real interpolator + real registry + stubbed metadata, and the `/interpolate` route over the real interpolator) is covered by integration tests. No behavioral requirement changes — this codifies verification of existing behavior at the integration seam.

## Impact

- Code: new `pkg/plugin/interpolator_integration_test.go`; a new route-integration test in `pkg/api` (`routes_test.go` or a sibling `_test.go`). No non-test files change.
- Coverage: catches dispatch/association/metadata-resolution regressions at the seam, complementing the per-macro unit tests.
- The Playwright e2e suite still tests macros in isolation (gap #13 and others); this change is Go-level integration, not e2e, and does not close those e2e gaps.
