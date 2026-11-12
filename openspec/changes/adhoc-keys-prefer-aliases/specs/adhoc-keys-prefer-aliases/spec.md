## ADDED Requirements

### Requirement: Ad-hoc key discovery resolves identity aliases in both DESCRIBE spellings
The metadata provider's ad-hoc key parsing SHALL resolve an `ALIAS` column's target from `default_expression` whether the expression is backtick-wrapped (`` `cliIP` ``) or bare (`cliIP`). An alias whose target is an existing column SHALL take that column's type. An alias whose whole expression is not a column name (a computed alias) SHALL be excluded in either spelling. This includes a computed alias whose expression contains a backticked column (``lower(`cliIP`)``), which the previous parser listed under the inner column's type.

#### Scenario: Bare identity alias is listed with its target's type
- **WHEN** DESCRIBE returns `cliIP String` and `client_ip String ALIAS` with expression `cliIP`
- **THEN** the key list contains both `cliIP` and `client_ip`, each with type `String`

#### Scenario: Backtick-wrapped identity alias is listed with its target's type
- **WHEN** DESCRIBE returns `cliIP String` and `client_ip String ALIAS` with expression `` `cliIP` ``
- **THEN** the key list contains both `cliIP` and `client_ip`, each with type `String`

#### Scenario: Computed alias is excluded in either spelling
- **WHEN** DESCRIBE returns an `ALIAS` column whose expression is `lower(cliIP)`, ``lower(`cliIP`)`` or `` `toStartOfHour(datetime)` `` (with no column of that name)
- **THEN** that column is absent from the key list

### Requirement: Keys expose the identity-alias relation
Each ad-hoc filter key SHALL carry `aliasFor` naming its physical target when, and only when, the key is an `ALIAS` column, the target exists, the target is not itself an alias, and the key's declared type equals the target's type.

#### Scenario: Same-type identity alias carries aliasFor
- **WHEN** `client_ip String ALIAS cliIP` and `cliIP String` are both present
- **THEN** the `client_ip` key has `aliasFor` equal to `cliIP` and the `cliIP` key has no `aliasFor`

#### Scenario: Type-changing, missing-target and chained aliases carry no aliasFor
- **WHEN** DESCRIBE returns `cast IPv4 ALIAS cliIP` (base is `String`), `missing String ALIAS unknown`, and `a String ALIAS b` with `b String ALIAS a`
- **THEN** none of those keys has `aliasFor`

### Requirement: The preference policy hides a base only behind a same-type identity alias
A pure helper SHALL return the key list minus every key named by some other key's `aliasFor`, preserving order and every other key unchanged.

#### Scenario: Base hidden, unrelated columns and Map columns kept
- **WHEN** the keys are `cliIP`, `unrelatedPhysical`, `client_ip` (aliasFor `cliIP`), `attrs Map(String, String)`
- **THEN** the result is `unrelatedPhysical`, `client_ip`, `attrs` in that order

#### Scenario: Multiple aliases of one base all remain
- **WHEN** `client_ip` and `src_ip` both have `aliasFor` `cliIP`
- **THEN** the result contains `client_ip` and `src_ip` and not `cliIP`

#### Scenario: Keys without aliasFor pass through unchanged
- **WHEN** no key carries `aliasFor`
- **THEN** the result equals the input

### Requirement: Opt-in ad-hoc keys prefer identity aliases without losing Map or quoted-table support
When the datasource option `adHocKeysPreferAliases` is `true`, `getTagKeys` SHALL apply the preference policy to the memoized key list of the ad-hoc table and SHALL then enumerate Map keys exactly as when the option is off. When the option is absent or `false`, `getTagKeys` SHALL apply no preference: it SHALL offer every key the parsing requirement above produces, with Map keys enumerated the same way. `getTagValues` SHALL resolve values for a hidden base exactly as for any other key.

#### Scenario: Alias offered instead of base, Map keys still enumerated
- **WHEN** the option is on and the ad-hoc table has `cliIP`, `client_ip ALIAS cliIP`, `customerField`, and `attrs Map(String, String)` with keys `env`
- **THEN** the keys offered are `client_ip`, `customerField`, and `attrs['env']`, and not `cliIP`

#### Scenario: Quoted view name works under the opt-in
- **WHEN** the option is on and the ad-hoc table variable resolves to `logs#redacted` in default database `akamai`
- **THEN** keys are read from ``DESCRIBE akamai.`logs#redacted` `` and no error is raised

#### Scenario: Single DESCRIBE per table across dropdown opens
- **WHEN** the option is on and `getTagKeys` is called twice for the same table
- **THEN** the cluster receives one `DESCRIBE` for that table

#### Scenario: Hidden base still yields values
- **WHEN** the option is on and `getTagValues` is called for `cliIP`
- **THEN** the value preload statement selects `cliIP` from the table with the standard time window, filters and guardrails

#### Scenario: Option off hides nothing
- **WHEN** the option is absent
- **THEN** `getTagKeys` returns every key of the parsed key list, in DESCRIBE order, including both an identity alias and its base

### Requirement: The option is exposed as a switch in the datasource config editor
The datasource configuration editor SHALL show an "Ad hoc filter keys prefer aliases" switch in the additional settings section, next to the ad-hoc table variable. The switch SHALL reflect `jsonData.adHocKeysPreferAliases`, SHALL be off for a datasource that has never set it, and toggling it SHALL write the boolean to `jsonData` without touching other options.

#### Scenario: Switch is hidden until additional settings expand and defaults off
- **WHEN** a new datasource config page opens and the additional settings are expanded
- **THEN** the switch becomes visible and is unchecked

#### Scenario: Toggling writes the option
- **WHEN** the switch is turned on
- **THEN** the editor emits options whose `jsonData.adHocKeysPreferAliases` is `true` and every other `jsonData` field is unchanged

### Requirement: Selecting an alias does not rewrite SQL
Choosing an alias or its base in an ad-hoc filter SHALL emit that exact column name in generated SQL; the plugin SHALL NOT substitute one name for the other.

#### Scenario: Alias filter reaches SQL by its own name
- **WHEN** an ad-hoc filter `client_ip = 192.0.2.20` is applied under the opt-in
- **THEN** the generated filter condition references `client_ip`, and a panel querying `cliIP` directly keeps `cliIP`
