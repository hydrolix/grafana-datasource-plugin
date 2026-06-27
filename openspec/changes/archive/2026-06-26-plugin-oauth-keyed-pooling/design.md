## Context

At `0f83082` the fork's `HydrolixConnector.Connect` (`connector.go:67-93`) does three things sqlds-internal Connectors don't:

1. Reads `headers.Get(backend.OAuthIdentityTokenHeaderName)` and derives a per-user `key` via `keyWithConnectionArgs(uid, getOAuthConnectionArgs(token))`.
2. Calls `driver.Connect(ctx, settings, getOAuthConnectionArgs(token))` with the OAuth-derived args, so the driver builds an authenticated `*sql.DB` per token.
3. At `NewConnector`, *skips* the bootstrap `driver.Connect(_, _, nil)` when `pluginSettings.CredentialsType == "forwardOAuth"`. The bootstrap-key entry is never populated for OAuth-only deployments.

sqlds at `ef925e1` does none of that. `Connector.Connect(ctx, headers)` always looks up `defaultKey(uid)`; `Connector.GetConnectionFromQuery(ctx, q)` derives the key from `q.ConnectionArgs`; `NewConnector` unconditionally calls `driver.Connect(ctx, settings, nil)` at construction. The plugin has to push OAuth awareness back upstream into the query JSON via the existing `QueryDataMutator` extension point — which the plugin already implements for `querySettings` merging (`driver.go:251`).

The plugin's existing `Driver.Connect` (`driver.go:68`) is OAuth-aware on the per-query path: when `settings.CredentialsType == "forwardOAuth"` and `args` carries a valid OAuth token, it builds the auth header. When `args == nil` (the bootstrap call), it errors with `"cannot get auth header"`. That error blocks OAuth-only datasource construction under sqlds at `ef925e1`.

## Goals / Non-Goals

**Goals:**
- Inject `connectionArgs = {"oauthToken": "<token>", "orgId": "<id>"}` into per-query JSON via `Driver.MutateQueryData` so sqlds keys the connection cache per user *and* per org.
- Rewrite `getOAuthToken` / `getOrgId` to read the flat connectionArgs shape (the legacy `sqlds.HeaderKey` nesting was populated by `ForwardHeaders=true`, which C2 turned off — those reads always miss now).
- Make `Driver.Connect(_, settings, nil)` return a usable lazy `*sql.DB` so `sqlds.NewConnector`'s bootstrap call succeeds in every deployment mode.
- Preserve `querySettings` merging — `MutateQueryData` already does it and continues to.

**Non-Goals:**
- Per-user health checks. `CheckHealth` continues to use the bootstrap entry. Different upstream hook required (`HealthKeyer` or similar); not in scope.
- Connection-args shape beyond `oauthToken` / `orgId`. Any additional dimension (e.g. role, ID-token) lands in its own future change.
- Validating the OAuth token content (JWT structure, expiry, signature). The plugin forwards what Grafana provides; auth validation is upstream Hydrolix's job.

## Decisions

### D1. Extend `MutateQueryData`; do not add a new mutator

`Driver.MutateQueryData` already runs once per `QueryDataRequest` and already iterates `req.Queries[i].JSON` to merge `querySettings`. Adding the `connectionArgs` injection inside the same loop is one extra branch and one extra `jsonSet` call per query.

```go
// pkg/plugin/driver.go (excerpt)
func (h *Hydrolix) MutateQueryData(ctx context.Context, req *backend.QueryDataRequest) (context.Context, *backend.QueryDataRequest) {
    pluginSettings, err := models.NewPluginSettings(ctx, *req.PluginContext.DataSourceInstanceSettings)
    if err != nil {
        log.DefaultLogger.Error("failed to parse plugin settings", "err", err)
        return ctx, req
    }
    if pluginSettings.QuerySettings == nil {
        pluginSettings.QuerySettings = []models.QuerySetting{}
    }

    headers := req.GetHTTPHeaders()
    connArgs := map[string]string{}
    if pluginSettings.CredentialsType == "forwardOAuth" {
        if token := strings.TrimPrefix(headers.Get(backend.OAuthIdentityTokenHeaderName), "Bearer "); token != "" {
            connArgs["oauthToken"] = token
        }
    }
    if org := headers.Get(OrgIdHeaderKey); org != "" {
        connArgs["orgId"] = org
    }

    for i, q := range req.Queries {
        // … existing querySettings merge (unchanged) → mergedSettingsArray …
        patches := map[string]any{"querySettings": mergedSettingsArray}
        if len(connArgs) > 0 {
            patches["connectionArgs"] = connArgs
        }
        if jmsg, err := jsonSet(q.JSON, patches); err == nil {
            req.Queries[i].JSON = jmsg
        } else {
            log.DefaultLogger.Error("failed to serialize query JSON", "err", err, "refId", q.RefID)
            continue
        }
    }
    return ctx, req
}
```

**Why a single mutator.** `MutateQueryData` runs at the right scope (per-request, before `handleQuery` iterates), already parses plugin settings once for the request, already iterates queries. Adding a separate mutator would parse settings twice and iterate queries twice. All three writes (querySettings, oauthToken, orgId) are semantically a single "rewrite this query JSON for sqlds consumption" pass.

**Why use `req.GetHTTPHeaders()` rather than the legacy `getOAuthToken(q.JSON)` reader.** The legacy reader pulled from `q.JSON.grafana-http-headers` — the nested map sqlds populated when `ForwardHeaders=true`. C2's substrate sets `ForwardHeaders=false`, so that JSON nesting is empty and the legacy reader always misses. The token + org id live on the request, reached via `req.GetHTTPHeaders()`. The reader rewrite (`getOAuthToken` / `getOrgId`) in D7 reads from the *flat* shape this function writes.

**Why scope `oauthToken` to `forwardOAuth` but not `orgId`.** OAuth tokens are only meaningful when the plugin is in OAuth mode — every other credentials type uses static, baked-in auth and per-request token keying would be meaningless. `X-Grafana-Org-Id`, by contrast, is set by Grafana on every authenticated request regardless of auth mode, and a multi-tenant Hydrolix cluster cares about it for *every* connection. Conditioning each key on its own predicate matches the wire contract: each key reflects a real per-request signal, never a fabricated one.

**Why only write `connectionArgs` when at least one key is set.** An empty `connectionArgs` would still serialise to `{}` and contribute to the cache-key SHA-256, gratuitously splitting the bootstrap entry. Skipping the write when the map is empty preserves the legacy `<uid>-default` routing for anonymous / non-OAuth / no-org requests.

### D2. `injectConnectionArgs` helper

Even though D1 inlines the patch via the existing `jsonSet` helper, the JSON-rewrite semantics deserve their own helper for testability:

```go
// pkg/plugin/connection_args.go
package plugin

import "encoding/json"

// injectConnectionArgs sets the connectionArgs field on a query JSON body
// to args (overwriting any existing value), preserving every other field.
// The returned body is a fresh allocation; the input is not modified.
func injectConnectionArgs(body json.RawMessage, args map[string]string) (json.RawMessage, error) {
    var obj map[string]json.RawMessage
    if err := json.Unmarshal(body, &obj); err != nil {
        return nil, err
    }
    if obj == nil {
        obj = make(map[string]json.RawMessage, 1)
    }
    argsJSON, err := json.Marshal(args)
    if err != nil {
        return nil, err
    }
    obj["connectionArgs"] = argsJSON
    return json.Marshal(obj)
}
```

**Why a helper rather than always going through `jsonSet`.** `jsonSet` is the plugin's generic shallow-merge utility. `injectConnectionArgs` is the specific OAuth-keying contract — its test surface is what assertions the migration's correctness depends on (round-trip preservation, overwrite-not-merge, byte-for-byte equality of unchanged fields). Pulling it out gives the test file a clean target.

**Why overwrite-not-merge.** If `q.JSON` already carried `connectionArgs` (e.g., a stale field from a prior request, or set by a frontend that pre-populates it), per-request keying must win. Merging would let a stale `oauthToken` or `orgId` override the live one. Overwrite is the only correct behaviour.

**Why `map[string]string` as the args type.** The fields today are `oauthToken` and `orgId` — both bare strings. A wider type (`map[string]any`) invites callers to stuff non-string values in, which downstream `keyWithConnectionArgs` then has to handle. Restricting to strings keeps the keying surface deterministic.

### D3. `Driver.Connect(_, settings, nil)` returns a lazy `*sql.DB` in all credentials modes

```go
// pkg/plugin/driver.go (excerpt of the forwardOAuth branch)
if settings.CredentialsType == "forwardOAuth" {
    if args == nil {
        // Bootstrap call from sqlds.NewConnector. No token available yet;
        // per-user *sql.DBs land in the cache lazily on first query.
        // sql.Open does not connect — returning here is cheap and safe.
        return sql.OpenDB(noAuthConnector{opts: optsWithoutAuth}), nil
    }
    oAuthToken, ok := getOAuthToken(args)
    if !ok {
        return nil, backend.DownstreamError(fmt.Errorf("forwardOAuth: missing OAuth token in connection args"))
    }
    token = oAuthToken
}
```

The bootstrap-call signal is `args == nil`. The plugin returns a `*sql.DB` whose driver `Connect` will fail if anyone ever pings it (because there's no token) — but nobody ever does, in OAuth-only deployments. `CheckHealth` will ping and fail; per-user queries land in their own cache entries.

**Why detect bootstrap via `args == nil`.** sqlds at `ef925e1` calls `driver.Connect(ctx, settings, nil)` for the bootstrap entry; per-query calls always pass a non-nil `args` (sqlds derives it from `q.ConnectionArgs`). Nil-vs-non-nil cleanly separates the two paths.

**Why "lazy `*sql.DB`" works.** Go's `database/sql.Open` (and `OpenDB`) returns immediately without contacting the upstream server. The connection handshake happens on first `Ping` / `Query`. The bootstrap entry sits idle in the cache; it's never used because every query carries OAuth args that route it to a per-user entry. For non-OAuth deployments, `CheckHealth` pings the bootstrap entry and it works because the static credentials are configured.

**Alternative considered: petition upstream for a `SkipInitialConnect` flag.** A flag on `DriverSettings` (`SkipInitialConnect bool`) would let `sqlds.NewConnector` skip the bootstrap call entirely for deployments that don't want a bootstrap entry. Cleaner conceptually, but requires upstream change and a release. Lazy-bootstrap is zero-cost, in-plugin, and matches the fork's net behaviour (bootstrap entry exists but is dead-weight for OAuth-only deployments). Defer the upstream flag — revisit if a real deployment surfaces a problem (e.g., a transport that errors at `sql.Open` time rather than at first `Ping`).

### D4. No `Connector`-level OAuth keying; everything flows through `ConnectionArgs`

sqlds's `Connector.GetConnectionFromQuery` derives the cache key as:

```go
// sqlds@ef925e1 connector.go
func (c *Connector) GetConnectionFromQuery(ctx context.Context, q *Query) (string, dbConnection, error) {
    key := defaultKey(c.UID)
    if c.enableMultipleConnections && len(q.ConnectionArgs) > 0 {
        key = keyWithConnectionArgs(c.UID, q.ConnectionArgs)
    }
    …
}
```

`keyWithConnectionArgs` hashes the SHA-256 of `q.ConnectionArgs` and produces `<uid>-<hex>`. The plugin's `MutateQueryData` writing `connectionArgs={"oauthToken":...}` into `q.JSON` is sufficient — sqlds parses `q.JSON.connectionArgs` into `q.ConnectionArgs` in `handleQuery`, and the derived key naturally separates per-user entries.

**Why nothing else is needed.** The plugin's TTL cache (C3) keys by whatever string `Store` receives. sqlds-internal logic derives the strings. Per-user keys arrive at `Store` and land in the TTL bucket; bootstrap key arrives at `Store` and lands with `NoTTL`.

**Why not also write `connectionArgs` for non-OAuth deployments.** Those deployments are happy with the bootstrap entry; per-request keying serves no purpose. Skipping `connectionArgs` keeps `q.ConnectionArgs == nil`, which sqlds resolves to `defaultKey(uid)` — exactly the bootstrap entry. Behaviour-equivalent to the fork.

### D5. The injected token is the raw bearer value, not "Bearer <value>"

The fork's `getOAuthConnectionArgs(headerValue string)` writes the *full* header value (`"Bearer <token>"`) into `connectionArgs.oauthToken` (see `connector.go:80` at `0f83082`). The plugin's existing `getOAuthToken(args)` then strips the `Bearer ` prefix when reading. This change normalises by stripping the prefix at write time:

```go
token := strings.TrimPrefix(req.GetHTTPHeaders().Get(backend.OAuthIdentityTokenHeaderName), "Bearer ")
```

**Why strip at write time.** Storing the raw bearer value in `connectionArgs.oauthToken` is what every reader expects (it's a "token", not an "authorization header"). The fork's approach of carrying the `Bearer ` prefix in storage and stripping at read time is a quirk that adds nothing.

**Cache-key impact.** The SHA-256 hash differs between the "Bearer "-prefixed and stripped forms. This change resets per-user cache entries on first deploy — a one-time cache miss for users with warm fork-era connections. After the deploy, the cache is normalised. Mention in the PR notes: "First request from each user after deploy will open a fresh connection. Existing connections drain via TTL within an hour."

**Alternative considered**: keep `Bearer <token>` in storage for byte-for-byte parity with the fork's cache keys. Rejected — the prefix carries no information once the value is in `connectionArgs.oauthToken` (it's only a header convention), and stripping at write makes the storage clearer.

### D6. `MutateQueryData` returns a fresh `req`? No — mutates in place

The function reassigns `req.Queries[i].JSON` per query, not the `req` pointer or `req.Queries` slice. Other handlers downstream see the mutated queries.

**Why mutate in place.** sqlds's `handleQuery` takes the queries from the same `req` after `MutateQueryData` returns. A fresh `req` would force a copy of every field for no benefit; the in-place mutation is what `QueryDataMutator` is designed for.

**Cross-request safety.** Each `QueryDataRequest` is constructed afresh by the Grafana SDK per HTTP request. There is no shared state between requests through `req`. The plugin's mutation cannot leak across requests.

Verify with a unit test: construct two `req`s sharing no fields, run `MutateQueryData` on the first with one token and on the second with a different token, assert the second `req.Queries[0].JSON.connectionArgs.oauthToken` matches the second token (not the first). Catches accidental cross-request state if any is introduced.

### D7. Rewrite `getOAuthToken` / `getOrgId` to read the flat shape; drop `getHeader`

The pre-C4 readers look at `q.JSON.grafana-http-headers[HeaderKey][headerName][0]` — the nested map sqlds populated when `ForwardHeaders=true`. With C2's substrate setting `ForwardHeaders=false`, that nesting is always empty: the readers return `("", false)` for every query and the `forwardOAuth` branch in `Driver.Connect` always errors with `"cannot get auth header"`. The readers and the `MutateQueryData` writes have to converge on the same shape.

```go
// pkg/plugin/driver.go (after C4)
func getOAuthToken(args json.RawMessage) (string, bool) {
    return readConnArg(args, "oauthToken")
}

func getOrgId(args json.RawMessage) (string, bool) {
    return readConnArg(args, "orgId")
}

func readConnArg(args json.RawMessage, key string) (string, bool) {
    if args == nil {
        return "", false
    }
    var m map[string]string
    if err := json.Unmarshal(args, &m); err != nil {
        return "", false
    }
    v, ok := m[key]
    if !ok || v == "" {
        return "", false
    }
    return v, true
}
```

The `getHeader` helper (and its dependence on `sqlds.HeaderKey`) is deleted in the same change — no other reader depends on it (grep across `pkg/` shows only the deleted callers).

**Why a single tiny helper.** The two readers are byte-identical modulo the key name. A single `readConnArg` is two lines shorter than duplicating and gives the test suite one focal point for the input-shape behaviour (nil, malformed JSON, missing key, empty value).

**Why no Bearer-prefix handling on the read side.** D5 strips the prefix at write time. After C4 the on-wire value in `connectionArgs.oauthToken` is always the bare token. Reading-side prefix logic would only become noise; if a future writer accidentally re-introduces the prefix, the downstream HTTP-header build (`opts.HttpHeaders["Authorization"] = "Bearer " + token`) would produce `"Bearer Bearer abc..."`, which Hydrolix would reject — loud, not silent.

**Why `args` arg shape is `json.RawMessage`, not `map[string]string`.** sqlds passes `q.ConnectionArgs json.RawMessage` directly to `Driver.Connect`. Decoding inside the reader keeps the type at the boundary; the driver doesn't have to know whether the field was set by `MutateQueryData`, a test, or a future hand-crafted caller.

## Risks / Trade-offs

- **[Token leaks via logs when `jsonSet` fails]** → Mitigation: the existing error-logging path logs `err` and `refId`, never the JSON body. Code review confirms no `log.*` call in the OAuth path includes `token` or `q.JSON`. Unit test that a synthetic JSON-marshal failure does not include the token in the logged error.
- **[Cache thrashing if a downstream proxy rotates OAuth tokens per request]** → Acceptable: per-request rotation is by definition per-user-and-per-request, so the cache would never reuse entries. The TTL cache caps the growth at one entry per token per hour. If a deployment surfaces this, the operator's choice is to lengthen the token lifetime upstream, not change this plugin.
- **[`sql.OpenDB` with a no-auth `driver.Connector` fails at construction in some `clickhouse-go` versions]** → Mitigation: the noAuthConnector returns a `*clickhouse.Conn` configured with empty creds; `clickhouse-go` does not error at `OpenDB` time, only on first `Ping`. Verified against the version pinned in `go.mod`. Unit test in `driver_test.go` that `Connect(_, forwardOAuth-settings, nil)` returns a non-nil `*sql.DB` without erroring.
- **[Missed `MutateQueryData` invocation if sqlds removes the `QueryDataMutator` extension upstream]** → Mitigation: the interface conformance assertion in `driver.go:36` (`_ sqlds.QueryDataMutator = (*Hydrolix)(nil)`) fails the build if upstream removes the interface. Loud, not silent.
- **[Cache-key normalisation (D5) breaks production observability that expected the "Bearer " prefix in connection-arg dumps]** → Mitigation: nothing the plugin emits exposes `connectionArgs` to logs or metrics today. If observability surfaces a need (e.g., to grep logs for a token without the prefix), the normalisation works in observability's favour, not against it.
- **[Pluggable cache (C3) holds a still-authenticated `*sql.DB` past TTL for an offline user]** → Acceptable: TTL is one hour. The risk is one hour of holding an OAuth-token-authenticated connection after the user logs out. This matches the fork; petition upstream Hydrolix if a deployment surfaces a need for sub-hour invalidation.

## Migration Plan

- **Forward**: ships in the C2-C7 coordinated merge window. Sequence inside its PR commit (or PR if stacked):
  1. Extend `MutateQueryData` in `pkg/plugin/driver.go` per D1.
  2. Add `pkg/plugin/connection_args.go` + `pkg/plugin/connection_args_test.go` per D2.
  3. Update the `forwardOAuth` branch of `Driver.Connect` per D3.
  4. Update existing `MutateQueryData` tests to assert the additional `connectionArgs` write.
  5. Add new unit tests per D6 (cross-request safety), D2 (injectConnectionArgs round-trip), D3 (lazy bootstrap).
  6. Run quality gates: `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`.
  7. E2E via `grafana-plugin-e2e`: deferred until C5-C7 also land (no Hydrolix query path in isolation).
- **Rollback**: revert this change's commit/PR. The plugin reverts to the fork-style behaviour from before the migration started, but only if C2 also reverts. Within the migration window, partial rollback is unsupported.
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2) and `plugin-ttl-connection-cache` (C3). Independent of C5-C7.

## Open Questions

- Should `MutateQueryData` log a debug-level line per OAuth-injection (e.g., `"applied OAuth keying" with refId and a truncated token hash`)? The token must not appear in logs; a hash is safe. Defer — current logging surface is minimal and adds nothing operational until a deployment surfaces a debugging need.
- Should the plugin expose `connectionArgs` shape (`oauthToken`, future fields) as a typed struct rather than `map[string]string`? Defer — single-field `map[string]string` is simpler than a one-field struct, and any second field would warrant the type then.
- Should the lazy-bootstrap path emit a metric (`bootstrap_lazy_total`) so operators can spot deployments stuck on bootstrap-only? Defer — existing metrics surface in `pkg/plugin/metrics.go` is opt-in; add a metric only if a need surfaces.
