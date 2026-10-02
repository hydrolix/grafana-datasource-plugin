# Tasks

## 1. The token source (`pkg/plugin/exchange`)

- [ ] 1.1 `TokenSource` with `Token(ctx, user string) (string, error)`, a per-user
      cache of `{token, deadline}`, and a `singleflight.Group` keyed on the user
      (`golang.org/x/sync` is already an indirect dependency).
- [ ] 1.2 Deadlines stamped from the request start, never from a server clock.
      Refresh lead = 20 % of lifetime clamped to [60 s, 300 s]; lazy skew 30 s;
      no scheduling under a 120 s lifetime.
- [ ] 1.3 Retry ladder 5/10/20/40/60 s, each rung taken only if it lands before
      the current token's real expiry.
- [ ] 1.4 `Invalidate(user)` with a generation counter, so an exchange that began
      before the invalidation cannot populate the cache after it.
- [ ] 1.5 The exchange call itself: form-encoded RFC 8693
      (`grant_type=urn:ietf:params:oauth:grant-type:token-exchange`,
      `subject_token`, `subject_token_type=…:access_token`, `audience`), Basic
      auth with the delegate credential, no `scope` (the console's facade refuses
      one). Classify the response into refused / unavailable / misconfigured.
- [ ] 1.6 Tests: single-flight under concurrency, refresh-ahead boundaries,
      ladder bounded by expiry, invalidate-mid-exchange, and a log-hygiene test
      asserting no token, SQL or secret reaches any log line.

## 2. Per-request credential (`pkg/plugin/driver.go`)

- [ ] 2.1 A `RoundTripper` that sets `Authorization: Bearer <exchanged token>` on
      each request, from the token source, for the user the connection belongs to.
- [ ] 2.2 Install it through `Options.TransportFunc` for the HTTP protocol when
      the mode is the exchanging one. The pool key stays the forwarded token.
- [ ] 2.3 On a cluster `401`: invalidate that user and retry the request once.
- [ ] 2.4 Native protocol: the credential is the connection's password and cannot
      be swapped per request. Either refuse the mode for native with a clear
      message, or key the pool on the exchanged token for native only. Decide and
      write it down; do not leave it implicit.

## 3. Settings and health

- [ ] 3.1 `CredentialsType` gains the exchanging mode; `src/types.ts` and the
      config editor follow. `oauthPassThru` stays true for it — Grafana must
      still forward the sign-in token.
- [ ] 3.2 Read `exchange_url`, `exchange_audience`, `exchange_client_id`,
      `exchange_client_secret` from server config. Never from `jsonData`.
- [ ] 3.3 Health check: distinguish "not configured" from "configured but the
      console refused" from "console unreachable". Note that Save & test carries
      no user token, so it can verify configuration reachability only — say so in
      the message rather than implying the user's access was checked.

## 4. The gaps this mode inherits, named not assumed

- [ ] 4.1 **Shared metadata caches.** Schema lookups cached per datasource rather
      than per user would cross users in any forwarding mode. Establish whether
      they do, and fix or document it — this decides whether forward modes are
      usable in a multi-tenant org at all.
- [ ] 4.2 Macros that run a schema lookup without a user context
      (`$__adHocFilter`, `$__timeFilter` with no column) lose the token. Confirm
      the behaviour and state it.
- [ ] 4.3 `hdx-query-attribution` already exists as a spec in this repo; the
      exchanged token's subject is the obvious thing to attribute a query to, and
      the cluster records nothing per user today. Worth connecting the two.

## 5. Verification

- [ ] 5.1 Unit tests above, green under `mage test` / `go test ./...`.
- [ ] 5.2 Against a real cluster: a dashboard renders as the signed-in user; one
      dashboard load causes one exchange per user; a 401 re-mints once; stopping
      the console leaves panels working until the cached token expires and then
      says so.
- [ ] 5.3 The control: the same user's **unexchanged** token sent to the cluster
      is refused. If it is ever accepted, the audience isolation is broken and
      this change is not what makes queries work.
