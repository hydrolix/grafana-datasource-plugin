## Context

The config editor's Query Settings section renders one row per entry of `jsonData.querySettings`. Each row looks its definition up in `querySettingDefinitions`, a map built from the catalog in `src/labels.ts` (`components.querySettings.values`) and typed `{ [setting: string]: any }`. Three render-time reads dereference the lookup without a guard: the row's `description`, and `.type` / `.min` in `settingInput`. An entry whose name is not in the catalog makes the first of them throw, and the whole page fails to render. The validation pass (`settingErrors`) and `addQuerySetting` already use `?.`.

Unknown names reach `jsonData` through provisioning, the HTTP API, or a catalog entry removed in a later release. The query path forwards every entry to Hydrolix unchanged, so only the editor is affected.

## Goals / Non-Goals

**Goals:**
- The config page renders whatever `jsonData.querySettings` holds.
- An unknown entry is visible, marked as unknown, editable and removable.
- The compiler rejects a future unguarded lookup.

**Non-Goals:**
- Validating unknown values, or dropping unknown entries that hold a value from `jsonData`.
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

A `querySettings` that is not an array is read as empty. An entry that is not an object (`null`, a string) gets no row, and the first edit of any row (change, add, remove) writes the list without it. Opening the page changes nothing.

Such an entry cannot be shown as a row, since it has no name or value to show, and the backend settings parser rejects the whole data source for it. Leaving it in `jsonData` would keep the data source broken with nothing in the UI to remove. The frontend query settings merge skips these entries too, so a query fails with the backend's message and not a `TypeError` in the browser.

**Alternative considered:** leave malformed entries untouched, as D1 does for unknown names. Rejected: an unknown name works and is visible; a malformed entry does neither.

An unknown row is also not marked `required`: its value is not validated, so the asterisk would promise a check that never runs.

### D5. An empty value means "not set"

A text, number, duration or unknown row whose input loses focus while empty is removed from the list, the way the ad hoc lookback field clears itself on blur. On blur, not on change, so the field can be cleared and retyped.

Blur only covers rows someone has touched, so `MutateQuery`, the one place every query's merged settings enter the ClickHouse context, also leaves out a setting whose value is `""`. It is done there and not in the frontend merge because `MutateQueryData` merges the data source's stored settings in again on the backend. A query-level empty value still overrides the data-source-level one in the merge, so the result is that the setting is not sent at all.

**Alternative considered:** keep sending empty values and show "setting value is required". Rejected: nothing stops the save, and what Hydrolix does with `setting=` depends on the setting's type.

### D6. A boolean setting reads `true` and `1` as on

The editor stores a boolean setting as `"1"` / `"0"`, and its select compared the stored value to `"1"` only. Unquoted provisioning YAML leaves a JSON `true` or `1` instead; the backend settings parser turns those into `"true"` / `"1"` and sends them, so the query ran with the setting on while the editor showed "No". The select now reads `"1"`, `1`, `true` and `"true"` as on. The stored value is not rewritten until the user picks an option.

## Risks / Trade-offs

- [The inferred catalog type may not expose `min` / `default` on every element, breaking existing reads] → `npm run typecheck`; fall back to an explicit interface in `types.ts` if inference is too narrow.
- [A stored entry with an empty or missing `setting` name renders a row with no label] → The row still renders and can be removed, which is the way out; `addQuerySetting` never writes one. The unit test for entries that are not objects proves the section renders.
- [An unknown row cleared by mistake and blurred cannot be re-added from the picker, which lists catalog names only] → Removal happens on blur, not while typing; the entry can be restored through provisioning or the API, where it came from.
- [Rows keyed by position remount when an earlier row is removed] → Acceptable: a row holds no local state, its value comes from `jsonData`. The duplicate-name unit tests prove edit and removal hit one row.
- [The warning is easy to miss in a description line] → Acceptable for a non-blocking state; the unit test and e2e assert the text is shown.
