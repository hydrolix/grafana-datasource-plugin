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
environment as `GF_PLUGIN_EXCHANGE_URL` and `GF_PLUGIN_EXCHANGE_CLIENT_SECRETS`,
and from no other name, and SHALL NOT be read from `jsonData` or
`secureJsonData`. Credentials SHALL be a map keyed by
cluster audience, so that one Grafana serving several clusters holds a
separate credential per cluster and each credential is scoped to the one
cluster it can obtain a token for. The credentials key SHALL be named so
that Grafana masks its value wherever it shows its own configuration
(`/api/admin/settings`, the startup log of environment overrides): Grafana
masks a setting only when the key matches one of `setting.RedactedValue`'s
patterns, and `exchange_client_secrets` matches `SECRET`.

Those environment variables SHALL be supplied through a
`[plugin.hydrolix-hydrolix-datasource]` section in Grafana's configuration
file, whose keys Grafana renders into the plugin's environment with a
`GF_PLUGIN_` prefix. Grafana builds a minimal environment for a backend plugin
process rather than passing its own, so a variable set on the Grafana process
itself does not reach the plugin (measured on Grafana 13.0.3 Enterprise).

A data source's audience SHALL be its configured host, and SHALL NOT be a
separate setting: an audience that could differ from the host would mint a
real cluster's token for every viewer and send it to whatever server the host
names.

#### Scenario: Each cluster's credential is its own

- **GIVEN** credentials configured for audiences `a.example` and `b.example`
- **WHEN** a data source whose audience is `a.example` exchanges
- **THEN** the exchange SHALL present the credential configured for `a.example`

#### Scenario: A cluster with no credential is the operator's problem, not the person's

- **GIVEN** a data source whose audience has no configured credential
- **WHEN** a query exchanges for it
- **THEN** the failure SHALL read as a misconfiguration rather than as a
  refusal of the person, and SHALL NOT name the cluster or say that the
  credential for it is the part that is missing

#### Scenario: Health does not reveal which clusters hold a credential

- **GIVEN** credentials configured for audience `a.example` only
- **WHEN** the health of a data source whose audience is `a.example` is
  checked, and the health of one whose audience is `b.example` is checked
- **THEN** both checks SHALL return the same status and the same message,
  so that someone who can edit a data source cannot learn which clusters
  this Grafana serves by naming them

### Requirement: One exchange per sign-in token, reused until shortly before expiry

The plugin SHALL cache an exchanged token per subject, in a slot keyed on the
token's issuer and subject claims together, and SHALL answer a cached token
only to the exact forwarded token it was minted from: on a hit, the console
verified that very token when it minted the entry, so a hit never stands in
for a verification the console did not do. Concurrent requests carrying one
forwarded token SHALL result in a single exchange in flight. Expiry SHALL be
computed from the moment the exchange request began, never from a
server-supplied clock.

#### Scenario: A dashboard load causes one exchange

- **GIVEN** a dashboard of many panels for one signed-in user
- **WHEN** it loads
- **THEN** the plugin SHALL perform one exchange for that user
- **AND** every panel's query SHALL use its result

#### Scenario: A refreshed sign-in token is exchanged afresh

- **GIVEN** a cached token for a subject
- **WHEN** the same subject arrives with a newly refreshed forwarded token
- **THEN** the plugin SHALL perform one exchange for the new token
- **AND** SHALL NOT serve the cached token to it, usable or stale

#### Scenario: A dashboard load that straddles a refresh makes one exchange per token

- **GIVEN** panels of one dashboard arriving with two forwarded tokens for one
  subject, the one before and the one after a Grafana refresh
- **WHEN** both are in flight at once
- **THEN** the plugin SHALL perform one exchange per forwarded token
- **AND** the panels carrying the second token SHALL NOT be handed the result
  minted for the first

#### Scenario: A forwarded token nobody verified is never served the token in hand

- **GIVEN** a cached token for a subject, inside its refresh window
- **WHEN** the same subject arrives with a different forwarded token and the
  exchange for it fails transiently
- **THEN** the plugin SHALL fail that request rather than serve the cached
  token, which was minted for a token the console verified and this one is not

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
SHALL report whether this Grafana has an exchange configuration at all, SHALL
NOT report whether that configuration holds a credential for this data
source's cluster (the credential map is instance-wide, and a per-host answer
would tell an org admin on a shared Grafana which other customers' clusters it
serves), and SHALL state that an individual's access is established when a
panel runs.

#### Scenario: A working forward-mode data source is not reported degraded

- **GIVEN** a correctly configured data source in either forwarding mode
- **WHEN** Save & test runs
- **THEN** the result SHALL NOT be a failure caused by the absence of a
  signed-in user
