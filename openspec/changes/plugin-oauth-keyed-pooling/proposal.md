## Why

The fork's `Connector` at `0f83082` keys per-user connections by OAuth token: `Connect` reads `headers.Get(backend.OAuthIdentityTokenHeaderName)`, computes `keyWithConnectionArgs(uid, {"oauthToken": token})`, and stores the resulting `*sql.DB` under that key in its TTL cache. For deployments where `CredentialsType == "forwardOAuth"`, the fork also *skips* the bootstrap `Connect(ctx, settings, nil)` call entirely, because there is no token at construction time.

After C2 pins to sqlds at `ef925e1`, `Connector` is upstream-generic — no OAuth awareness. Two adjustments are needed in the plugin to restore per-user keying:

1. **Per-query OAuth injection.** The plugin's `Driver.MutateQueryData` (already exists for `querySettings` merging) extends to also read the OAuth token from the request headers and write `connectionArgs = {"oauthToken": "<token>"}` into each `q.JSON`. sqlds's `Connector.GetConnectionFromQuery` then naturally computes `keyWithConnectionArgs(uid, q.ConnectionArgs)` and routes per-user, because the token differs per HTTP request.
2. **Lazy bootstrap.** `sqlds.NewConnector` at `ef925e1` always calls `driver.Connect(ctx, settings, nil)` once during construction. The plugin's current `Connect` returns `"cannot get auth header"` when `settings.CredentialsType == "forwardOAuth"` and `args` is nil — which breaks OAuth-only datasource instantiation. `Connect(_, _, nil)` must return a usable `*sql.DB` (Go's `sql.Open` is lazy — no network until first ping), so construction succeeds even when no token is available yet.

`EnableMultipleConnections = true` (set in C2's wrapper) and `Driver.Settings().ForwardHeaders = false` (set in C2's substrate) are the upstream-side prerequisites. With those plus the two adjustments above, the cache keying flow is: HTTP request → `MutateQueryData` injects `connectionArgs` → sqlds derives `<uid>-<sha256(connectionArgs)>` → C3's `TTLConnectionCache` stores per-user `*sql.DB` with one-hour TTL. Anonymous / service-account deployments emit no `connectionArgs` and stay on the bootstrap `<uid>-default` entry.

## What Changes

- Extend `Driver.MutateQueryData` in `pkg/plugin/driver.go` to inject `connectionArgs = {"oauthToken": "<token>"}` into each `q.JSON` when `pluginSettings.CredentialsType == "forwardOAuth"` and a non-empty OAuth token is present in `req.GetHTTPHeaders().Get(backend.OAuthIdentityTokenHeaderName)`. The existing `querySettings` merge stays — both writes happen in the same pass.
- Add `pkg/plugin/connection_args.go` with `injectConnectionArgs(jsonBody json.RawMessage, args map[string]string) (json.RawMessage, error)` — round-trips the JSON through `map[string]json.RawMessage`, sets `connectionArgs` to the marshalled `args`, remarshal. Preserves byte-for-byte identity of other fields modulo the new key.
- Update `Driver.Connect(ctx, settings, args)` in `pkg/plugin/driver.go`: when `settings.CredentialsType == "forwardOAuth"` and `args == nil` (the bootstrap call from `sqlds.NewConnector`), return a lazy `*sql.DB` (`sql.Open` with no usable credentials — succeeds because `sql.Open` does not connect). The error path activates only when `args != nil` and the token is missing or malformed (the actual per-query path).
- Confirm `EnableMultipleConnections = true` from C2's wrapper and `ForwardHeaders = false` from C2's substrate are unchanged. No new code in `hdx_sqlds.go`.
- Go unit-test coverage:
  - `MutateQueryData`: empty/absent OAuth token is a no-op (no `connectionArgs` written, `querySettings` merge still happens); non-empty token writes `connectionArgs.oauthToken` into every query's JSON; existing JSON fields are preserved byte-for-byte; `CredentialsType != "forwardOAuth"` is a no-op for the OAuth path regardless of token presence; the original `req.Queries` pointer/slice is not mutated cross-request.
  - `injectConnectionArgs`: round-trip preservation; existing `connectionArgs` is overwritten, not merged; malformed JSON propagates as a typed error.
  - `Driver.Connect`: `args == nil` with `forwardOAuth` returns a usable (lazy) `*sql.DB` with no auth; `args == nil` with non-OAuth credentials returns a `*sql.DB` configured with the static credentials; `args != nil` with a valid token configures the OAuth header; `args != nil` with a missing token returns a `backend.DownstreamError`.
- Playwright e2e coverage unchanged.

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics. Existing OAuth-only deployments continue to get per-user `*sql.DB`s; existing non-OAuth deployments continue to use the bootstrap connection.

## Capabilities

### New Capabilities

- `hdx-oauth-keyed-pooling`: Plugin-owned per-request OAuth-token injection into `connectionArgs` via `Driver.MutateQueryData`, combined with lazy-bootstrap `Driver.Connect(_, _, nil)`. Together they let sqlds's upstream `Connector` key per-user without any sqlds-side OAuth awareness.

### Modified Capabilities

<!-- C2's hdx-sqlds-wrapper is unchanged. EnableMultipleConnections and ForwardHeaders were set there as substrate-level invariants; this change exercises but does not modify them. -->

## Impact

- **Frontend**: none.
- **Backend (Go)**: extend `MutateQueryData` in `pkg/plugin/driver.go`; new `pkg/plugin/connection_args.go` + `pkg/plugin/connection_args_test.go`; small change to `Driver.Connect`'s OAuth-token branch.
- **Tests**: new unit tests for `MutateQueryData` OAuth-injection, `injectConnectionArgs` round-trip, and `Connect`'s lazy-bootstrap path. Existing `MutateQueryData` tests adjust to assert the additional `connectionArgs` write does not perturb `querySettings`.
- **Dependencies**: none added or removed.
- **User-visible**: none. Per-user pooling continues; deployments that aren't OAuth keep using the bootstrap connection.
- **Security**: OAuth token handling moves from inside the fork's `Connector` into the plugin's `Driver` — same surface, plugin-owned and reviewable in this repo. Token never logged.
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2) for the wrapper and substrate settings, and on `plugin-ttl-connection-cache` (C3) so per-user entries have a TTL'd cache to land in. Ships in the C2-C7 coordinated merge window.
