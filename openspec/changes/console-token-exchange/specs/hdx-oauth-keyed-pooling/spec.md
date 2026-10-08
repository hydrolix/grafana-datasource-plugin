# hdx-oauth-keyed-pooling Specification (delta)

## ADDED Requirements

### Requirement: `MutateQueryData` keys the pool on the subject in exchanging mode

When `pluginSettings.CredentialsType == "forwardOAuthExchange"` and a forwarded
token is present, `Driver.MutateQueryData` SHALL write that token's subject
claim into every query's `connectionArgs.sub`, and SHALL NOT write
`connectionArgs.oauthToken`.

The pool key is the subject rather than the token because the credential is
refreshed independently of the connection: keying on the token would discard
every pooled connection each time Grafana refreshed a user's sign-in token,
while the subject is stable for as long as the person is.

`forwardOAuth` is unchanged and continues to key on `oauthToken`.

#### Scenario: Exchanging mode keys on the subject

- **GIVEN** plugin settings with `CredentialsType = "forwardOAuthExchange"`
- **AND** a forwarded token whose subject is `alice`
- **WHEN** `MutateQueryData` runs against a request with one query
- **THEN** the resulting `connectionArgs` SHALL contain `sub: "alice"`
- **AND** it SHALL NOT contain `oauthToken`

#### Scenario: A refreshed token reuses the pooled connection

- **GIVEN** a pooled connection for subject `alice`
- **WHEN** `alice` arrives with a refreshed forwarded token
- **THEN** the connection args SHALL be unchanged
- **AND** the pooled connection SHALL be reused
