## Why

Grafana's catalog security review found that PR #152 closed only one of the ad-hoc filter injection inputs (the filter *value*, now escaped). The `$__adHocFilter` macro still copies the filter **operator** and the **map key subscript** into SQL verbatim, so a Viewer submitting raw filter JSON via `/api/ds/query` can rewrite the `WHERE` clause (e.g. operator `= 'x' OR 1=1 -- `). Because this ships as a signed catalog plugin to many Grafana Cloud and on-prem deployments, the injection class must be fully closed before publish.

## What Changes

- Allowlist the operators accepted by the scalar/map path in `buildFilterCondition` (`pkg/plugin/macros_adhoc.go`); reject anything outside the allowlist with a typed error, matching the array path's existing behavior.
- Validate and quote the map key subscript in `AdHocFilterMacro`: the base column stays schema-validated, and the `['...']` subscript is re-emitted from trusted pieces (backtick-quoted column + escaped subscript literal) instead of the raw `filter.Key`.
- Add regression tests covering the operator position, the map-key/subscript position, and value quotes/backslashes/SQL metacharacters (Go unit tests, run with `-race`).
- No frontend, wire-format, or public-API changes. Honest dashboards behave identically; only malformed operators/keys are now rejected. Non-breaking for Grafana 10.x dashboards.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `hdx-adhoc-filter-macro-secure`: adds requirements that the scalar/map operator set is allowlisted and that the map key/subscript is validated and quoted before reaching SQL.

## Impact

- Code: `pkg/plugin/macros_adhoc.go` (`buildFilterCondition`, `AdHocFilterMacro`, `mapTypeFilterKey`), new tests in `pkg/plugin/macros_adhoc_test.go`.
- Behavior: filters with unknown operators or malformed map keys now produce an error / are dropped rather than being interpolated.
- Companion change `secure-metadata-identifiers` closes the remaining metadata-query vector of the same review; the two together fully close the injection class.
- Related but out of scope: the raw `params[0]` explicit-arg identifier check is owned by `secure-metadata-identifiers`.
