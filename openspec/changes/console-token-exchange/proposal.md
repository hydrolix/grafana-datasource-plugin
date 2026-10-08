# Forward OAuth exchanges the sign-in token for a cluster token

## Why

A Hydrolix Console customer signs in to Grafana through the **console's** Keycloak
realm, not the cluster's. Their sign-in token therefore carries the audience of
the Grafana OAuth client (`grafana-api`), and a Hydrolix cluster refuses it: a
cluster accepts a token from an external issuer only when its `aud` holds the
audience that cluster configures for that issuer. Forwarding the token unchanged
— what `credentialsType: forwardOAuth` does today — fails every panel.

Verified against a live cluster on 2026-10-01: the same signed-in user's token
returns rows when exchanged first, and is refused with
`Code: 516 … AUTHENTICATION_FAILED` when forwarded unchanged.

The console already mints per-user cluster tokens for its own SPA, and exposes an
RFC 8693 token-exchange endpoint for non-browser callers. This change makes the
plugin do for a dashboard what the console's browser client does for its own
views: exchange the signed-in user's token for a short-lived, deployment-scoped
cluster token, cache it per user, and keep it fresh.

Forwarding unchanged stays the right behaviour when Grafana signs people into the
cluster's **own** realm, so this is an additional mode, not a replacement.

## What changes

- A new credentials mode (working name `forwardOAuthExchange`) that behaves like
  `forwardOAuth` except that the forwarded token is exchanged before use.
- The exchange's endpoint, audience and client credentials come from **Grafana
  server configuration**, not datasource settings: they are per-cluster operator
  values, and a datasource holds no secret today — a property worth keeping.
- A per-user token source with the console SPA's semantics: cache until shortly
  before expiry, one exchange in flight per user, a retry ladder on failure, and
  invalidate-and-retry-once on a cluster `401`.
- Connection pooling is unchanged: entries stay keyed on the **forwarded**
  sign-in token, and the exchanged token is attached per request through the
  driver's `TransportFunc` hook, so a token refresh never churns the pool.

## Capabilities

### New Capabilities

- `console-token-exchange`: the `forwardOAuthExchange` credentials type — how a
  forwarded sign-in token becomes a cluster token, where the delegate
  credential comes from and how it is scoped per cluster, how exchanged tokens
  are cached and refreshed per user, which protocol the mode requires, and what
  the health check reports.

### Modified Capabilities

- `hdx-oauth-keyed-pooling`: the exchanging mode keys the connection pool on the
  forwarded token's subject rather than on the token, which is refreshed
  independently of the connection. `forwardOAuth` is unchanged.

## What does not change

- `forwardOAuth`, `serviceAccount` and `userAccount` keep their current behaviour.
- No datasource gains a stored credential.
- Alerting, recording rules, reports, rendering and public dashboards carry no
  user, so they continue to need a service-account datasource. This change does
  not make userless work possible.

## Out of scope

- Grafana's own sign-in and refresh of the user token (`use_refresh_token`).
- Which realm an operator points Grafana at.
- Per-user attribution of a query at the cluster, which nothing records today.
