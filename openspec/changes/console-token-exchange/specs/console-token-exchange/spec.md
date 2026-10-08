# console-token-exchange Specification (delta)

## ADDED Requirements

### Requirement: A credentials type that exchanges the forwarded token

The plugin SHALL offer a credentials type `forwardOAuthExchange`. In it the
plugin SHALL obtain a cluster token by presenting the signed-in user's
forwarded token to an RFC 8693 token-exchange endpoint, and SHALL use the
result as the query credential. The user's forwarded token SHALL NOT be sent
to the cluster, including when the exchange fails.

Like `forwardOAuth`, selecting it SHALL set `oauthPassThru`, because Grafana
forwards a sign-in token only when the data source asks it to.

#### Scenario: The forwarded token never reaches the cluster

- **GIVEN** a data source in `forwardOAuthExchange` mode
- **WHEN** a query runs for a signed-in user
- **THEN** the request to the cluster SHALL carry the exchanged token
- **AND** it SHALL NOT carry the forwarded sign-in token

#### Scenario: A failed exchange sends nothing

- **GIVEN** a data source in `forwardOAuthExchange` mode
- **AND** an exchange endpoint that refuses
- **WHEN** a query runs
- **THEN** the query SHALL fail
- **AND** no request carrying the forwarded sign-in token SHALL reach the cluster

### Requirement: The delegate credential is operator configuration, keyed by audience

The exchange endpoint and its credentials SHALL be read from the process
environment — `GF_PLUGIN_EXCHANGE_URL` and `GF_PLUGIN_EXCHANGE_CREDENTIALS`,
or `HDX_EXCHANGE_URL` and `HDX_EXCHANGE_CREDENTIALS` — and SHALL NOT be read
from `jsonData` or `secureJsonData`. Credentials SHALL be a map keyed by
cluster audience, so that one Grafana serving several clusters holds a
separate credential per cluster and each credential is scoped to the one
cluster it can obtain a token for.

A data source's audience SHALL be its `jsonData.exchangeAudience` when set,
and its configured host otherwise.

#### Scenario: Each cluster's credential is its own

- **GIVEN** credentials configured for audiences `a.example` and `b.example`
- **WHEN** a data source whose audience is `a.example` exchanges
- **THEN** the exchange SHALL present the credential configured for `a.example`

#### Scenario: A cluster with no credential is not a query failure

- **GIVEN** a data source whose audience has no configured credential
- **WHEN** its health is checked
- **THEN** the check SHALL report that this Grafana holds no credential for
  that cluster

### Requirement: One exchange per user, reused until shortly before expiry

The plugin SHALL cache an exchanged token per subject, keyed on the subject
claim and never on the forwarded token, which changes when Grafana refreshes
it. Concurrent requests for one subject SHALL result in a single exchange in
flight. Expiry SHALL be computed from the moment the exchange request began,
never from a server-supplied clock.

#### Scenario: A dashboard load causes one exchange

- **GIVEN** a dashboard of many panels for one signed-in user
- **WHEN** it loads
- **THEN** the plugin SHALL perform one exchange for that user
- **AND** every panel's query SHALL use its result

#### Scenario: A refreshed sign-in token does not force a new exchange

- **GIVEN** a cached token for a subject
- **WHEN** the same subject arrives with a newly refreshed forwarded token
- **THEN** the cached token SHALL still be used

### Requirement: The mode requires the HTTP protocol

The plugin SHALL refuse `forwardOAuthExchange` on the native protocol, with a
message naming the HTTP protocol as the requirement. Native binds its
credential to the connection as a password, which cannot be replaced when the
token is refreshed.

#### Scenario: Native is refused at connect

- **GIVEN** a data source in `forwardOAuthExchange` mode on the native protocol
- **WHEN** a connection is established
- **THEN** it SHALL fail with a message naming the HTTP protocol

### Requirement: Health reports what configuration establishes, not a user's access

For both forwarding modes, the health check SHALL NOT verify the bootstrap
connection, which carries no signed-in user. For `forwardOAuthExchange` it
SHALL report whether this Grafana holds a credential for this data source's
cluster, and SHALL state that an individual's access is established when a
panel runs.

#### Scenario: A working forward-mode data source is not reported degraded

- **GIVEN** a correctly configured data source in either forwarding mode
- **WHEN** Save & test runs
- **THEN** the result SHALL NOT be a failure caused by the absence of a
  signed-in user
