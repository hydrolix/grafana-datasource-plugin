# Changelog

## 0.12.0

- **Feature**: Grafana Assistant support via context integration and MCP skill (HDX-11525)
- **Feature**: Support UUID, IPv4, and IPv6 column types (HDX-12151)
- **Fix**: Dashboard ad-hoc filter value dropdown times out on high-cardinality columns — bounded `topK` preload with query guardrails (HDX-11854)
- **Refactor**: Pass editor/ad-hoc interpolation context explicitly instead of caching request state on the datasource; the interpolated-query preview follows the current time-picker selection (HDX-11854)
- **Docs**: Document that ad-hoc key/value suggestions are bounded, and recommend enabling the ad-hoc variable's *Allow custom values* option so unlisted keys/values can still be filtered on (HDX-11854)
- **Chore**: Bind provenance to release tags via tag-driven channels (HDX-12163)
- **Chore**: Update create-plugin scaffold to 7.10.0 and fix dependency CVEs from the Aikido report (HDX-12187)

## 0.11.0

- **Feature**: Add Grafana annotations support (HDX-11579)
- **Feature**: Support Grafana 13 while keeping Grafana 10 compatibility (GRAP-160)
- **Feature**: Attribute queries via `hdx_query_comment` with canonical `panelId`/`panelName` defaults
- **Refactor**: Retire the Hydrolix `sqlds` fork and adopt upstream `grafana/sqlds` v5.2.0 (AST interpolator, CTE extraction, TTL connection cache, OAuth/Org-Id keyed pooling, ClickHouse time/date macros, HTTP custom routes)
- **Security**: Close SQL injection in metadata queries
- **Security**: Close ad-hoc filter operator and map-key injection
- **Security**: Resolve WITH-alias CTEs for ad-hoc filters
- **Security**: Fix tasks from Aikido report (#159) and address reported CVEs
- **Fix**: Revert Grafana User telemetry
- **Chore**: Address Grafana plugin review remarks and add the `plugin-validator` release gate
- **Chore**: Update create-plugin scaffold to 7.9.0 (webpack toolchain, ESLint 9 flat config, Prettier 3, `@grafana/*` 13.1.0), bump `grafana-plugin-sdk-go` and CI actions, add `golangci-lint` config

## 0.10.6
- **Fix**: Default compression value
- **Feature**: Improve Grafana User telemetry

## 0.10.5
- **Feature**: Add Grafana User telemetry

## 0.10.2

- **Fix**: Query Editor doesn't store default query type (GRAP-165)
- 
## 0.10.1

- **Fix**: Address Grafana plugin review feedback

## 0.10.0

- **Feature**: Add support for setting Hydrolix query options per query through Grafana plugin UI (GRAP-59)
- **Feature**: Expose datasource errors to grafana variables (GRAP-155)
- **Fix**: Query interpolation fails when cte dependent plugin is commented (GRAP-152)
- **Fix**: AST parser, support of REGEXP operator (GRAP-154)

## 0.9.0

- **Feature**: Remove Query Assistant from Grafana plugin (GRAP-135)
- **Feature**: Support regex patterns for ad hoc filtering (GRAP-77)
- **Feature**: AST Parser should handle || and '' (GRAP-84)
- **Feature**: Add optional parameter for adHocFilter macro to define CTE (GRAP-87)
- **Feature**: Support ad-hoc filtering for subquery variables by escaping macro in Grafana plugin (GRAP-131)
- **Fix**: Error Beautifier doesn't transform error when connection type is native (GRAP-123)
- **Fix**: src/components/ConfigEditor.tsx mutates React props (GRAP-129)
- **Fix**: Trim adHocFilterTableName varible value (GRAP-127)
- **Fix**: Authorization header is missing for HTTP connections when secure=false in data source configuration (GRAP-126)

## 0.8.0

- **Feature**: Grafana Query Assistant v1 (GRAP-88)

## 0.7.0

- **Feature**: Support externally shared dashboards (GRAP-39)
- **Fix**: All macros support for alerts (GRAP-79)
- **Fix**: Support of Math expressions using Hydrolix queries (GRAP-104)
- **Fix**: failing when macro name is present in comment (GRAP-102)

## 0.6.0

- **Feature**: Service Account support (GRAP-41)
- **Feature**: Support of limit values for ad-hoc filter (GRAP-78)
- **Feature**: Add “Run Query” button to the query editor (GRAP-35)
- **Fix**: Query parsing error when user tries to format query (GRAP-75)
- **Fix**: Incorrect handling of single quotes inside ad-hoc filter values (GRAP-80)

## 0.5.0

- **Feature**: Support \$__timeFilter and \$__timeInterval macros without timestamp (GRAP-71)
- **Feature**: Support automatic timestamp column detection in ad hoc filters (GRAP-68)

## 0.4.0

- **Feature**: Support configurable Hydrolix query settings per data source (GRAP-46)

## 0.3.1

- **Fix**: Some queries in template variables fail in Grafana 10.x (GRAP-64)

## 0.3.0

- **Feature**: Support `*` wildcard in ad hoc filters and convert to SQL `%` (HDX-8167)
- **Feature**: Support synthetic ad hoc filter values `__empty__` and `__null__` (HDX-8468)
- **Fix**: Do not crash when ad hoc tag values are missing in dashboard time range (HDX-8605)
- **Fix**: Tooltip for invalid round value causes layout shift in query editor (HDX-8391)
- **Fix**: No loading spinner when changing round value in query editor (HDX-8491)

## 0.2.0

- **Feature**: Add support for `one-of` (`=|`) and `not-one-of` (`!=|`) ad hoc filter operators (HDX-8336)
- **Fix**: Do not crash on complex queries with ad hoc filters (HDX-8193)
- **Data source config**: Drop support for ad hoc query definitions in data source settings (HDX-8173)
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
