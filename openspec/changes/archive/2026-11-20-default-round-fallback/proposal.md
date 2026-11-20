## Why

The data source's **Default round** setting (`jsonData.defaultRound`) has had no effect since v0.7.0 (HDX-12840, duplicate HDX-12067). Moving macro expansion to the backend removed the frontend fallback, and nothing replaced it: the backend rounds only by the query's own round and never reads the setting. A query without its own round is not rounded although the setting is saved, shown in the config editor and documented. Dashboards that rely on it get inconsistent time windows and a lower query-cache hit rate.

## What Changes

- The backend reads `defaultRound` from the data source settings.
- The interpolator falls back to the data source default round for a query that has no round of its own.
- The fallback covers panel queries, alerting and other backend-only callers, the query editor's interpolated-SQL preview, and query validation.
- A non-string `defaultRound` is ignored instead of breaking the data source.
- A `defaultRound` that cannot round anything is ignored, with one warning per data source instance.
- The README's supported round units drop `ms`.
- Go unit coverage for the precedence rules and the settings parsing; Playwright e2e coverage for a query without its own round.

Behaviour change, matching the documentation and v0.6.0: data sources with a default round configured start rounding queries that have no round of their own.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `hdx-interpolator`: the `Round` requirement gains the data source default as the fallback when the query has no round. Its existing scenario's example time is corrected: `1m` rounds 12:00:30 up, not down.
- `hdx-query-models`: the `PluginSettings` requirement gains `DefaultRound`, read only from a JSON string.

## Impact

- `pkg/plugin/models/settings.go`: `PluginSettings.DefaultRound`.
- `pkg/plugin/interpolator.go`, `pkg/plugin/hdx_sqlds.go`: the fallback and its wiring.
- `README.md`: the supported round units.
- Go unit tests and a new e2e spec in `tests/`.
- No frontend change.
