# Tasks

## 1. Browser key list

- [x] 1.1 `AD_HOC_KEYS_TTL_MS` (one minute) in `src/constants.ts`.
- [x] 1.2 `tableKeys` stores the time its DESCRIBE was issued and re-reads once
      `AD_HOC_KEYS_TTL_MS` has passed; entries stay per table.

## 2. Backend column-type map

- [x] 2.1 `adHocKeysTTL` (one minute) for `keyCache`, with touch on hit disabled.
- [x] 2.2 `newMetadataProvider(ds, keysTTL)` so tests can watch an entry expire;
      `NewMetadataProvider` passes `adHocKeysTTL`.

## 3. Tests

- [x] 3.1 Jest: a name published after the first read appears at the TTL and not
      before; one DESCRIBE per TTL per table.
- [x] 3.2 Go: an entry's TTL is more than zero and at most a minute; a hit does
      not move its expiry; `$__adHocFilter` applies a filter on a newly published name within the TTL
      while it is queried faster than the TTL.
- [x] 3.3 The existing Jest and Go suites pass (`go test -race`).
- [x] 3.4 Playwright e2e (`tests/adHocKeyNamesExpire.spec.ts`): with a
      dashboard open and a filter set, add an ALIAS column to the test's own
      table; within the TTL the dropdown still shows the old list, past it
      (browser clock advanced) it offers the new name, and a filter on it
      applies once the backend's minute has passed (count 3 → 1), all without a
      reload. Local Grafana 13.2.1: passes on this branch (twice, 1.3 min);
      fails on `develop` (the dropdown lists only `UA`); with this branch's
      frontend and `develop`'s backend the filter stays dropped (count 3 for
      2 min). The other ad hoc specs pass on this branch (27/27).

## 4. Docs and archive order

- [x] 4.1 README, ad hoc filters: new names appear within a minute.
- [x] 4.2 The `hdx-adhoc-filter-macro-secure` delta carries the keyless-table
      content of `adhoc-filters-without-primary-key`, which modifies the same
      requirement under its old name; that change is archived first.
