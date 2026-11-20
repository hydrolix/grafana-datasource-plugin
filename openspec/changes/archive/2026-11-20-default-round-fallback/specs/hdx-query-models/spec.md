## MODIFIED Requirements

### Requirement: Datasource settings shape `PluginSettings`

The plugin SHALL define a `PluginSettings` type in `pkg/plugin/models` that carries the configuration JSON Grafana sends in `DataSourceInstanceSettings.JSONData`, plus the secret fields (`Password`, `Token`) pulled from `DecryptedSecureJSONData`. `DefaultRound` SHALL be read from `jsonData.defaultRound` only when it is a JSON string; any other type SHALL be ignored without an error.

#### Scenario: `PluginSettings` round-trips the expected JSON fields

- **GIVEN** Grafana datasource JSON with `host`, `port`, `protocol`, `username`, `credentialsType`, `secure`, `path`, `skipTlsVerify`, `dialTimeout`, `queryTimeout`, `defaultDatabase`, and `querySettings`
- **WHEN** the JSON is unmarshalled via `models.NewPluginSettings`
- **THEN** each named field SHALL be populated according to the JSON values
- **AND** `Password` and `Token` SHALL be populated from `DecryptedSecureJSONData["password"]` and `["token"]`

#### Scenario: `PluginSettings` validates mandatory fields

- **GIVEN** a `PluginSettings` value
- **WHEN** `IsValid` is invoked
- **THEN** the call SHALL return `ErrorMessageInvalidHost` if `Host` is empty
- **AND** SHALL return `ErrorMessageInvalidPort` if `Port` is zero
- **AND** SHALL return `ErrorMessageInvalidProtocol` if `Protocol` is not in `{"http", "native"}`
- **AND** SHALL return `ErrorMessageInvalidDialTimeout` if `DialTimeout` is not parseable as an integer
- **AND** SHALL return `ErrorMessageInvalidQueryTimeout` if `QueryTimeout` is not parseable as an integer

#### Scenario: `PluginSettings` applies defaults to omitted timeouts

- **GIVEN** a `PluginSettings` value with empty `DialTimeout` and `QueryTimeout`
- **WHEN** `SetDefaults` is invoked
- **THEN** `DialTimeout` SHALL be set to `"10"`
- **AND** `QueryTimeout` SHALL be set to `"60"`

#### Scenario: A string `defaultRound` is read

- **GIVEN** `jsonData.defaultRound` is the JSON string `"5m"`
- **WHEN** the data source settings are parsed
- **THEN** parsing SHALL succeed and `DefaultRound` SHALL be `"5m"`

#### Scenario: A non-string `defaultRound` is ignored

- **GIVEN** `jsonData.defaultRound` is the JSON number `60`
- **WHEN** the data source settings are parsed
- **THEN** parsing SHALL succeed and `DefaultRound` SHALL be empty
