## Context

`NewPluginSettings` unmarshals `JSONData` into `map[string]interface{}` and reads each field with a type assertion. `port`, `secure` and `skipTlsVerify` go through tolerant helpers (`parseUint`, `parseBool`) that accept strings, numbers and booleans and return a `backend.DownstreamError` otherwise. Every other field uses an unchecked `.(string)`: `host`, `protocol`, `credentialsType`, `path`, `username`, `defaultDatabase`, `dialTimeout`, `queryTimeout`, and inside `querySettings` both `setting` and `value`. The `querySettings` loop also asserts each element to `map[string]interface{}`.

`encoding/json` decodes every JSON number to `float64`, so `dialTimeout: 10` in provisioning YAML arrives as `float64(10)` and the assertion panics. `NewPluginSettings` runs on every instance build (`Connect`, health check, `MutateQueryData`, the sqlds wrapper), so the data source never works. The SDK's `handlePanic` keeps the plugin process alive.

## Goals / Non-Goals

**Goals:**
- No input to `NewPluginSettings` panics.
- Numbers and booleans in string fields work, matching how `port` / `secure` already behave.
- Unsupported types fail with an error that names the field.

**Non-Goals:**
- Changing validation rules (`IsValid`), defaults, or the frontend.
- Checking that query setting names are known (HDX-12858 covers the editor side).

## Decisions

### D1. One `parseString(field, value)` helper for every string field

It returns a string unchanged, a number in plain decimal form without exponent or trailing zeros (`10` → `"10"`, `0.5` → `"0.5"`, `1e21` → `"1000000000000000000000"`), and a boolean as `"true"` / `"false"`. Anything else returns `backend.DownstreamError` wrapping a new plain sentinel, `ErrorMessageInvalidSettingType`, with the field name in the message. It sits beside `parseBool` / `parseUint` and follows their shape.

The timeouts need no special case: a whole number becomes `"10"` and passes `IsValid`; `10.5` becomes `"10.5"` and fails `IsValid` with the existing `ErrorMessageInvalidDialTimeout` / `ErrorMessageInvalidQueryTimeout`.

Booleans become `"true"` / `"false"`, the same text the frontend produces when it interpolates a datasource-level setting (`${s.value}`), so the backend and frontend send the same value. The config editor's own `"1"` / `"0"` storage is unaffected because those are already strings.

**Alternative considered:** accept only strings and whole numbers, and reject booleans and fractions. Rejected: `hdx_query_streaming_result: true` is a natural thing to provision, and Hydrolix, not the plugin, is the right judge of a setting's value.

### D2. The sentinel is a plain `errors.New`; the source is attached at the return site

`errors.Is(err, ErrorMessageInvalidSettingType)` must identify this error and nothing else. A sentinel built with `backend.DownstreamError(...)` would match every downstream error, because the SDK's `ErrorWithSource.Is` compares only the source. The existing sentinels in `settings.go` follow the same pattern.

### D3. `querySettings`: error on a non-array and on a non-object entry, skip a nameless entry, empty a missing value

- A `querySettings` value that is not an array returns the invalid-type error with field `querySettings`. It was ignored before, which dropped the operator's settings without a word; Grafana's plugin review asked for a validation error on a wrongly typed value.
- A non-object element returns the invalid-type error with field `querySettings[i]`; it cannot be a setting.
- An element whose `setting` is missing or empty is skipped. The frontend already drops it (`filter((s) => s.setting)`), so both sides send the same set.
- A missing `value` becomes `""`, which is what the config editor stores for an empty field today.

Iteration uses a type switch on `[]any` instead of `reflect`, which also removes the unchecked element assertion.

## Risks / Trade-offs

- [A number's text form differs from what the user wrote (`1.50` → `"1.5"`, a decimal with more than ~17 significant digits rounds)] → `strconv.FormatFloat(v, 'f', -1, 64)` prints the shortest form that reads back as the same `float64`, which is all `encoding/json` preserved anyway; covered by table tests.
- [An error message reveals a field name only, never its value] → deliberate: values can be sensitive (`username`); covered by a test asserting the message names the field.
- [A setting that used to panic now silently works with an unexpected text form] → the behaviour is documented in the spec scenarios; the e2e asserts the health check and a query succeed with unquoted numbers.
