# hdx-query-models Specification

## Purpose
TBD - created by archiving change extract-hdx-query-models. Update Purpose after archive.
## Requirements
### Requirement: Query shape `HdxQuery`

The plugin SHALL define a `HdxQuery` type in `pkg/plugin/models` that captures every per-query field the plugin reads from `backend.DataQuery.JSON` or sets at request time.

#### Scenario: `HdxQuery` carries the per-query JSON fields

- **GIVEN** a `backend.DataQuery` with `JSON` containing `rawSql`, `format`, `round`, `querySettings`, `filters`, and `meta.timezone`
- **WHEN** the JSON is unmarshalled into a `models.HdxQuery`
- **THEN** every field SHALL be populated; numeric, string, slice, and struct fields SHALL match the input verbatim modulo JSON-decoding conventions
- **AND** the non-JSON fields (`TimeRange`, `Interval`, `Headers`) SHALL be settable by the caller after unmarshal without affecting subsequent re-marshalling

#### Scenario: `HdxQuery` re-marshals to byte-identical JSON for round-trip fields

- **GIVEN** an `HdxQuery` populated from a JSON literal
- **WHEN** the value is re-marshalled via `json.Marshal`
- **THEN** the output SHALL contain every JSON-tagged field with its original value
- **AND** the non-JSON-tagged fields (`TimeRange`, `Interval`, `Headers`) SHALL NOT appear in the output

### Requirement: Ad-hoc filter shape `AdHocFilter`

The plugin SHALL define an `AdHocFilter` type in `pkg/plugin/models` carrying the `key`, `operator`, `value`, and optional `values` fields the Grafana ad-hoc filter UI sends.

#### Scenario: `AdHocFilter` unmarshals from the Grafana filter wire format

- **GIVEN** a JSON object `{"key": "host", "operator": "=", "value": "prod-1"}`
- **WHEN** it is unmarshalled into a `models.AdHocFilter`
- **THEN** `Key` SHALL be `"host"`, `Operator` SHALL be `"="`, `Value` SHALL be `"prod-1"`, and `Values` SHALL be empty

#### Scenario: `AdHocFilter` supports multi-value operators

- **GIVEN** a JSON object `{"key": "tier", "operator": "=|", "value": "", "values": ["a", "b"]}`
- **WHEN** it is unmarshalled into a `models.AdHocFilter`
- **THEN** `Values` SHALL be `["a", "b"]`

### Requirement: Datasource settings shape `PluginSettings`

The plugin SHALL define a `PluginSettings` type in `pkg/plugin/models` that carries the configuration JSON Grafana sends in `DataSourceInstanceSettings.JSONData`, plus the secret fields (`Password`, `Token`) pulled from `DecryptedSecureJSONData`.

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

### Requirement: Datasource settings constructor `NewPluginSettings`

The plugin SHALL define `NewPluginSettings(ctx context.Context, src backend.DataSourceInstanceSettings) (PluginSettings, error)` that parses, defaults, and validates the settings JSON in one pass.

#### Scenario: `NewPluginSettings` returns validated settings on a valid input

- **GIVEN** a valid `DataSourceInstanceSettings` with `JSONData` containing the required fields
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the returned `PluginSettings` SHALL equal the parsed JSON modulo applied defaults
- **AND** the returned error SHALL be nil

#### Scenario: `NewPluginSettings` propagates a wrapped JSON-parse error

- **GIVEN** a `DataSourceInstanceSettings` with `JSONData == []byte("invalid")`
- **WHEN** `NewPluginSettings` is invoked
- **THEN** the returned error SHALL wrap `ErrorMessageInvalidJSON`

### Requirement: Query setting shape `QuerySetting`

The plugin SHALL define a `QuerySetting` type carrying a `setting` name and a `value` for use both in `PluginSettings.QuerySettings` (datasource-level defaults) and in the per-query JSON's `querySettings` array.

#### Scenario: `QuerySetting` JSON tags match the wire format

- **GIVEN** a JSON object `{"setting": "max_threads", "value": "4"}`
- **WHEN** it is unmarshalled into a `models.QuerySetting`
- **THEN** `Setting` SHALL be `"max_threads"` and `Value` SHALL be `"4"`

### Requirement: Models package has no sqlds dependency

The plugin SHALL implement `pkg/plugin/models` so that the package imports only the Grafana plugin SDK (`backend`, `data/sqlutil`) and the Go standard library. It SHALL NOT import `github.com/hydrolix/sqlds/v5` or its eventual replacement.

#### Scenario: Package imports do not include sqlds

- **GIVEN** the `pkg/plugin/models` source tree
- **WHEN** `goimports -l pkg/plugin/models` and `grep -r 'sqlds' pkg/plugin/models` are executed
- **THEN** there SHALL be no reference to `sqlds` in any file in `pkg/plugin/models`

