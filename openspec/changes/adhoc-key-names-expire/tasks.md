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
- [x] 3.2 Go: an entry's TTL is at most a minute; a hit does not move its expiry;
      `$__adHocFilter` applies a filter on a newly published name within the TTL
      while it is queried faster than the TTL.
- [x] 3.3 The existing Jest and Go suites pass (`go test -race`).
- [ ] 3.4 Playwright e2e: with a dashboard open, add an ALIAS column to the
      fixture table, advance the browser clock past the TTL, and the key
      dropdown offers the new name without a reload.

## 4. Docs

- [x] 4.1 README, ad hoc filters: new names appear within a minute.
