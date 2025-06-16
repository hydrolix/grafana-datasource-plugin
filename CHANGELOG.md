# Changelog

## 0.3.0

- **Feature**: Support `*` wildcard in ad hoc filters and convert to SQL `%` (HDX-8167)
- **Feature**: Support synthetic ad hoc filter values `__empty__` and `__null__` (HDX-8468)
- **Fix**: Do not crash when ad hoc tag values are missing in dashboard time range (HDX-8605)
- **Fix**: Tooltip for invalid round value causes layout shift in query editor (HDX-8391)
- **Fix**: No loading spinner when changing round value in query editor (HDX-8491)

## 0.2.0

- **Feature**: Add support for `one-of` (`=|`) and `not-one-of` (`!=|`) ad hoc filter operators (HDX-8336)
- **Fix**: Do not crash on complex queries with ad hoc filters (HDX-8193)
- **Data source config**:  Drop support for ad hoc query definitions in data source settings (HDX-8173)
- **Chore**: Update plugin screenshots and exclude test Go code from release build (HDX-8422)

## 0.1.6

- **Fix**: Apply default round setting to queries defined in template variables

## 0.1.5

- **Feature**: Add option to show interpolated SQL in the query editor
- **Feature**: Add new column type support for ad hoc filter keys

## 0.1.4

- **Fix**: Resolve issues reported by the Grafana plugin validator to comply with publishing requirements

## 0.1.3

- **Fix**: Rename plugin ID to follow the naming convention

## 0.1.2

- **Feature**: Add support for alerting

## 0.1.1

- **Compatibility**: Improve compatibility with Grafana 10.4.x

## 0.1.0

- Initial beta release.
