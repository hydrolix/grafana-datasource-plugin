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

- [The browser and the backend refresh independently, so for up to a minute after
  a name is published the dropdown can offer it while the backend still drops a
  filter on it] → Bounded by the TTL; both sides use the same value. Go test
  `TestAFilterOnANewlyPublishedNameAppliesOnABusyDashboard` proves the filter
  applies after the TTL while panels keep querying.
- [A cluster error when the key list is re-read now surfaces in the dropdown where
  the old list was served before] → The dropdown's own retry re-reads; panels hit
  the same cluster and show the same error.
- [More DESCRIBE traffic] → At most one per table per minute while dropdowns open,
  and one per (identity, table) per minute while filtered panels refresh.
