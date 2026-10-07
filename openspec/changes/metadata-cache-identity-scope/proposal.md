# Schema metadata is cached per user, not per datasource

## Why

`MetadataProvider` caches a table's primary key on `database + "_" + table` and
a CTE's columns on the CTE reference, for an hour, per datasource instance. The
caller's headers — carrying the signed-in user's token — reach the cluster only
on a **miss**.

In `forwardOAuth`, where every user of one datasource is a different person,
that means the first user's lookup populates the cache and for the next hour
every other user of that datasource is served that table's primary key and its
column name→type map without the cluster ever authorizing them. What leaks is
schema shape, not rows — but it is served to people the cluster may refuse.

This is in the plugin as it ships. It is not introduced by any new mode, and it
needs none of the work around it to be fixed.

There is no workaround short of this change. An operator cannot scope the cache
from configuration, and the only setting that touches it — shortening the TTL —
narrows the window without closing it, at the cost of a cluster round trip per
table per user per window. Turning a datasource over to one user each would
avoid it and defeat the point of a shared instance.

## What changes

- `pkg/identity` (new): carries the forwarded token on the request context. Its
  own package because `pkg/plugin` imports `pkg/api` for the resource routes,
  so the route handler cannot reach back into `pkg/plugin`.
- `pkg/plugin/driver.go`: `MutateQueryData` puts the identity on the context.
- `pkg/api/routes.go`: the `/interpolate` route does the same, from its own
  request headers.
- `pkg/plugin/metadata.go`: both caches key on that identity as well as the
  table or CTE; and `executeQuery` sets the auth header on the inner request it
  builds, which previously carried none.
- A mode that forwards no identity keeps the key it has always used.
- Cache log lines name the table or the CTE rather than the key, which now
  carries a subject.
- Unit coverage for the crossing itself: two users, one table, the second must
  reach the cluster.

Non-breaking. Cache hit rate falls in forwarding modes, which is the cost of
each person's view being their own; every other mode is unaffected.

## Capabilities

### New Capabilities

- `metadata-cache-identity-scope`: what a schema-metadata cache entry belongs
  to, and which identity it may be served to.

## Out of scope

Row-level results, which were never cached, and query attribution.
