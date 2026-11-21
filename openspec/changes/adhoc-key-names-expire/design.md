# Design: ad hoc filter key names expire after a minute

## Context

Two caches hold a table's column names.

- Browser: `getMetadataProvider` memoizes `tableKeys(table)` (one DESCRIBE) for
  the life of the DataSource instance. Grafana 13 keeps that instance in
  `DatasourceSrv.datasources` until `reload()`, which runs only when a datasource
  is created or saved (`features/datasources/state/actions.ts`, `updateDataSource`),
  or until the page loads again. Grafana scenes asks the plugin on every key or
  value dropdown open (`AdHocFiltersVariable._getKeys` → `ds.getTagKeys`); it keeps
  no list of its own. So the plugin's memo is the only cache in front of the
  dropdown.
- Backend: `MetadataProvider.keyCache` (ttlcache v3, 1 h) maps a CTE or table to
  its column types for `$__adHocFilter`, which skips a filter whose column is not
  in the map. ttlcache's `Get` extends an entry's expiry on every hit unless the
  cache is built with `WithDisableTouchOnHit`.

Neither system offers a schema revision the plugin could key on: Hydrolix
`DESCRIBE` returns none, and the config API that records name changes is not one
the datasource talks to.

## Decisions

### D1. A fixed one-minute TTL on both sides, counted from the fetch

The browser stores the time each DESCRIBE was issued with its result and re-reads
once a minute has passed. The backend's key map uses a one-minute TTL with touch
on hit disabled. One minute keeps the cost at one metadata query per table per
minute under use, and is short enough that a name published during a session
shows up while the person is still looking.

- *Alternative: invalidate on a schema revision.* No revision is available
  through the query interface (see Context).
- *Alternative: a refresh button in the filter list.* The list belongs to Grafana
  scenes; a datasource plugin cannot add controls to it, and a person has to know
  to press it.
- *Alternative: re-DESCRIBE in the backend whenever a filter's column is missing.*
  In a dashboard whose panels read different tables, a filter's column is missing
  from the other tables by design, so this re-reads on every panel query unless
  rate-limited, and the rate limit is a TTL again.

### D2. The primary-key cache keeps its hour

A primary key is not a name a person picks from a list, and it does not change
when a name is published. Shortening it is outside this change.

## Risks / Trade-offs

- [The browser and the backend refresh independently. If the backend read the
  table less than a minute before a name was published and the browser read it
  after, the dropdown offers the name while the backend still drops a filter on
  it: a panel query in that window returns unfiltered rows, and a dashboard
  without auto-refresh keeps showing them until its next query] → The window
  closes when the backend entry expires, at most a minute after it was filled.
  Waiting a minute after a publish before filtering on the new name avoids it.
  Proving signal: Go test
  `TestAFilterOnANewlyPublishedNameAppliesOnABusyDashboard` and the Playwright
  spec `adHocKeyNamesExpire`, whose panel count goes from unfiltered to filtered
  without a reload. Re-reading on a missing filter column would close the
  window, but a dashboard whose panels read several tables misses by design on
  every query, so it needs a per-key memory of what was already re-read: a
  follow-up if the window matters.
- [A cluster error when the key list is re-read now surfaces in the dropdown
  ("An error has occurred fetching labels") where the old list was served
  before] → The dropdown's own retry reads again; panels hit the same cluster
  and show the same error. Proving signal: the scenes error text in the
  dropdown, and the failed DESCRIBE in the Grafana server log.
- [More DESCRIBE traffic] → At most one per table per minute per dropdown user,
  and one per (identity, table) per minute while filtered panels refresh, times
  the number of panels whose queries miss at the same moment (misses are not
  coalesced; before this change that burst happened once per instance). Proving
  signal: `MetadataProvider: keys cache miss` debug lines in the Grafana server
  log, and DESCRIBE entries in the cluster's query log.
