## MODIFIED Requirements

### Requirement: Datasource settings constructor `NewPluginSettings`

The plugin SHALL define `NewPluginSettings(ctx context.Context, src backend.DataSourceInstanceSettings) (PluginSettings, error)` that parses, defaults, and validates the settings JSON in one pass.

`NewPluginSettings` SHALL NOT panic for any `JSONData` that is valid JSON. Each string field (`host`, `protocol`, `credentialsType`, `path`, `username`, `defaultDatabase`, `dialTimeout`, `queryTimeout`, and each `querySettings` entry's `setting` and `value`) SHALL accept a JSON string as-is, a JSON number as its shortest plain decimal text (no exponent), and a JSON boolean as `"true"` or `"false"`. Any other JSON type in one of these fields SHALL return a `backend.DownstreamError` that wraps `ErrorMessageInvalidSettingType` and names the field, without its value.

A `querySettings` value that is not a JSON array SHALL return that error with the field `querySettings`. In `querySettings`, an element that is not a JSON object SHALL return that error with the field `querySettings[<index>]`; an element whose `setting` is missing or empty SHALL be skipped; an element with no `value` SHALL get the value `""`.

#### Scenario: `NewPluginSettings` returns validated settings on a valid input

- **GIVEN** a valid `DataSourceInstanceSettings` with `JSONData` containing the required fields
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the returned `PluginSettings` SHALL equal the parsed JSON modulo applied defaults
- **AND** the returned error SHALL be nil

#### Scenario: `NewPluginSettings` propagates a wrapped JSON-parse error

- **GIVEN** a `DataSourceInstanceSettings` with `JSONData == []byte("invalid")`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the returned error SHALL wrap `ErrorMessageInvalidJSON`

#### Scenario: Numeric timeouts and query setting values are read as text

- **GIVEN** `JSONData` with `dialTimeout: 10`, `queryTimeout: 120` and `querySettings: [{"setting": "hdx_query_max_execution_time", "value": 30}]` as JSON numbers
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the returned error SHALL be nil
- **AND** `DialTimeout` SHALL be `"10"`, `QueryTimeout` SHALL be `"120"`, and the query setting's `Value` SHALL be `"30"`

#### Scenario: Numbers and booleans in other string fields are read as text

- **GIVEN** `JSONData` with `username: 12345`, `defaultDatabase: 2024`, and a query setting `{"setting": "hdx_query_streaming_result", "value": true}`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** `UserName` SHALL be `"12345"`, `DefaultDatabase` SHALL be `"2024"`, and the query setting's `Value` SHALL be `"true"`

#### Scenario: Fractional numbers keep their decimal text

- **GIVEN** a query setting `{"setting": "x", "value": 0.5}`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the query setting's `Value` SHALL be `"0.5"`

#### Scenario: A fractional timeout fails the existing timeout validation

- **GIVEN** `JSONData` with `dialTimeout: 10.5`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the returned error SHALL wrap `ErrorMessageInvalidDialTimeout`

#### Scenario: An unsupported type returns an error naming the field

- **GIVEN** `JSONData` with `dialTimeout: {"seconds": 10}`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** it SHALL NOT panic
- **AND** the returned error SHALL wrap `ErrorMessageInvalidSettingType`
- **AND** `backend.IsDownstreamError` SHALL be true for it
- **AND** its message SHALL contain `dialTimeout`

#### Scenario: A query setting that is not an object returns an error naming its index

- **GIVEN** `JSONData` with `querySettings: ["hdx_query_max_rows"]`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** it SHALL NOT panic
- **AND** the returned error SHALL wrap `ErrorMessageInvalidSettingType`
- **AND** its message SHALL contain `querySettings[0]`

#### Scenario: A `querySettings` value that is not an array returns an error naming the field

- **GIVEN** `JSONData` with `querySettings: {"setting": "hdx_query_max_rows", "value": "1"}`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** it SHALL NOT panic
- **AND** the returned error SHALL wrap `ErrorMessageInvalidSettingType`
- **AND** its message SHALL contain `querySettings` and SHALL NOT contain the value

#### Scenario: Nameless entries are skipped and a missing value is empty

- **GIVEN** `querySettings: [{"value": "1"}, {"setting": ""}, {"setting": "hdx_query_max_rows"}]`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the returned error SHALL be nil
- **AND** `QuerySettings` SHALL equal `[{Setting: "hdx_query_max_rows", Value: ""}]`
