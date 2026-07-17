# hdx-oauth-keyed-pooling

## ADDED Requirements

### Requirement: `MutateQueryData` injects `oauthToken` into `connectionArgs` when in `forwardOAuth` mode

When `pluginSettings.CredentialsType == "forwardOAuth"` and `req.GetHTTPHeaders().Get(backend.OAuthIdentityTokenHeaderName)` returns a non-empty value, `Driver.MutateQueryData` SHALL write the value (with any `"Bearer "` prefix stripped) into every query's `connectionArgs.oauthToken` field. The write SHALL preserve every other JSON field byte-for-byte modulo the new key.

#### Scenario: `forwardOAuth` with `Authorization: Bearer abc...`

- **GIVEN** plugin settings with `CredentialsType = "forwardOAuth"`
- **AND** an HTTP header `Authorization: Bearer abc123`
- **WHEN** `MutateQueryData` runs against a request with one query
- **THEN** the resulting `req.Queries[0].JSON` SHALL parse to an object containing `connectionArgs: {"oauthToken": "abc123"}`
- **AND** the `querySettings` field SHALL be present (existing merge behaviour preserved)

#### Scenario: `forwardOAuth` with empty `Authorization` header

- **GIVEN** plugin settings with `CredentialsType = "forwardOAuth"`
- **AND** no `Authorization` header on the request
- **WHEN** `MutateQueryData` runs
- **THEN** `connectionArgs.oauthToken` SHALL NOT be written
- **AND** if no other connection-args key is set, `connectionArgs` SHALL NOT appear in the resulting JSON

### Requirement: `MutateQueryData` injects `orgId` into `connectionArgs` independent of credentials type

When `req.GetHTTPHeaders().Get("X-Grafana-Org-Id")` returns a non-empty value, `Driver.MutateQueryData` SHALL write that value into every query's `connectionArgs.orgId` field, regardless of `pluginSettings.CredentialsType`. Multi-tenant deployments care about org separation on every auth mode.

#### Scenario: `userAccount` mode with Org-Id header

- **GIVEN** plugin settings with `CredentialsType = "userAccount"`
- **AND** an HTTP header `X-Grafana-Org-Id: 5`
- **WHEN** `MutateQueryData` runs
- **THEN** `req.Queries[0].JSON.connectionArgs.orgId` SHALL be `"5"`
- **AND** `connectionArgs.oauthToken` SHALL NOT be set

#### Scenario: Both Authorization and Org-Id headers in `forwardOAuth` mode

- **GIVEN** `forwardOAuth` mode, `Authorization: Bearer t`, `X-Grafana-Org-Id: 5`
- **WHEN** `MutateQueryData` runs
- **THEN** `connectionArgs` SHALL contain exactly `{"oauthToken": "t", "orgId": "5"}`

### Requirement: `MutateQueryData` skips `connectionArgs` write when no per-request keying signal is present

When neither the OAuth token (under `forwardOAuth`) nor the Org-Id header is present, `MutateQueryData` SHALL leave `connectionArgs` unwritten. An empty `connectionArgs` object would still SHA-256 to a non-default cache key and fragment the bootstrap entry.

#### Scenario: No headers, non-OAuth mode

- **GIVEN** `CredentialsType = "userAccount"` and no Authorization or Org-Id headers
- **WHEN** `MutateQueryData` runs
- **THEN** `req.Queries[0].JSON` SHALL NOT contain a `connectionArgs` field after the call (if it was absent before, it stays absent)

### Requirement: `injectConnectionArgs` overwrites, does not merge

The plugin SHALL define `injectConnectionArgs(body json.RawMessage, args map[string]string) (json.RawMessage, error)` in `pkg/plugin/connection_args.go`. The function SHALL replace any existing `connectionArgs` value with the marshalled `args` map (no key-level merge) and SHALL preserve every other field of `body`.

#### Scenario: No prior connectionArgs

- **GIVEN** `body = {"rawSql": "SELECT 1"}` and `args = {"oauthToken": "t"}`
- **WHEN** `injectConnectionArgs(body, args)` is called
- **THEN** the result SHALL parse to `{"rawSql": "SELECT 1", "connectionArgs": {"oauthToken": "t"}}`

#### Scenario: Prior connectionArgs is overwritten

- **GIVEN** `body = {"rawSql": "X", "connectionArgs": {"stale": "v"}}` and `args = {"oauthToken": "t"}`
- **WHEN** `injectConnectionArgs(body, args)` is called
- **THEN** the result's `connectionArgs` SHALL equal `{"oauthToken": "t"}` exactly (no `"stale"` key)

#### Scenario: Malformed body propagates the unmarshal error

- **GIVEN** `body = []byte("not json")`
- **WHEN** `injectConnectionArgs(body, anything)` is called
- **THEN** the returned error SHALL be non-nil

### Requirement: `getOAuthToken` / `getOrgId` read the flat `connectionArgs` shape

The plugin SHALL rewrite `getOAuthToken` and `getOrgId` in `pkg/plugin/driver.go` to read `connectionArgs` decoded as `map[string]string`, looking up `"oauthToken"` / `"orgId"` respectively. Empty values and missing keys SHALL produce `("", false)`. The legacy `sqlds.HeaderKey`-nested reader (`getHeader`) SHALL be removed; nothing else in `pkg/` depends on it.

#### Scenario: Token present at the expected key

- **GIVEN** `args = []byte('{"oauthToken": "abc"}')`
- **WHEN** `getOAuthToken(args)` is called
- **THEN** the return SHALL be `("abc", true)`

#### Scenario: Token absent or empty

- **GIVEN** `args = []byte('{"orgId": "5"}')` (no oauthToken)
- **WHEN** `getOAuthToken(args)` is called
- **THEN** the return SHALL be `("", false)`

#### Scenario: Nil or malformed args

- **GIVEN** `args = nil` (or non-JSON bytes)
- **WHEN** `getOAuthToken(args)` is called
- **THEN** the return SHALL be `("", false)`

### Requirement: `Driver.Connect(_, settings, nil)` returns a lazy `*sql.DB` in `forwardOAuth` mode

When invoked with `args == nil` (the bootstrap call from `sqlds.NewConnector`), `Driver.Connect` SHALL return a usable `*sql.DB` even when `settings.CredentialsType == "forwardOAuth"`. `sql.OpenDB` does not contact the upstream server; per-user `*sql.DB` instances arrive on first per-query call once `MutateQueryData` has populated `connectionArgs.oauthToken`.

#### Scenario: Bootstrap call under forwardOAuth succeeds

- **GIVEN** `settings.CredentialsType == "forwardOAuth"` and `args == nil`
- **WHEN** `Connect` is called
- **THEN** the return SHALL be `(*sql.DB, nil)` with a non-nil DB
- **AND** the existing skip-ping behaviour under `forwardOAuth` SHALL keep the call from contacting the server

#### Scenario: Per-query call with `args != nil` and missing token surfaces an error

- **GIVEN** `settings.CredentialsType == "forwardOAuth"` and `args = []byte('{"orgId": "5"}')` (no oauthToken)
- **WHEN** `Connect` is called
- **THEN** the return SHALL include a non-nil error
- **AND** the `*sql.DB` SHALL be nil

### Requirement: `MutateQueryData` does not leak per-request state between `*backend.QueryDataRequest`s

Each invocation of `MutateQueryData` SHALL derive `connectionArgs` solely from the headers attached to the inbound `*backend.QueryDataRequest`. No state SHALL persist on the `Hydrolix` driver between calls.

#### Scenario: Two requests with different OAuth tokens

- **GIVEN** request A with `Authorization: Bearer A` and request B with `Authorization: Bearer B`, constructed independently
- **WHEN** `MutateQueryData(reqA)` runs, then `MutateQueryData(reqB)` runs
- **THEN** `reqA.Queries[0].JSON.connectionArgs.oauthToken` SHALL be `"A"`
- **AND** `reqB.Queries[0].JSON.connectionArgs.oauthToken` SHALL be `"B"`
