## ADDED Requirements

### Requirement: Assistant surfaces are inert when Assistant is unavailable

The plugin SHALL determine Assistant availability before rendering any
Assistant-related UI or publishing any context. When the Assistant app is not
installed, is disabled, or the current user lacks access, the plugin SHALL
render no Assistant UI and SHALL NOT publish page context.

#### Scenario: Assistant app absent

- **WHEN** the QueryEditor renders on a Grafana instance without the Assistant app
- **THEN** no Assistant entry point is present in the DOM
- **AND** no page context is registered

#### Scenario: Assistant becomes available after initial render

- **WHEN** availability resolves to available after the editor has mounted
- **THEN** the Assistant entry point appears
- **AND** page context is published for the current query state

#### Scenario: Availability check fails

- **WHEN** the availability check errors or never resolves
- **THEN** the plugin treats Assistant as unavailable
- **AND** the QueryEditor renders and functions exactly as it does today

### Requirement: QueryEditor publishes Hydrolix query context

While Assistant is available, the QueryEditor SHALL publish a structured
context item describing the user's current query state. The payload SHALL
include the raw SQL, the resolved database and table when determinable, the
active time range, the column schema known to the metadata provider, the
primary time column, and the datasource's configured host.

#### Scenario: Context published for a query with a resolved table

- **WHEN** the user has a query naming a table whose schema the metadata provider holds
- **THEN** the published context contains that database and table, its column schema, and the primary time column

#### Scenario: Context published before a table is resolvable

- **WHEN** the query is empty, does not parse, or names no resolvable table
- **THEN** context is still published with the SQL, time range, and datasource host
- **AND** the table, schema, and primary-time-column fields are absent rather than empty placeholders

#### Scenario: Unqualified table resolved against the default database

- **WHEN** the query names a table without a database and the datasource has a default database configured
- **THEN** the published context and the schema lookup use the default database as the table's schema

#### Scenario: Datasource host is always present

- **WHEN** any context item is published
- **THEN** it includes the datasource's configured host, so a mismatch against the MCP server's cluster is detectable

#### Scenario: Context tracks query edits

- **WHEN** the user changes the SQL or the dashboard time range
- **THEN** the published context is updated to reflect the new state

### Requirement: Context publication never blocks the editor

Building and publishing Assistant context SHALL run asynchronously and
debounced, off the editor's render and input paths. Table structure SHALL be
resolved by the backend SQL parser, not by pattern-matching raw SQL. Schema
lookups SHALL go through the metadata provider's memoized fetchers so the
cluster is queried at most once per table per session, and out-of-order
completions SHALL NOT overwrite a newer publication.

#### Scenario: Editing is never gated on publication

- **WHEN** the user types while a context parse or schema fetch is in flight
- **THEN** editor input, autocomplete, and query execution proceed without waiting on it

#### Scenario: Repeated publications reuse fetched schema

- **WHEN** context is republished for a table whose schema was already fetched
- **THEN** no additional cluster query is issued for that table

#### Scenario: Stale completion is discarded

- **WHEN** a parse or schema fetch for an older query state resolves after a newer publication
- **THEN** the newer published context is not overwritten

### Requirement: QueryEditor offers an Assistant entry point

While Assistant is available, the QueryEditor SHALL present a control that
opens Assistant with the current datasource and query attached.

#### Scenario: Opening Assistant from the editor

- **WHEN** the user activates the Assistant control with a non-empty query
- **THEN** Assistant opens with the datasource and the current query as context

### Requirement: Query errors can be explained by Assistant

The QueryEditor SHALL offer an explain action when the panel's latest response
carries an error for the editor's query and Assistant is available, opening
Assistant with the failing SQL and the error text. When the error matches a
curated solution template, the prompt SHALL also carry the classification and
the rendered remediation guidance. The prompt SHALL NOT be sent without the
user's confirmation, and the action SHALL NOT replace or suppress the existing
error reporting.

#### Scenario: Explaining a time-range guard rejection

- **WHEN** a query fails with the cluster's `hdx_query_timerange_required` message and the user activates the explain action
- **THEN** Assistant opens with an unsent prompt containing the failing SQL, the error text, and the template's remediation guidance

#### Scenario: Unclassified errors still get an explanation

- **WHEN** a query fails with a message no solution template matches
- **THEN** the explain action is still offered, with the SQL and error text alone

#### Scenario: The action is scoped to the failing query

- **WHEN** the panel response carries an error for a different query's refId
- **THEN** this editor offers no explain action

#### Scenario: Existing error handling is preserved

- **WHEN** a query fails
- **THEN** the error is reported and logged exactly as it is today, whether or not Assistant is available

### Requirement: No change to the query pipeline

Publishing context and opening Assistant SHALL NOT mutate the query, the
`DataQueryRequest`, or any cached request state on the datasource instance.

#### Scenario: Query execution is unaffected

- **WHEN** Assistant context is published and the user then runs the query
- **THEN** the request sent to the backend is identical to the request sent with Assistant unavailable
