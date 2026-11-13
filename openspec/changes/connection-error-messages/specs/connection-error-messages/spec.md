## ADDED Requirements

### Requirement: A connection failure is reported by a fixed message

When the plugin cannot establish or verify a connection to the configured address, the error it returns SHALL be one of a fixed set of messages chosen by the kind of failure. The message SHALL NOT contain the host, an IP address, a port, the resolver's address, the request URL, the user name, a response body, an exception's text, or a redirect target.

The messages SHALL be:

| Failure | Message |
|---|---|
| Host name does not resolve | `Host name could not be resolved` |
| Connection refused | `Connection refused` |
| Timeout while connecting | `Connection timed out` |
| TLS handshake or certificate failure, or a plain answer to a TLS client | `TLS handshake failed` |
| HTTP 401 or 403, or ClickHouse code 192, 193, 194 or 516 | `Authentication failed` |
| Any other HTTP status | `The server answered HTTP <status>` |
| Any other ClickHouse exception | `The server refused the connection (ClickHouse error code <code>)` |
| Anything else | `Could not connect to the server` |

The returned error SHALL be a downstream error and SHALL unwrap to the original error.

#### Scenario: A refused connection fails the instance with the fixed message

- **GIVEN** a data source in a stored-credential mode whose host and port accept no connection
- **WHEN** the instance is built
- **THEN** the build SHALL fail with the message `Connection refused`
- **AND** the error SHALL be a downstream error
- **AND** the message SHALL NOT contain the host or the port

#### Scenario: An HTTP service that is not a cluster is reported by status only

- **GIVEN** a data source using the HTTP protocol whose address answers `404` with a body
- **WHEN** the instance is built
- **THEN** the build SHALL fail with the message `The server answered HTTP 404`
- **AND** the message SHALL NOT contain any part of the body

#### Scenario: A rejected credential is reported the same way on both protocols

- **GIVEN** an address that answers the handshake with HTTP 403, or with ClickHouse exception code 516
- **WHEN** the instance is built
- **THEN** the build SHALL fail with the message `Authentication failed`

#### Scenario: A TLS client reaching a plain port is a TLS failure

- **GIVEN** a data source with TLS enabled whose address speaks plain HTTP
- **WHEN** the instance is built
- **THEN** the build SHALL fail with the message `TLS handshake failed`

#### Scenario: An unrecognised failure gets the generic message

- **GIVEN** a connection error that matches no listed kind
- **WHEN** it is converted for a person
- **THEN** the message SHALL be `Could not connect to the server`

#### Scenario: The original error stays reachable

- **GIVEN** a connection error converted to a fixed message
- **WHEN** `errors.Is` is asked whether the result is the original error
- **THEN** it SHALL report true

### Requirement: The health check of a built instance uses the same messages

When the health check of a data source in a stored-credential mode returns an error status, its message SHALL be one of the fixed messages. A message the plugin does not recognise SHALL be replaced by `Could not connect to the server`.

#### Scenario: A cluster lost after the instance was built

- **GIVEN** a data source instance that connected successfully
- **AND** its cluster no longer accepts connections
- **WHEN** the health check runs
- **THEN** the status SHALL be an error
- **AND** the message SHALL be `Connection refused`
- **AND** the message SHALL NOT contain the host or the port

### Requirement: A query that never reached the cluster uses the same messages

When a query fails while resolving, dialling, in the TLS handshake, or on a timeout, the error shown for it SHALL be the fixed message for that kind. A timeout that is not a dial timeout SHALL be reported as `The request timed out`. The error SHALL remain a downstream error and SHALL unwrap to the original, so that a query error and an exceeded deadline are still recognised as such.

An error that carries an answer from the cluster, a ClickHouse exception or an HTTP status with a body, SHALL NOT be replaced by this requirement.

#### Scenario: A forwarding-mode query to a closed port

- **GIVEN** a data source in a credentials mode that forwards the signed-in user's identity
- **AND** its host and port accept no connection
- **WHEN** a query runs
- **THEN** the query's error SHALL be `Connection refused`
- **AND** its error source SHALL be downstream

#### Scenario: A deadline is still a deadline

- **GIVEN** a query error that wraps `context.DeadlineExceeded` inside a URL error
- **WHEN** it is converted for the panel
- **THEN** the message SHALL be `The request timed out`
- **AND** `errors.Is(err, context.DeadlineExceeded)` SHALL report true

#### Scenario: A SQL error from the cluster is shown as before

- **GIVEN** a query error whose text is an HTTP 400 with a ClickHouse syntax error in the body
- **WHEN** it is converted for the panel
- **THEN** the message SHALL still contain the syntax error

### Requirement: A refused redirect is reported by its status alone

The message of a refused redirect SHALL name the HTTP status and SHALL NOT contain any part of the `Location` it pointed to.

#### Scenario: A redirect to another host

- **GIVEN** an address that answers `302` with a `Location` on another host
- **WHEN** the plugin refuses the redirect
- **THEN** the message SHALL contain `HTTP 302`
- **AND** the message SHALL NOT contain the other host or its path

### Requirement: The original error is logged

Each connection failure replaced by a fixed message SHALL be written to the plugin's log with the original error. A refused redirect SHALL be logged with its status and its redacted location.

#### Scenario: A failed connect is logged once

- **GIVEN** a data source whose host and port accept no connection
- **WHEN** the instance build fails
- **THEN** the plugin log SHALL contain one error entry carrying the original error text
