## Why

`NewPluginSettings` reads the data source's string fields with unchecked type assertions (HDX-12842). A JSON number in `dialTimeout`, `queryTimeout` or a query setting `value`, which is what an unquoted value in provisioning YAML, Pulumi or the HTTP API produces, panics the backend every time it builds the data source. The SDK recovers the panic, but that data source then fails every health check and query with a generic error, and the only clue is a panic in the Grafana server log. Programmatic provisioning (HDX-12629) makes this easy to hit.

## What Changes

- The backend accepts a JSON number or boolean wherever the data source configuration holds a string (`host`, `protocol`, `credentialsType`, `path`, `username`, `defaultDatabase`, `dialTimeout`, `queryTimeout`, and each query setting's `setting` and `value`) and uses its text form.
- A value of any other JSON type (object, array) returns a data source error naming the field instead of panicking.
- A `querySettings` entry that is not an object returns the same kind of error; an entry with no setting name is skipped, as the frontend already does; an entry with no `value` gets an empty value.
- Existing validation is unchanged: a timeout that is not a whole number still fails with the existing invalid-timeout error.
- Go unit coverage for every field and type; Playwright e2e coverage for a provisioned data source with unquoted numbers.

Not breaking: every configuration that works today parses to the same settings.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `hdx-query-models`: `NewPluginSettings` accepts numbers and booleans in string fields, and returns an error naming the field, never a panic, for any other type.

## Impact

- `pkg/plugin/models/settings.go` and `settings_test.go`.
- `tests/` e2e spec for health and query with numeric settings.
- No frontend change; the config editor already displays numeric values.
