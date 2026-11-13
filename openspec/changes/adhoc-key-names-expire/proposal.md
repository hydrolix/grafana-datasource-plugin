# Ad hoc filter key names expire after a minute

## Why

A Hydrolix table gains column names while dashboards are open. A naming profile
publishes `userAgent` as an alias of the existing column `UA`; the add-datasource
wizard and Semantix add names too. On console-playpen, after `userAgent` was
published on `tpnaming_20261112_9fdfab.logs` (13 Nov, 22:41:50Z), the dashboard's
filter key list did not offer it. It appeared only after the datasource was opened
and saved with **Save & test**.

The plugin keeps two copies of a table's column names, and neither notices a new
name:

- **The key list in the browser** is cached on the DataSource instance with no
  expiry. Grafana keeps that instance until the page reloads or the datasource is
  saved, so a long-lived tab never sees a new name.
- **The column-type map in the backend**, which decides which ad hoc filters
  `$__adHocFilter` applies, is cached for an hour. Every read extends that hour,
  so on a dashboard that queries more often than hourly the map never expires
  until the datasource is saved. A filter whose column the map does not list is
  dropped, so a filter on a newly published name returns unfiltered rows, with no
  error.

Saving the datasource is an administrator's action, and a customer looking at a
dashboard cannot be expected to know about it.

## What Changes

- The browser's key list for a table expires one minute after the DESCRIBE that
  produced it; the next key or value dropdown reads the table again.
- The backend's column-type map for `$__adHocFilter` expires one minute after the
  DESCRIBE that produced it; reading it no longer extends it.
- The primary-key cache, the editor's autocomplete lookups and the connection
  cache are unchanged.
- No new setting, no UI change, not breaking.
- Unit coverage on both sides (Jest and Go); Playwright e2e for the key list.

## Capabilities

### New Capabilities

- `adhoc-key-list`: how long the ad hoc filter key list for a table is reused.

### Modified Capabilities

- `hdx-adhoc-filter-macro-secure`: the column-type map's TTL is one minute,
  counted from the fetch, and not extended by reads.

## Impact

- `src/constants.ts`, `src/editor/metadataProvider.ts`; `pkg/plugin/metadata.go`.
- More DESCRIBE traffic: about one per table per minute while dropdowns are
  opened, and one per (identity, table) per minute for each filtered panel whose
  query misses at the moment the entry expires.
