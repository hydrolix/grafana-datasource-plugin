# Tasks

## 1. Scope the caches

- [x] 1.1 `cacheScope(headers)` derives the scope from the forwarded token's
      subject: stable across a refresh, distinct between people. A token whose
      subject cannot be read falls back to a digest of the token — wasteful
      across refreshes, and the right way to be wrong, since the alternative
      shares one entry between different people.
- [x] 1.2 `scopedKey(scope, key)` returns the bare key for an empty scope, so a
      mode that forwards no identity keeps today's behaviour exactly: one
      credential is one legitimate view.
- [x] 1.3 Log lines name the table or CTE, not the key.

## 2. Tests

- [x] 2.1 Two users, one table: the second user's lookup must reach the cluster.
- [x] 2.2 A mode forwarding no identity still shares one entry.
- [x] 2.3 The existing suites are unaffected.
- [x] 2.5 The repo's e2e suite run against this branch (Grafana 13.2.1): 59
      passed, 1 flaky — `macroFunctions / fromTime_ms`, which passed on retry
      and flaked on the untouched `develop` tree too. Nothing here is a
      regression from scoping the caches.
- [ ] 2.4 No e2e of the crossing itself. The crossing needs two signed-in users against one datasource
      and a cluster that refuses one of them; the e2e stack runs a single user
      against a local ClickHouse, so an e2e here would assert nothing the Go
      tests do not. Recorded rather than silently skipped.

## 2b. The cost, measured

- [x] 2b.1 Per lookup against a local ClickHouse: primary key 3.7 ms / ~2.9 KB,
      DESCRIBE 2.1 ms / ~2.8 KB (20 runs each). A real cluster adds round-trip
      time; the sizes hold.
- [x] 2b.2 Cache-key cardinality across the real bundle dashboards: **1–3
      distinct tables each**, against 8–36 macro uses. Macro occurrences do not
      each cost a lookup — they resolve onto the same few tables and share
      entries, which is what keeps this cheap.
- [x] 2b.3 So the added load is `(U−1) × K` lookups per hour for `U` users and
      `K` tables: at 100 users and K=3, ~297/hour, one every 12 seconds,
      ~870 KB/hour. It scales with distinct tables, not with panels or macro
      use.

## 3. Landing

- [x] 3.1 CHANGELOG entry, flagged as a behaviour change for anyone on
      `forwardOAuth` today.
- [x] 3.2 Spec delta.
