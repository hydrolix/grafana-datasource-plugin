## ADDED Requirements

### Requirement: One effective error-exposure configuration

The plugin SHALL resolve the effective error-exposure configuration from `jsonData.exposeErrors` key by key: each key that is missing or `undefined` SHALL take its default (`enabled: true`, `variableName: "hdx_query_errors"`, `maxCount: 5`, `ttl: 300`), and each key that is present SHALL be kept, including `false`, `0` and `""`. The data source's error exposer and the config editor SHALL both use this resolution.

#### Scenario: Missing block resolves to the defaults

- **GIVEN** `jsonData` with no `exposeErrors`
- **WHEN** the configuration is resolved
- **THEN** it SHALL equal `{enabled: true, variableName: "hdx_query_errors", maxCount: 5, ttl: 300}`

#### Scenario: Partial block keeps its keys and fills the rest

- **GIVEN** `exposeErrors: {enabled: false}`
- **WHEN** the configuration is resolved
- **THEN** `enabled` SHALL be `false` and the other keys SHALL be their defaults

#### Scenario: Block without `enabled` resolves to enabled

- **GIVEN** `exposeErrors: {variableName: "errs"}`
- **WHEN** the configuration is resolved
- **THEN** `enabled` SHALL be `true` and `variableName` SHALL be `"errs"`

#### Scenario: Explicit zero is kept

- **GIVEN** `exposeErrors: {maxCount: 0}`
- **WHEN** the configuration is resolved
- **THEN** `maxCount` SHALL be `0`

#### Scenario: The data source's exposer uses the resolved configuration

- **GIVEN** a data source whose `jsonData` has no `exposeErrors`
- **WHEN** the data source is constructed
- **THEN** its error exposer SHALL receive the resolved default configuration

### Requirement: Config editor shows and edits the effective configuration

The config editor's Error Exposure switch, the visibility of its fields, and the field values SHALL reflect the resolved configuration. Every change SHALL write the complete resolved block with the edited key replaced. Rendering without an edit SHALL NOT call `onOptionsChange`.

#### Scenario: Missing block shows error exposure on with defaults

- **GIVEN** an existing data source whose `jsonData` has no `exposeErrors`
- **WHEN** the config editor renders
- **THEN** the Enable Error Exposure switch SHALL be on
- **AND** Variable Name, Max Error Count and Error TTL SHALL show `hdx_query_errors`, `5` and `300`
- **AND** `onOptionsChange` SHALL NOT have been called

#### Scenario: Turning it off writes a complete block

- **GIVEN** an existing data source whose `jsonData` has no `exposeErrors`
- **WHEN** the user turns the switch off
- **THEN** `onOptionsChange` SHALL be called with `exposeErrors` equal to `{enabled: false, variableName: "hdx_query_errors", maxCount: 5, ttl: 300}`

#### Scenario: Stored off stays off

- **GIVEN** `exposeErrors: {enabled: false, variableName: "hdx_query_errors", maxCount: 5, ttl: 300}`
- **WHEN** the config editor renders
- **THEN** the switch SHALL be off and the other fields SHALL be hidden

### Requirement: Errors are written only to an existing dashboard variable

When error exposure is enabled, a query error SHALL be written to the URL parameter `var-<variableName>` only if the current dashboard defines a variable named `variableName`. Otherwise the error SHALL NOT be written and the URL SHALL NOT change.

#### Scenario: No matching variable leaves the URL alone

- **GIVEN** error exposure is enabled and the dashboard has no variable named `hdx_query_errors`
- **WHEN** a query error is reported
- **THEN** `locationService.partial` SHALL NOT be called

#### Scenario: Matching variable receives the error

- **GIVEN** error exposure is enabled and the dashboard has a variable named `hdx_query_errors`
- **WHEN** a query error is reported
- **THEN** `locationService.partial` SHALL be called with `var-hdx_query_errors` holding a JSON array that contains the error message

#### Scenario: Disabled exposure writes nothing

- **GIVEN** `enabled: false` and a dashboard variable named `hdx_query_errors`
- **WHEN** a query error is reported
- **THEN** `locationService.partial` SHALL NOT be called
