## Why

When a data source's `jsonData` has no `exposeErrors` block, which is the usual case for provisioned, API-created and pre-0.10.0 data sources, error exposure runs with its defaults (on), while the config page shows the switch off (HDX-12841). An admin has no reason to expect it to be running. On every query error it then writes a `var-hdx_query_errors=[…]` parameter into the URL, even on dashboards that have no such variable, so users see an unexplained parameter that carries error text along when the link is shared. A partial `exposeErrors` block (any of the four keys missing) silently records nothing, which disagrees with both.

## What Changes

- One effective error-exposure configuration: each key missing from `jsonData.exposeErrors` takes its default (`enabled: true`, `variableName: "hdx_query_errors"`, `maxCount: 5`, `ttl: 300`). The runtime and the config editor both use it.
- The config editor shows the effective state: the switch is on and the fields show the defaults when `exposeErrors` is missing or partial; editing writes the complete block.
- Error exposure writes to the URL only when the current dashboard defines the configured variable. Otherwise it does nothing.
- Unit coverage for the shared resolution, the editor, the data source wiring and the exposer; Playwright e2e coverage for the editor with an API-created data source.

Behaviour change: a partial block, or one missing `enabled`, now uses the defaults for the missing keys, so a block such as `{variableName: "x"}` turns error exposure on where it was silently off. A data source without the block keeps today's runtime behaviour, except that no URL parameter is written when the dashboard lacks the variable. Not breaking for Grafana 11.x dashboards.

## Capabilities

### New Capabilities

- `hdx-error-exposure`: how the data source resolves its error-exposure configuration, how the config editor displays and edits it, and when a query error is written to the dashboard variable.

### Modified Capabilities

None.

## Impact

- `src/defaultConfigs.ts`: the shared resolution.
- `src/datasource.ts`: the exposer's configuration.
- `src/components/ConfigEditor.tsx`: the Error Exposure section.
- `src/errors/errorExposer.ts`: skip when the variable is absent.
- Unit tests in `src/`, a new `errorExposer.test.ts`, and an e2e in `tests/configEditor.spec.ts`.
- No backend change.
