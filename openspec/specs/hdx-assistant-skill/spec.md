# hdx-assistant-skill Specification

## Purpose
The Assistant Skill document contract: the Hydrolix dialect rules it must state, the macro and ad-hoc-filter translations it must carry, error triage, which MCP tools it auto-approves, and the operator documentation requirements. Delivered by change add-assistant-mcp-support.
## Requirements

### Requirement: The Skill states the Hydrolix dialect rules

The Skill document SHALL state the Hydrolix behaviors that differ from stock
ClickHouse and that cause wrong or failing queries when assumed away. It SHALL
cover: the cluster's rejection of queries lacking a `WHERE` time filter; that
`is_in_primary_key` and `is_in_sorting_key` are false for every column in
`system.columns` so the primary key must be read from
`system.tables.primary_key`; that every column reports as nullable and this is
not to be trusted; that `SETTINGS max_execution_time = N` is accepted; and that
the storage engine reports as `TurbineStorage`.

#### Scenario: Generated query satisfies the time-range guard

- **WHEN** the Skill is in effect and the model writes a query against a Hydrolix table
- **THEN** the query includes an explicit time filter in its `WHERE` clause

#### Scenario: Primary key sourced correctly

- **WHEN** the model needs a table's primary time column
- **THEN** it reads `system.tables.primary_key` rather than relying on `is_in_primary_key`

#### Scenario: Nullability is not over-reported to the user

- **WHEN** the model describes a table's schema
- **THEN** it does not assert that every column is nullable on the basis of the reported metadata

### Requirement: The Skill instructs explicit time bounds over macros

The Skill SHALL instruct the model to emit literal time bounds in generated
SQL rather than Grafana macros, because the MCP server performs no macro
expansion.

#### Scenario: Generated SQL contains no macros

- **WHEN** the model writes a new query for execution through the MCP server
- **THEN** the query contains literal time bounds and no `$__` macro tokens

### Requirement: The Skill carries a macro translation reference

The Skill SHALL provide translations for the plugin's macros so the model can
read and reason about saved panel queries. It SHALL cover at minimum
`$__timeFilter` and `$__conditionalAll`.

#### Scenario: Reading a saved panel query

- **WHEN** a user pastes a panel query containing `$__timeFilter(timestamp)` into Assistant
- **THEN** the model can explain the query and produce an executable equivalent with literal bounds

### Requirement: Read-only MCP tools are auto-approved

The Skill SHALL auto-approve the MCP server's read-only discovery tools —
`list_databases`, `list_tables`, and `get_table_info` — and SHALL NOT
auto-approve `run_select_query`, leaving that to the operator.

#### Scenario: Discovery runs without prompting

- **WHEN** the Skill is invoked and the model calls `list_tables`
- **THEN** the call executes without a manual approval prompt

#### Scenario: Query execution requires a decision

- **WHEN** the model calls `run_select_query` under the Skill's default configuration
- **THEN** the call is subject to the deployment's approval setting rather than being auto-approved by the Skill

### Requirement: The Skill is provisionable and within platform limits

The Skill SHALL be maintained as a versioned document in this repository so it
can be provisioned as code and tracked against dialect changes. Its content
SHALL remain within the Assistant Skill size limit.

#### Scenario: Skill fits the platform limit

- **WHEN** the Skill document is validated
- **THEN** its content is under 64KB

#### Scenario: Skill is provisionable without manual authoring

- **WHEN** an operator provisions the Skill via the documented mechanism
- **THEN** the repository's Skill content is used verbatim, with no copy-paste editing step

### Requirement: Operator documentation states the identity and reachability model

The documentation SHALL state, before the registration steps, that queries
issued through the MCP server bypass Grafana datasource permissions and the
plugin's per-user OAuth forwarding, and that the MCP server must be reachable
from the Grafana instance. It SHALL describe the scope trade-off between a
personal and an organization-wide server registration.

#### Scenario: Reachability verified before registration

- **WHEN** an operator follows the guide
- **THEN** they verify the MCP server is reachable from the Grafana instance before registering it

#### Scenario: Same-cluster requirement stated

- **WHEN** an operator registers the MCP server
- **THEN** the guide requires confirming it targets the same cluster as the Hydrolix datasource

#### Scenario: Auth exposure is disclosed

- **WHEN** an operator reads the guide
- **THEN** they are told that an organization-wide registration lets any Assistant user query the cluster as the server's identity, including users without access to the Hydrolix datasource in Grafana
