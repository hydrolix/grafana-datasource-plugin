## 1. Implementation

- [x] 1.1 Add the unknown-setting warning text to `components.querySettings` in `src/labels.ts`
- [x] 1.2 Type `querySettingDefinitions` in `ConfigEditor.tsx` as `Partial<Record<string, QuerySettingDefinition>>` (catalog element type) and fix the reads `tsc` then flags
- [x] 1.3 Render an unknown setting's row with the warning as its description and a plain text input (no `min`)

## 2. Unit tests

- [x] 2.1 `ConfigEditor.test.tsx`: an unknown setting renders without throwing, shows its value and the warning, and no validation error
- [x] 2.2 `ConfigEditor.test.tsx`: known and unknown settings render together; the known row keeps its catalog description
- [x] 2.3 `ConfigEditor.test.tsx`: editing an unknown setting calls `onOptionsChange` with the new value
- [x] 2.4 `ConfigEditor.test.tsx`: removing an unknown setting leaves only the other entries

## 3. E2E

- [x] 3.1 `tests/configEditor.spec.ts`: create a data source via `createDataSource` with an unknown `querySettings` entry, open it with `gotoDataSourceConfigPage`, and assert the row and warning are visible

## 4. Verification

- [x] 4.1 `npm run typecheck`, `npm run lint`, `npm run test:ci` pass (test count ≠ 0)
- [x] 4.2 `npm run build` produces a clean `dist/`
- [x] 4.3 Run the new e2e spec locally once against the dev stack (`e2e-dev`)
