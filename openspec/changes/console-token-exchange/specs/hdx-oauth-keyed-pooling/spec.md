# hdx-oauth-keyed-pooling Specification (delta)

## ADDED Requirements

### Requirement: `MutateQueryData` keys the pool on the subject in exchanging mode

When `pluginSettings.CredentialsType == "forwardOAuthExchange"` and a forwarded
token is present, `Driver.MutateQueryData` SHALL write that token's issuer and
subject claims, joined, into every query's `connectionArgs.sub`, and SHALL NOT
write `connectionArgs.oauthToken`. A `sub` is unique per issuer only, and
Grafana can run several OAuth providers at once.

The pool key is the subject rather than the token because the credential is
refreshed independently of the connection: keying on the token would discard
every pooled connection each time Grafana refreshed a user's sign-in token,
while the subject is stable for as long as the person is.

`forwardOAuth` is unchanged and continues to key on `oauthToken`.

### Requirement: A query's own `connectionArgs` never reach the pool

In every credentials mode, `Driver.MutateQueryData` SHALL replace each query's
`connectionArgs` with the ones it built, and SHALL remove them when it built
none. sqlds keys the connection on the query JSON, which the browser supplies,
so a client-supplied value would otherwise choose whose connection a query
runs on.

#### Scenario: A caller with no forwarded token names another subject

- **GIVEN** plugin settings with `CredentialsType = "forwardOAuthExchange"`
- **AND** a query whose JSON carries `connectionArgs: {"sub": "<victim>"}`
- **AND** no forwarded token
- **WHEN** `MutateQueryData` runs
- **THEN** the resulting query SHALL carry no `connectionArgs`

#### Scenario: Exchanging mode keys on the subject

- **GIVEN** plugin settings with `CredentialsType = "forwardOAuthExchange"`
- **AND** a forwarded token whose issuer is `https://kc` and subject is `alice`
- **WHEN** `MutateQueryData` runs against a request with one query
- **THEN** the resulting `connectionArgs` SHALL contain `sub: "https://kc|alice"`
- **AND** it SHALL NOT contain `oauthToken`

#### Scenario: A refreshed token reuses the pooled connection

- **GIVEN** a pooled connection for subject `alice`
- **WHEN** `alice` arrives with a refreshed forwarded token
- **THEN** the connection args SHALL be unchanged
- **AND** the pooled connection SHALL be reused
