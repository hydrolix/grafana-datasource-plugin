## Context

The config editor's Query Settings section renders one row per entry of `jsonData.querySettings`. Each row looks its definition up in `querySettingDefinitions`, a map built from the catalog in `src/labels.ts` (`components.querySettings.values`) and typed `{ [setting: string]: any }`. Three render-time reads dereference the lookup without a guard: the row's `description`, and `.type` / `.min` in `settingInput`. An entry whose name is not in the catalog makes the first of them throw, and the whole page fails to render. The validation pass (`settingErrors`) and `addQuerySetting` already use `?.`.

Unknown names reach `jsonData` through provisioning, the HTTP API, or a catalog entry removed in a later release. The query path forwards every entry to Hydrolix unchanged, so only the editor is affected.

## Goals / Non-Goals

**Goals:**
- The config page renders whatever `jsonData.querySettings` holds.
- An unknown entry is visible, marked as unknown, editable and removable.
- The compiler rejects a future unguarded lookup.

**Non-Goals:**
- Validating unknown values, or dropping unknown entries from `jsonData`.
- Changing the per-query QuerySettings editor, which already tolerates unknown names.
- Backend validation of setting names.

## Decisions

### D1. Render an unknown entry as a plain text input with a warning description

The row keeps its name as the label and its stored value. The input is the same `Input` used for `text` / `number` settings, with no `min`. The `Field` description, which carries the catalog description for known settings, carries a warning instead: the plugin does not know this setting and sends it to Hydrolix as-is. The text lives in `src/labels.ts` beside the catalog.

The entry is not marked invalid and has no value validation: the plugin cannot know which values are valid, and the data source already sends it on every query.

**Alternative considered:** drop unknown entries from the editor (and from `jsonData` on the next save). Rejected: it silently changes what the data source sends, and a renamed setting would vanish without the user seeing it.

**Alternative considered:** show the entry as `invalid` with the warning as the error. Rejected: an error state reads as "fix this before saving", while the entry works; the user may have set it on purpose for a newer Hydrolix setting.

### D2. Type the catalog lookup so a missing entry is `undefined`

`querySettingDefinitions` becomes `Partial<Record<string, QuerySettingDefinition>>`, where `QuerySettingDefinition` is the catalog element type inferred from `labels.ts`. Every read then has type `QuerySettingDefinition | undefined`, so `tsc` fails on an unguarded dereference. This replaces an `any`.

### D3. Removal and edit reuse the existing handlers

`onQuerySettingsChange` and `deleteQuerySetting` match by name and never consult the catalog, so they already work for unknown entries. No change is needed beyond rendering the row. The row's remove button gets the accessible name `Remove <setting>` in place of an empty `aria-label`, so screen readers and tests can tell the rows' buttons apart.

## Risks / Trade-offs

- [The inferred catalog type may not expose `min` / `default` on every element, breaking existing reads] → `npm run typecheck`; fall back to an explicit interface in `types.ts` if inference is too narrow.
- [A stored entry with an empty `setting` name renders an empty row] → Unchanged from today (`addQuerySetting` never writes one); covered by the existing behaviour, not by this change.
- [The warning is easy to miss in a description line] → Acceptable for a non-blocking state; the unit test and e2e assert the text is shown.
