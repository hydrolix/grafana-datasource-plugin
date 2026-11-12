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

`querySettingDefinitions` becomes a `Map<string, QuerySettingDefinition>`, where `QuerySettingDefinition` is the catalog element type inferred from `labels.ts`. Every `get` has type `QuerySettingDefinition | undefined`, so `tsc` fails on an unguarded dereference. This replaces an `any`.

A `Map` rather than a plain object: a stored name such as `constructor` or `toString` would otherwise resolve to an `Object.prototype` member and read as a known setting. The validation errors are held by row position for the same reason.

### D3. Edit and removal address a row by position

`onQuerySettingsChange` and `deleteQuerySetting` take the row's index in `jsonData.querySettings`, not its name. Names are not unique in a provisioned data source, and matching by name edited or removed every entry that shared one. Neither handler consults the catalog, so unknown entries need nothing more. The row's remove button gets the accessible name `Remove <setting>` (from `src/labels.ts`) in place of an empty `aria-label`, so screen readers and tests can tell the rows' buttons apart.

### D4. Tolerate malformed `querySettings`

A `querySettings` that is not an array is read as empty. An entry that is not an object (`null`, a string) gets no row. Both are left as they are in `jsonData`: the editor writes entries back by position and does not drop or rewrite what it cannot show. A non-array value is replaced by an array only when the user adds a setting.

**Alternative considered:** strip malformed entries on the first edit. Rejected for the same reason as dropping unknown entries in D1: the editor should not silently change what is stored.

An unknown row is also not marked `required`: its value is not validated, so the asterisk would promise a check that never runs.

## Risks / Trade-offs

- [The inferred catalog type may not expose `min` / `default` on every element, breaking existing reads] → `npm run typecheck`; fall back to an explicit interface in `types.ts` if inference is too narrow.
- [A stored entry with an empty or missing `setting` name renders a row with no label] → The row still renders and can be removed, which is the way out; `addQuerySetting` never writes one. The unit test for entries that are not objects proves the section renders.
- [Rows keyed by position remount when an earlier row is removed] → Acceptable: a row holds no local state, its value comes from `jsonData`. The duplicate-name unit tests prove edit and removal hit one row.
- [The warning is easy to miss in a description line] → Acceptable for a non-blocking state; the unit test and e2e assert the text is shown.
