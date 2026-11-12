## Context

Until v0.6.0 the frontend chose the effective round (`getFirstValidRound([query.round, jsonData.defaultRound])`) in `datasource.ts` and in the query editor's preview. Commit `dca0c52` (v0.7.0) moved interpolation to the backend and removed both, without a backend equivalent. Today `HdxInterpolator.interpolate` rounds only when `query.Round` is non-empty and not `"0"`, and `PluginSettings` has no `DefaultRound`.

Every query path reaches `HdxInterpolator.interpolate`: sqlds queries (panels, alerting, annotations) through `Interpolate`, the `/interpolate` preview resource through the same method value, and `/validate` through the `QueryValidator` that holds the same interpolator. One interpolator is built per data source instance in `NewHdxSqlDatasource`, which has the parsed settings.

## Goals / Non-Goals

**Goals:**
- Effective round = the query's round if set; otherwise the data source default; `"0"` on the query disables rounding.
- One place applies it, so every path agrees.

**Non-Goals:**
- Sub-second rounding or new round formats.
- Frontend changes; the preview already goes through `/interpolate`.
- Validating `defaultRound` on the backend; the config editor validates it, and an unparseable value is already a logged no-op in `roundTimeRange`.

## Decisions

### D1. Apply the fallback in `HdxInterpolator.interpolate`

The interpolator computes the effective round from `query.Round` and its own `defaultRound`, then rounds as today. The query is not mutated beyond its time range, so `Round` keeps what the caller sent.

**Alternative considered:** fill `round` into the query JSON in `MutateQuery`. Rejected: `/interpolate` and `/validate` do not pass through `MutateQuery`, so the preview and validation would disagree with execution.

### D2. The interpolator gets the default as an unexported field set at construction

`NewHdxSqlDatasource` sets `interp.defaultRound` from the parsed settings. `NewHdxInterpolator`'s signature and the `metadataDS` interface stay as they are, so the 17 existing constructor calls and the test fakes are untouched; in-package tests set the field directly.

**Alternative considered:** add `DefaultRound()` to `metadataDS` next to `DefaultDatabase()`. Rejected: the metadata provider has no use for it, and every fake would need a new method.

### D3. Parse `defaultRound` with a checked assertion

`NewPluginSettings` stores `defaultRound` only when it is a JSON string; any other type is ignored. An unchecked assertion would panic like HDX-12842. A number has no valid meaning here anyway: the round format needs a unit (`30s`, `5m`), and `"0"` is the only bare number the editor accepts.

## Risks / Trade-offs

- [Data sources that configured a default round years ago start rounding, shifting their time windows] → This is the documented behaviour and what v0.6.0 did; called out in the PR. Covered by the e2e.
- [Metadata queries without their own round are now rounded too] → They either carry their own round (`5m` for ad-hoc preloads) or use `ZERO_TIME_RANGE`, which rounds to itself; unit test covers the zero range.
- [The field is set only on the production path] → `NewHdxSqlDatasource` test asserts the wiring.
