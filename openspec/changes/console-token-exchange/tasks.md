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
- [x] 2.2 Install it through `Options.TransportFunc` for the HTTP protocol when
      the mode is the exchanging one. `MutateQueryData` puts the subject in
      `connectionArgs` instead of the token, so the pool key survives a refresh.
- [x] 2.2a A test that the forwarded console token can never reach the cluster,
      including when the exchange fails.
- [x] 2.3 On a cluster `401`: invalidate that user and retry the request once.
- [x] 2.4 Native protocol: the credential is the connection's password and cannot
      be swapped per request. Either refuse the mode for native with a clear
      message, or key the pool on the exchanged token for native only. **Decided:
      refused**, with a message saying the mode needs the http protocol. Native
      binds its credential to the connection as a password, so a refreshed token
      could never reach an open connection; a second, worse code path is not
      worth shipping for a protocol the console's own datasources never use
      (they are http, port 443, path /query).

**The transport seam is built** — `exchange.Transport` plus the context-carried
`Principal`, 29 tests in the package. The mechanism that makes it work: the
driver builds its outbound request with `http.NewRequestWithContext`
(`conn_http.go`), so the query's context reaches the round tripper, and the
principal can ride it. That is why the subject token needs no side map and no
place in `connectionArgs`.

Section 2 is built, including the wiring in `driver.go`. Six tests cover the
wiring itself (`driver_exchange_test.go`), and every existing suite in the repo
still passes. The diff in `models/settings.go` is one field and its comment; the
rest of that file's diff is gofmt realigning the struct tags around a longer
field name.

## 3. Settings and health

- [x] 3.1 `CredentialsType` gains the exchanging mode; `src/types.ts` and the
      config editor follow. `oauthPassThru` is set for **both** forwarding modes
      — Grafana forwards a token only when asked to, and the exchanging mode
      then swaps it before querying. The mode asks for no credential at all:
      the delegate lives in server configuration. Its one optional field is the
      cluster audience, in `jsonData`, where nothing secret belongs; empty means
      the host, which is the audience on every cluster the console registers
      today. Seven tests in `ConfigEditor.exchange.test.tsx`; the repo's 332
      frontend tests still pass.
- [x] 3.2 Read the endpoint and the delegate credential from server config,
      never from `jsonData`. **Built as a map, not four scalars.** Four names
      can express one credential; a Grafana serving several clusters needs one
      per cluster, so `GF_PLUGIN_EXCHANGE_CREDENTIALS` is a JSON object keyed by
      audience and `GF_PLUGIN_EXCHANGE_URL` is the endpoint
      (`HDX_`-prefixed names accepted too, for deployments that set the
      environment on the process rather than through Grafana's config). A
      half-configured instance is a named error, not a silent fallback.
- [x] 3.3 **Done, and it also fixes plain `forwardOAuth`.** Upstream runs the
      check on the bootstrap connection, which has no user, so a working
      forward-mode datasource reported degraded health — and a green tick would
      have read as "your access works" when nothing about anyone's access was
      tested. Both forwarding modes now answer with what can actually be
      established: for the exchanging mode, whether this Grafana holds a
      delegate credential **for this cluster** (they are per cluster, so the
      message names which), and in every case that a person's access is proven
      by opening a panel, not here. Four tests.

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
      - [x] 4.1a **Done.** Both caches key on `(forwarded identity, …)` through
            `cacheScope`/`scopedKey` in `metadata.go`. The scope is the
            forwarded token's subject — stable across a refresh, distinct
            between people. A mode that forwards no identity yields an empty
            scope and the bare key the cache has always used, because one
            credential is one legitimate view. A forwarded token whose subject
            cannot be read falls back to a digest of the token: wasteful across
            refreshes, and the right way to be wrong, since the alternative
            shares one entry between different people. The cache log lines now
            name the table or CTE rather than the key, which carries a subject.
      - [x] 4.1b Test: two users, one table, second user must reach the cluster.
- [x] 4.2 **Tested live, and they do not lose the token.** This was the gating
      question for package dashboards, not a detail: across the bundle corpus
      `$__adHocFilter` appears 228 times and `$__timeFilter` 183, and each runs a
      schema lookup that is itself a query needing the signed-in user. A
      dashboard of `$__timeFilter()` (primary-key lookup), `$__adHocFilter()`
      (DESCRIBE) and `$__conditionalAll` against a 484M-row table returned
      numbers on every panel, with no refusal and no extra exchange — the
      lookups rode the cached token, and repeats were served by the metadata
      cache.
      - What this did **not** cover: ad-hoc filter *key population* in the
        editor, which is a frontend resource call rather than this backend path.
        Worth a look before anyone calls the mode finished.
- [ ] 4.3 (not blocking) `hdx-query-attribution` already exists as a spec in this repo; the
      exchanged token's subject is the obvious thing to attribute a query to, and
      the cluster records nothing per user today. Worth connecting the two.

## 5. Verification

**Proven live, 2026-10-02, against console-playpen with no shim in the path.**
A signed-in user's dashboard returned rows: Django logged one
`token_exchange_minted … delegate=grafana-playpen … expires_in=300` and the
plugin logged six `status=ok` queries off that single mint — the
one-exchange-per-dashboard property, measured on the real thing.

Two defects the live run found, which no unit test would have:

1. **`Connect` pinged the cluster.** `db.PingContext` at connect time is the
   "server hello", and it ran for every mode except `forwardOAuth` — a literal
   string comparison that a mode added later could not match. A connection whose
   credential belongs to the signed-in user cannot be verified there: Connect has
   no user in hand. Every query failed before it began. Now both forwarding modes
   skip it, behind a named predicate rather than a string.
2. **The query context does not reach the driver's handshake.** It is issued when
   `database/sql` establishes the connection, with a context that is not the
   query's, so a principal carried only on the context left the handshake with
   nobody to be. The connection now also knows the subject that keys it, and a
   registry supplies that person's newest forwarded token.

One thing that looked like a defect and was not: Django answered 400
`DisallowedHost` because the plugin calls `host.docker.internal` in this rig. A
deployed Grafana calls the console's real hostname, which is already allowed.


- [x] 5.1 Unit tests above, green under `mage test` / `go test ./...`.
- [x] 5.2 Against a real cluster: a dashboard renders as the signed-in user; one
      dashboard load causes one exchange per user; a 401 re-mints once; stopping
      the console leaves panels working until the cached token expires and then
      says so.
- [x] 5.3 The control: the same user's **unexchanged** token sent to the cluster
      is refused. If it is ever accepted, the audience isolation is broken and
      this change is not what makes queries work. **Run 2026-10-02 against
      playpen: refused.** So the exchange is what makes the query work, rather
      than the cluster having accepted the console token all along.

## 5b. Found by running the rig again (2026-10-05)

- [x] 5b.1 **A reply carrying no OAuth error was reported as a refusal of the
      person.** Django answered the rig's exchange URL with a 400
      `DisallowedHost`, and the panel said the console had refused this account
      access to the cluster. The facade answers every failure with a code from
      its own vocabulary, so a body without one did not come from the facade:
      a wrong exchange URL, something in front of it, or a host the console
      rejects. Those are the operator's problem, and calling one a refusal
      sends somebody to look at permissions when the cause is the URL.
      `classify`'s default is now `ErrMisconfigured`, whose sentinel widened
      from "no delegate credential" to cover the whole family. Four cases
      tested, plus the counterpart that a real OAuth error still reads as a
      refusal.
      - Worth noting *why* only a live run finds this: every unit test fed the
        classifier a well-formed facade error, because that is what the facade
        returns. Nothing was wrong with the tests; the case they missed was one
        the facade cannot produce.

## 6. Landing

- [x] 6.1 Spec deltas under `specs/`: the new `console-token-exchange`
      capability, and the `hdx-oauth-keyed-pooling` modification — the
      exchanging mode keys the pool on the subject, while `forwardOAuth` keys
      on the token as before.
- [x] 6.2 `README` documents the mode and the two environment variables. Nobody
      can deploy this by reading the source.
- [x] 6.3 `CHANGELOG` entry for the feature and for the health-check fix, which
      lands for plain `forwardOAuth` users too.
- [x] 6.4 e2e: `tests/configEditorCredentials.spec.ts` — each credentials mode
      asks for its own fields, and the exchanging mode asks for no credential
      at all, which is the property that would be broken silently.
- [x] 6.5 **Run: 61 passed, 0 failed** against Grafana 13.2.1, the new spec
      included. One trap worth recording for the next person: run it as CI
      does, `docker compose run --rm playwright`, not on the host. Two tests
      reach ClickHouse directly at its container hostname, and
      `CLICKHOUSE_HOSTNAME` is read both for that and for the datasource's own
      host — which the plugin resolves from inside Grafana — so no host-side
      value satisfies both. Run on the host, those two fail for reasons that
      have nothing to do with the change.
- [x] 6.6a The `Connect`, pooling and metadata-cache changes touch every mode,
      so the question was whether the ordinary ones still work. The full e2e
      suite drives them end to end — config save and test, queries, macros,
      ad-hoc filters, annotations, template variables — and passes.
- [ ] 6.6b Still owed, and narrower than it was: ad-hoc filter **key
      population** in `forwardOAuthExchange` specifically. The suite proves
      that path works on a datasource holding its own credential. Whether the
      editor's resource call carries the signed-in user the way the query path
      does is a different question, and 4.2's live run did not cover it. Needs
      the playpen rig.
