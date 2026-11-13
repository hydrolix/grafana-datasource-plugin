## 1. Implementation

- [x] 1.1 Add the unknown-setting warning text to `components.querySettings` in `src/labels.ts`
- [x] 1.2 Type `querySettingDefinitions` in `ConfigEditor.tsx` as `Map<string, QuerySettingDefinition>` (catalog element type) and fix the reads `tsc` then flags
- [x] 1.3 Render an unknown setting's row with the warning as its description and a plain text input (no `min`, not `required`)
- [x] 1.4 Address rows by position in `onQuerySettingsChange`, `deleteQuerySetting` and `settingErrors`
- [x] 1.5 Read a non-array `querySettings` as empty and skip entries that are not objects; the first edit writes the list without them
- [x] 1.6 Move the remove button's accessible name to `src/labels.ts`
- [x] 1.7 Remove a row whose input loses focus while empty
- [x] 1.8 `src/datasource.ts`: the query settings merge skips entries that are not objects
- [x] 1.9 `pkg/plugin/driver.go`: `MutateQuery` leaves empty-valued settings out of the query context

- [x] 1.10 Show a boolean setting stored as `true`, `"true"` or `1` as "Yes"

## 2. Unit tests

- [x] 2.1 `ConfigEditor.test.tsx`: an unknown setting renders without throwing, shows its value and the warning, and no validation error
- [x] 2.2 `ConfigEditor.test.tsx`: known and unknown settings render together; the known row keeps its catalog description
- [x] 2.3 `ConfigEditor.test.tsx`: editing an unknown setting calls `onOptionsChange` with the new value
- [x] 2.4 `ConfigEditor.test.tsx`: removing an unknown setting leaves only the other entries
- [x] 2.5 `ConfigEditor.test.tsx`: a bad value of a known setting shows a validation error (positive control for 2.1)
- [x] 2.6 `ConfigEditor.test.tsx`: an unknown setting named `constructor` is not marked invalid
- [x] 2.7 `ConfigEditor.test.tsx`: a non-array `querySettings` and entries that are not objects render; an edit drops those entries
- [x] 2.8 `ConfigEditor.test.tsx`: with two entries of one name, edit and removal affect one row

- [x] 2.9 `ConfigEditor.test.tsx`: an empty known or unknown setting is removed on blur; one with a value is kept
- [x] 2.10 `datasource.test.ts`: a `null` data-source-level entry is skipped in the merge
- [x] 2.11 `driver_test.go`: `MutateQuery` leaves an empty-valued setting out of the context

- [x] 2.12 `ConfigEditor.test.tsx`: a boolean setting stored as `"1"`, `1`, `true` or `"true"` shows "Yes"; `"0"`, `0` or `false` shows "No"

## 3. E2E

- [x] 3.1 `tests/configEditor.spec.ts`: create a data source via `createDataSource` with an unknown `querySettings` entry, open it with `gotoDataSourceConfigPage`, and assert the row and warning are visible

## 4. Verification

- [x] 4.1 `npm run typecheck`, `npm run lint`, `npm run test:ci` pass (test count ≠ 0)
- [x] 4.4 `go vet ./...`, `golangci-lint run`, `go test -race ./...` pass
- [x] 4.2 `npm run build` produces a clean `dist/`
- [x] 4.3 Run the new e2e spec locally once against the dev stack (`e2e-dev`)
