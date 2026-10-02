# Design

## Where the exchange happens, and why not where you would expect

The obvious place is `Driver.Connect`, beside the existing `forwardOAuth` branch
that puts `Authorization: Bearer <token>` into `opts.HttpHeaders`. That is the
wrong place, and the reason is the connection cache.

`MutateQueryData` injects the forwarded token into `connectionArgs.oauthToken`,
and sqlds keys its connection cache on those args
(`openspec/specs/hdx-oauth-keyed-pooling`). `TTLConnectionCache` then holds the
resulting `*sql.DB` for up to an hour. Headers built in `Connect` are frozen into
that cached connection. An exchanged cluster token lives **300 s** (the console's
Keycloak governs the lifetime; its facade only caps it). So a header built at
connect time is dead for roughly 55 minutes of the connection's life.

Two ways out:

1. **Key the pool on the exchanged token.** Correct, and simple to reason about,
   but it mints a new `*sql.DB` per user every few minutes — each with its own
   25-connection pool — and leaves the old ones to idle out. Pool churn
   proportional to active users divided by token lifetime.
2. **Keep the pool keyed on the forwarded token and make the credential
   per-request.** The fork of clickhouse-go this plugin already depends on
   exposes `TransportFunc func(*http.Transport) (http.RoundTripper, error)`
   (`clickhouse_options.go`). A `RoundTripper` that sets the Authorization header
   on each outbound request, reading from a per-user token source, makes refresh
   invisible to pooling.

**This change takes (2).** It is also what the console's own client does: the
browser attaches a token from its session manager at request time rather than
binding one to a connection.

The forwarded sign-in token remains the pool key. It is stable for a Grafana
session, so one user gets one pooled connection, and two users never share one.

## The token source, ported from the console's session manager

`packages/cluster-client/src/session/ClusterSessionManager.ts` in the console
repo is the reference. The semantics worth porting exactly, because they were
each paid for by an incident:

- **Client-clock deadlines.** The wire gives a duration (`expires_in`); stamp the
  deadline from the moment the request *started*, so the local deadline sits at
  or before the server's true expiry by one round trip. Never compare a server
  timestamp to local time.
- **Refresh ahead, proportionally.** Lead time is 20 % of the token's lifetime,
  clamped to [60 s, 300 s]. A 300 s token refreshes at 240 s. A token shorter
  than 120 s is never scheduled and stays on the lazy path.
- **Lazy skew.** With no background renewal running, treat a token inside 30 s of
  expiry as already expired, so no in-flight request carries a dead token.
- **One exchange in flight per user.** Concurrent panels coalesce onto a single
  exchange. A 30-panel dashboard makes one, not thirty — measured on the console
  prototype, and the reason the console's facade quota survives a dashboard load.
- **A retry ladder, bounded by expiry.** 5 s → 10 s → 20 s → 40 s → 60 s cap, each
  rung taken only if it lands before the current token's real expiry.
- **Invalidate and retry once on a cluster 401.** The cluster is the authority on
  whether a token is still good; a 401 means re-mint, once, then surface it.
- **A generation counter.** An exchange that started before an invalidation must
  not populate the cache afterwards, or a torn-down session resurrects itself.

What is deliberately *not* ported: the console's `provision` path on a `409
not_provisioned`. The console's exchange facade provisions inline, so a delegate
never sees a 409.

## Configuration

Server configuration, under the plugin's section:

| Key | Meaning |
|---|---|
| `exchange_url` | The console's RFC 8693 endpoint |
| `exchange_audience` | The audience to request — the cluster's configured audience |
| `exchange_client_id` / `exchange_client_secret` | The delegate credential, one per cluster |

All four absent ⇒ the mode is unavailable and a datasource configured for it
fails its health check with a message naming what is missing. A secret belongs in
server config, never in `jsonData`.

## Failure messages a panel can act on

A panel must be able to tell these apart, because the remedies differ:

| Condition | Panel says |
|---|---|
| The console refused this person for this cluster | access was refused for this account |
| The console could not be reached, or could not mint | the console is unavailable, so the panel has no cluster token |
| The mode is configured but the server config is incomplete | an operator has to finish configuring the exchange |

The console's facade answers with a closed error vocabulary (RFC 6749 §5.2 plus
RFC 8693 §2.2.2) and never includes token material, so these three can be
distinguished without parsing prose. `temporarily_unavailable` is the console
saying "not now" — a removal converging, Keycloak unreachable — and reads as
unavailable rather than as a refusal of the person.

## Hygiene

No token, no SQL and no cluster response body in a log line. The console's
prototype shim logs a subject prefix, an outcome and a latency and nothing else;
its self-test asserts that no token, query text or client secret can appear in
any log output. The same assertion belongs here.

## Reference implementation

The console repo carries a working stand-in at
`spikes/grafana-oauth-forward/shim.py`: a stateless proxy that performs this
exchange in front of a cluster, with the cache, the single-flight and the panel
messages above. `selftest.py` beside it is an executable statement of the
contract — thirty concurrent requests causing one exchange, a refusal never
reaching the cluster, `temporarily_unavailable` reading as unavailable, and no
secret in any log line. It is a spike, not a dependency; it exists so this
change has something to be compared against.
