## ADDED Requirements

### Requirement: Config editor renders every stored query setting

The data source config editor SHALL render the Query Settings section for any content of `jsonData.querySettings`, including entries whose `setting` name is not in the plugin's QuerySettings catalog (`labels.components.querySettings.values`). Rendering SHALL NOT throw.

#### Scenario: Unknown setting name does not crash the page

- **GIVEN** a data source whose `jsonData.querySettings` is `[{ setting: "hdx_query_unknown", value: "10" }]`
- **WHEN** the config editor renders
- **THEN** the page SHALL render without throwing
- **AND** the Query Settings section SHALL show a row labelled `hdx_query_unknown`

#### Scenario: Known and unknown settings render side by side

- **GIVEN** `jsonData.querySettings` holds `hdx_query_max_rows` with value `"5"` and `hdx_query_unknown` with value `"10"`
- **WHEN** the config editor renders
- **THEN** both rows SHALL be shown
- **AND** the `hdx_query_max_rows` row SHALL show its catalog description

#### Scenario: Malformed content does not crash the page

- **GIVEN** `jsonData.querySettings` is not a list, or is a list holding entries that are not objects (for example `null`)
- **WHEN** the config editor renders
- **THEN** the Query Settings section SHALL render without throwing
- **AND** an entry that is not an object SHALL have no row

#### Scenario: Entries that are not objects are dropped on the first edit

- **GIVEN** `jsonData.querySettings` is `[null, { setting: "hdx_query_unknown", value: "10" }]`
- **WHEN** the user changes the `hdx_query_unknown` input to `20`
- **THEN** `onOptionsChange` SHALL be called with `jsonData.querySettings` equal to `[{ setting: "hdx_query_unknown", value: "20" }]`

### Requirement: Unknown query setting is shown as an editable text value with a warning

A stored query setting whose name is not in the catalog SHALL be rendered as a text input holding its stored value. Its description SHALL be a warning that the plugin does not know the setting and sends it to Hydrolix as-is. The entry SHALL NOT be marked invalid and its value SHALL NOT be validated.

#### Scenario: Unknown setting shows its value and the warning

- **GIVEN** `jsonData.querySettings` is `[{ setting: "hdx_query_unknown", value: "10" }]`
- **WHEN** the config editor renders
- **THEN** a text input labelled `hdx_query_unknown` SHALL hold `10`
- **AND** the row SHALL show the unknown-setting warning
- **AND** the row SHALL NOT show a validation error

#### Scenario: Unknown setting named like a built-in object member

- **GIVEN** `jsonData.querySettings` is `[{ setting: "constructor", value: "1" }]`
- **WHEN** the config editor renders
- **THEN** the row SHALL show the unknown-setting warning
- **AND** the row SHALL NOT show a validation error

#### Scenario: Editing an unknown setting updates jsonData

- **GIVEN** the unknown setting `hdx_query_unknown` with value `"10"`
- **WHEN** the user changes the input to `20`
- **THEN** `onOptionsChange` SHALL be called with `jsonData.querySettings` containing `{ setting: "hdx_query_unknown", value: "20" }`

### Requirement: Unknown query setting can be removed

The remove button of an unknown query setting SHALL remove that entry from `jsonData.querySettings` and leave the other entries unchanged, including entries with the same `setting` name.

#### Scenario: Removing an unknown setting

- **GIVEN** `jsonData.querySettings` holds `hdx_query_max_rows` and `hdx_query_unknown`
- **WHEN** the user clicks the remove button of `hdx_query_unknown`
- **THEN** `onOptionsChange` SHALL be called with `jsonData.querySettings` equal to the `hdx_query_max_rows` entry only

#### Scenario: Removing one of two entries with the same name

- **GIVEN** `jsonData.querySettings` holds two entries named `hdx_query_unknown`, with values `"1"` and `"2"`
- **WHEN** the user clicks the remove button of the second row
- **THEN** `onOptionsChange` SHALL be called with `jsonData.querySettings` equal to the first entry only

### Requirement: A query setting left empty is removed and never sent

When the text input of a query setting, known or unknown, loses focus while empty, the config editor SHALL remove that entry from `jsonData.querySettings`. A query setting whose value is the empty string SHALL NOT be sent to Hydrolix with a query.

#### Scenario: Empty setting is removed on blur

- **GIVEN** `jsonData.querySettings` holds `hdx_query_unknown` with value `""` and one other entry
- **WHEN** the `hdx_query_unknown` input loses focus
- **THEN** `onOptionsChange` SHALL be called with `jsonData.querySettings` equal to the other entry only

#### Scenario: Setting with a value is kept on blur

- **GIVEN** `jsonData.querySettings` holds `hdx_query_unknown` with value `"10"`
- **WHEN** its input loses focus
- **THEN** `onOptionsChange` SHALL NOT be called

#### Scenario: Stored empty value is not sent

- **GIVEN** a query whose merged settings are `hdx_query_max_rows` = `"100"` and `hdx_query_unknown` = `""`
- **WHEN** the backend builds the query context
- **THEN** the context SHALL carry `hdx_query_max_rows` only
