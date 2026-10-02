# Tasks

## 1. The token source (`pkg/plugin/exchange`)

- [x] 1.1 `TokenSource` with `Token(ctx, sub string) (string, error)`, a cache of
      `{token, deadline}` keyed on the subject claim — **never on the forwarded
      token**, which changes on every Grafana refresh — and a
      `singleflight.Group` keyed the same way (`golang.org/x/sync` is already an
      indirect dependency).
- [x] 1.2 Deadlines stamped from the request start, never from a server clock.
      Refresh lead = 20 % of lifetime clamped to [60 s, 300 s]; lazy skew 30 s;
      no scheduling under a 120 s lifetime.
- [x] 1.3 Retry ladder 5/10/20/40/60 s, each rung taken only if it lands before
      the current token's real expiry.
- [x] 1.4 `Invalidate(user)` with a generation counter, so an exchange that began
      before the invalidation cannot populate the cache after it.
- [x] 1.5 The exchange call itself: form-encoded RFC 8693
      (`grant_type=urn:ietf:params:oauth:grant-type:token-exchange`,
      `subject_token`, `subject_token_type=…:access_token`, `audience`), Basic
      auth with the delegate credential, no `scope` (the console's facade refuses
      one). Classify the response into refused / unavailable / misconfigured.
- [x] 1.6 Tests: single-flight under concurrency, refresh-ahead boundaries,
      ladder bounded by expiry, invalidate-mid-exchange, and a log-hygiene test
      asserting no token, SQL or secret reaches any log line.

**Section 1 is built** — `pkg/plugin/exchange` (config, client, source) with 20 tests,
green under `-race`, and the repo's existing suites unaffected. Two notes on what
was decided while building it:

- **The refresh is synchronous, not a background goroutine.** A caller inside the
  lead window performs the refresh itself while its current token is still
  usable, so a failed refresh is invisible — the token in hand keeps serving. A
  renewal timer would add process lifecycle for the same outcome; the browser
  needs one because a tab has no request to attach the work to, and a plugin
  does.
- **A refusal drops the cached token and is remembered for ten seconds.** The
  browser has no equivalent because a browser is one user. Server-side, a refused
  person reloading a thirty-panel dashboard would otherwise ask the console
  thirty times, and a converging role removal would become a load spike against
  one delegate's quota. Dropping the token is deliberate: a refusal is the
  removal path taking effect, and continuing to serve would defeat the gate chain
  that produced it.

## 2. Per-request credential (`pkg/plugin/driver.go`)

- [x] 2.1 A `RoundTripper` that sets `Authorization: Bearer <exchanged token>` on
      each request, from the token source, for the user the connection belongs to.
- [ ] 2.2 Install it through `Options.TransportFunc` for the HTTP protocol when
      the mode is the exchanging one. `MutateQueryData` puts the subject in
      `connectionArgs` instead of the token, so the pool key survives a refresh.
- [x] 2.2a A test that the forwarded console token can never reach the cluster,
      including when the exchange fails.
- [x] 2.3 On a cluster `401`: invalidate that user and retry the request once.
- [ ] 2.4 Native protocol: the credential is the connection's password and cannot
      be swapped per request. Either refuse the mode for native with a clear
      message, or key the pool on the exchanged token for native only. Decide and
      write it down; do not leave it implicit.

**The transport seam is built** — `exchange.Transport` plus the context-carried
`Principal`, 29 tests in the package. The mechanism that makes it work: the
driver builds its outbound request with `http.NewRequestWithContext`
(`conn_http.go`), so the query's context reaches the round tripper, and the
principal can ride it. That is why the subject token needs no side map and no
place in `connectionArgs`.

Still open in this section: **2.2** (installing it through `Options.TransportFunc`
in `driver.go` and putting the subject in `connectionArgs`) and **2.4** (the
native protocol, whose credential is the connection's password and cannot be
swapped per request). Both touch the plugin's own files rather than this new
package, so they are the first changes a reviewer there will care about.

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

- [x] 4.1 **Shared metadata caches — confirmed, and it affects shipped
      `forwardOAuth`, not just this mode.** `MetadataProvider` keys `pkCache` on
      `database + "_" + table` and `keyCache` on the CTE reference
      (`pkg/plugin/metadata.go`), both with a one-hour TTL, per datasource
      instance. The caller's `headers` — carrying the user's token — reach the
      cluster only on a **miss**. So one user's lookup populates the cache, and
      for the next hour every other user of that datasource is served that
      table's primary key and column name→type map without the cluster
      authorizing them. Disclosure is schema shape, not rows.
      - [ ] 4.1a Add the user (the forwarded token's subject) to both cache keys
            in forwarding modes, or bypass these caches there. Keying on the
            token itself would churn the cache on every refresh; the subject is
            stable.
      - [ ] 4.1b Test: two users, one table, second user must reach the cluster.
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
