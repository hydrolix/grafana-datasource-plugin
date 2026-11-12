## Why

The data source config page crashes with a `TypeError` when `jsonData.querySettings` holds a setting name that is not in the plugin's QuerySettings catalog (HDX-12858). The page cannot be opened, so the entry cannot be fixed or removed from the UI. A provisioning typo is enough to hit it, and so is any future release that renames or drops a catalog entry: every data source that saved the old name would stop opening. Queries are unaffected; the setting is still sent to Hydrolix as-is.

## What Changes

- The config editor renders a stored query setting whose name is not in the catalog instead of crashing.
- Such a setting shows as a plain text input with a warning that the plugin does not know it and sends it as-is.
- An unknown setting can be edited and removed like a known one.
- Known settings render and validate exactly as before.
- The section also renders when `jsonData.querySettings` is not a list or holds entries that are not objects.
- Editing or removing a row affects that row only, including when two stored entries share a name.
- Frontend unit and Playwright e2e coverage for the config editor with an unknown setting.

No backend change. Not breaking: stored `jsonData` is read and written unchanged.

## Capabilities

### New Capabilities

- `hdx-config-query-settings`: how the data source config editor lists, edits, validates and removes the data-source-level QuerySettings stored in `jsonData.querySettings`, including entries the catalog does not define.

### Modified Capabilities

None.

## Impact

- `src/components/ConfigEditor.tsx`: the Query Settings section and its catalog lookup.
- `src/labels.ts`: the warning text for an unknown setting.
- `src/components/ConfigEditor.test.tsx`: new cases.
- `tests/configEditor.spec.ts`: new e2e case.
- The per-query QuerySettings editor in the query editor already tolerates unknown names and is not changed.
