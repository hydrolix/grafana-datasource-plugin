## MODIFIED Requirements

### Requirement: `HdxSqlDatasource` wrapper embeds `*sqlds.SQLDatasource`

The plugin SHALL define `HdxSqlDatasource` in `pkg/plugin/hdx_sqlds.go` as a struct that embeds `*sqlds.SQLDatasource` and exposes the embedded pointer's promoted methods (`QueryData`, `Dispose`, `GetDBFromQuery`, etc.) without delegation.

The wrapper SHALL define its own `CheckHealth`. For a stored-credential mode it SHALL call the embedded `CheckHealth` and return its result, replacing the message of an error result as the `connection-error-messages` capability requires.

#### Scenario: Wrapper promotes upstream methods

- **GIVEN** an `*HdxSqlDatasource` value
- **WHEN** any method defined on `*sqlds.SQLDatasource`, other than `CheckHealth`, is invoked
- **THEN** the call SHALL succeed without a compile-time error or a runtime panic
- **AND** the receiver inside the upstream method SHALL be the embedded `*sqlds.SQLDatasource`

#### Scenario: A healthy stored-credential data source reports the upstream result

- **GIVEN** an `*HdxSqlDatasource` in a stored-credential mode whose cluster answers
- **WHEN** `CheckHealth` is invoked
- **THEN** the result SHALL be the embedded `CheckHealth` result, unchanged
