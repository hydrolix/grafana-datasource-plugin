## ADDED Requirements

### Requirement: The primary-key check accepts an identity alias of the key column
When no filter references a table's leading primary-key column by name, the validation bar's primary-key check SHALL read the table's `ALIAS` columns from `DESCRIBE TABLE` and SHALL treat a filter on any alias whose expression, bare or backtick-wrapped, is exactly that column as filtering the column. The alias's declared type SHALL NOT be compared. An alias whose expression is any other expression SHALL NOT count. The DESCRIBE SHALL carry the user's request headers, SHALL NOT be cached, and SHALL NOT be issued when the column is filtered by name. A failed alias lookup SHALL skip the check for that table, as a failed primary-key lookup does.

#### Scenario: Summary key column filtered through its alias is valid
- **WHEN** the table's primary key is `` `toStartOfDay(reqTimeSec)` ``, DESCRIBE lists `reqTimeSec DateTime ALIAS` with expression `` `toStartOfDay(reqTimeSec)` ``, and the query filters with `$__timeFilter(reqTimeSec)`
- **THEN** the validation result carries no warning

#### Scenario: Bare-spelled identity alias counts
- **WHEN** the table is keyed on `ts`, DESCRIBE lists `ts_alias` as an `ALIAS` with expression `ts`, and the query filters `ts_alias > 1`
- **THEN** the validation result carries no warning

#### Scenario: A reader filtering through the alias satisfies a CTE body
- **WHEN** a CTE body reads the summary table unfiltered and its only reader filters `reqTimeSec`
- **THEN** the validation result carries no warning

#### Scenario: Computed aliases and aliases of other columns still warn
- **WHEN** the query on the summary table filters only an alias whose expression is `` toDate(`toStartOfDay(reqTimeSec)`) `` or `` countMerge(`count()`) ``
- **THEN** the warning names primary key `toStartOfDay(reqTimeSec)` of that table

#### Scenario: No alias lookup when the key column is filtered by name
- **WHEN** the query filters the leading primary-key column by name
- **THEN** no DESCRIBE is issued for that table

#### Scenario: A failed alias lookup skips the check
- **WHEN** the key column is unfiltered and the DESCRIBE fails
- **THEN** the validation result carries no primary-key warning for that table and the skip is logged
