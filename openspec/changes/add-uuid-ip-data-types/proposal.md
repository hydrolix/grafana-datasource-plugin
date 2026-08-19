## Why

Hydrolix tables expose `UUID`, `IPv4`, and `IPv6` columns, but the plugin has no
converter for them. `sqlutil.MakeScanRow` falls back to a `sql.NullString` scan
target that `database/sql` cannot fill from the driver's `uuid.UUID` / `net.IP`
values, so any query selecting such a column fails outright instead of returning
data. The same omission hides these columns from the ad-hoc filter key picker.

## What Changes

- Add `UUID`, `IPv4`, `IPv6` and their `Nullable(...)` variants to the plugin's
  converter registry (`pkg/converters`), rendering each as a string frame field.
- Extend the frontend supported-type list (`src/constants.ts`) so UUID/IP
  columns — and their `Nullable`, `Array`, and `Map` wrappers — become eligible
  ad-hoc filter keys.
- Seed the ClickHouse e2e fixture with a table carrying UUID/IP columns in both
  nullable and non-nullable form, populated and NULL.
- Add Go unit, testcontainer integration, and Playwright e2e coverage.
- Non-breaking: no existing type mapping, query shape, or dashboard behavior
  changes. Grafana 10.x dashboards are unaffected.

## Capabilities

### New Capabilities
- `hdx-column-type-converters`: which ClickHouse column types the plugin maps
  into Grafana data frames, the frame field type each produces, NULL handling,
  and which types are eligible as ad-hoc filter keys.

### Modified Capabilities

None. Ad-hoc filter SQL generation, the query models, and the sqlds wrapper keep
their current requirements.

## Impact

- `pkg/converters/converters.go` — new registry entries and IP-to-text helpers.
- `src/constants.ts` — supported-type list consumed by `getKeyMap` in
  `src/editor/metadataProvider.ts`.
- `testdata/containers/initdb.sql` — new e2e fixture table.
- `pkg/converters/converters_test.go`, `pkg/plugin/driver_conv_test.go`, and a
  new Playwright spec under `tests/`.
- No new dependencies: `net` is stdlib, and `google/uuid` already arrives
  transitively through the ClickHouse driver.
- Panels, table views, transformations, and ad-hoc filters all consume UUID/IP
  columns as strings, so downstream string transformations apply unchanged.
