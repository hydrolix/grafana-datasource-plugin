## 1. Implementation

- [x] 1.1 Add the plain sentinel `ErrorMessageInvalidSettingType` and `parseString(field string, in any) (string, error)` beside `parseBool` / `parseUint` in `pkg/plugin/models/settings.go`
- [x] 1.2 Read `host`, `protocol`, `credentialsType`, `path`, `username`, `defaultDatabase`, `dialTimeout` and `queryTimeout` through `parseString`
- [x] 1.3 Rewrite the `querySettings` loop as a checked assertion to `[]any`: error on a non-array value and on a non-object element, skip a nameless one, `""` for a missing value, `parseString` for `setting` and `value`

## 2. Go unit tests

- [x] 2.1 `settings_test.go`: table cases for numbers and booleans in every string field, including fractional and large numbers
- [x] 2.2 `settings_test.go`: a fractional timeout fails with the existing invalid-timeout error
- [x] 2.3 `settings_test.go`: object / array values return `ErrorMessageInvalidSettingType` (via `errors.Is`), are downstream errors, and name the field without its value
- [x] 2.4 `settings_test.go`: `querySettings` non-array value, non-object element, nameless entries and missing value
- [x] 2.5 Fuzz-style guard: `NewPluginSettings` does not panic for each field set to every JSON type

## 3. E2E

- [x] 3.1 New spec: create a data source through the API with unquoted numeric `dialTimeout`, `queryTimeout` and a numeric query setting `value`; assert the health check succeeds and a `SELECT 1` query returns data
- [x] 3.2 Same spec: a data source with an object `dialTimeout` reports a health-check error that names `dialTimeout`

## 4. Verification

- [x] 4.1 `go vet ./...`, `golangci-lint run`, `go test -race ./...` pass (inside the dev container, per `build-plugin`)
- [x] 4.2 Backend binary rebuilt in the dev container; run the new e2e spec locally once (`e2e-dev`)
