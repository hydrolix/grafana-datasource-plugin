## MODIFIED Requirements

### Requirement: Query setting shape `QuerySetting`

The plugin SHALL define a `QuerySetting` type carrying a `setting` name and a `value` for use both in `PluginSettings.QuerySettings` (datasource-level defaults) and in the per-query JSON's `querySettings` array.

Unmarshalling a `QuerySetting` from JSON SHALL read `setting` and `value` the way `NewPluginSettings` reads a stored entry: a JSON string as-is, a JSON number as its shortest plain decimal text, a JSON boolean as `"true"` or `"false"`, and a missing or `null` field as `""`. Any other JSON type, or an entry that is not a JSON object, SHALL return an error that wraps `ErrorMessageInvalidSettingType` and does not contain the value.

When a query's `querySettings` cannot be read, `MutateQueryData` SHALL log a warning and still run the query.

`MutateQueryData` SHALL leave a per-query entry whose `setting` is missing or empty out of the merged settings, the way `NewPluginSettings` skips a stored one.

#### Scenario: A nameless per-query entry is dropped

- **GIVEN** a query whose JSON carries `querySettings: [{"value": "5"}, {"setting": "hdx_query_max_rows", "value": "100"}]`
- **WHEN** `MutateQueryData` and then `MutateQuery` process the query
- **THEN** the query's `querySettings` and the query context SHALL carry only `hdx_query_max_rows` = `"100"`

#### Scenario: `QuerySetting` JSON tags match the wire format

- **GIVEN** a JSON object `{"setting": "max_threads", "value": "4"}`
- **WHEN** it is unmarshalled into a `models.QuerySetting`
- **THEN** `Setting` SHALL be `"max_threads"` and `Value` SHALL be `"4"`

#### Scenario: A per-query numeric value overrides the data source's value

- **GIVEN** a data source whose `querySettings` hold `hdx_query_max_execution_time` = `"60"`
- **AND** a query whose JSON carries `querySettings: [{"setting": "hdx_query_max_execution_time", "value": 30}]`
- **WHEN** `MutateQueryData` and then `MutateQuery` process the query
- **THEN** the query context SHALL carry `hdx_query_max_execution_time` = `"30"`

#### Scenario: An unsupported value type returns an error

- **GIVEN** a JSON object `{"setting": "hdx_query_max_rows", "value": {"n": 1}}`
- **WHEN** it is unmarshalled into a `models.QuerySetting`
- **THEN** the returned error SHALL wrap `ErrorMessageInvalidSettingType`
