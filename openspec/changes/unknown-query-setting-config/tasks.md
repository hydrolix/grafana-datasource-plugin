## 1. Implementation

- [x] 1.1 Add the unknown-setting warning text to `components.querySettings` in `src/labels.ts`
- [x] 1.2 Type `querySettingDefinitions` in `ConfigEditor.tsx` as `Map<string, QuerySettingDefinition>` (catalog element type) and fix the reads `tsc` then flags
- [x] 1.3 Render an unknown setting's row with the warning as its description and a plain text input (no `min`, not `required`)
- [x] 1.4 Address rows by position in `onQuerySettingsChange`, `deleteQuerySetting` and `settingErrors`
- [x] 1.5 Read a non-array `querySettings` as empty and skip entries that are not objects, leaving them in `jsonData`
- [x] 1.6 Move the remove button's accessible name to `src/labels.ts`

## 2. Unit tests

- [x] 2.1 `ConfigEditor.test.tsx`: an unknown setting renders without throwing, shows its value and the warning, and no validation error
- [x] 2.2 `ConfigEditor.test.tsx`: known and unknown settings render together; the known row keeps its catalog description
- [x] 2.3 `ConfigEditor.test.tsx`: editing an unknown setting calls `onOptionsChange` with the new value
- [x] 2.4 `ConfigEditor.test.tsx`: removing an unknown setting leaves only the other entries
- [x] 2.5 `ConfigEditor.test.tsx`: a bad value of a known setting shows a validation error (positive control for 2.1)
- [x] 2.6 `ConfigEditor.test.tsx`: an unknown setting named `constructor` is not marked invalid
- [x] 2.7 `ConfigEditor.test.tsx`: a non-array `querySettings` and entries that are not objects render; an edit keeps those entries
- [x] 2.8 `ConfigEditor.test.tsx`: with two entries of one name, edit and removal affect one row

## 3. E2E

- [x] 3.1 `tests/configEditor.spec.ts`: create a data source via `createDataSource` with an unknown `querySettings` entry, open it with `gotoDataSourceConfigPage`, and assert the row and warning are visible

## 4. Verification

- [x] 4.1 `npm run typecheck`, `npm run lint`, `npm run test:ci` pass (test count ≠ 0)
- [x] 4.2 `npm run build` produces a clean `dist/`
- [x] 4.3 Run the new e2e spec locally once against the dev stack (`e2e-dev`)
