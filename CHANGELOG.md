# Changelog

## 0.13.4

- **Security**: Removed the unused `token` field from the data source's clear-text `jsonData` type. The service account
  token was, and is, stored only in `secureJsonData`; with the field gone, code that tried to save it unencrypted no
  longer compiles
- **Fix**: The token exchange client no longer falls back to a plain HTTP client without Grafana's tracing, metrics and
  error-source middlewares when the SDK client cannot be built. The error is reported instead, and a data source set to
  "Forward OAuth with a console token exchange" then reads as misconfigured in queries and in the connection test

## 0.13.3

- **Feature**: New datasource option "Ad hoc filter keys prefer aliases" (additional settings, off by default). When on,
  the ad hoc filter key picker hides a physical column that has a same-type identity alias (`client_ip String ALIAS
  cliIP`) and offers the alias alone, so dashboards can present canonical names (CFB-4004). Computed, type-changing and
  chained aliases stay visible, Map columns keep their key enumeration, and an existing filter on a hidden column still
  loads values. See `docs/adhoc-keys-prefer-aliases.md`
- **Fix**: An identity alias whose DESCRIBE expression is a bare column name (not backtick-quoted) was dropped from the
  ad hoc filter keys; it is now listed like a backtick-quoted one
- **Fix**: The query editor's validation bar warned about an unfiltered primary key on Hydrolix summary tables when the
  key column (`toStartOfDay(reqTimeSec)`) was filtered through its identity alias, e.g. `$__timeFilter(reqTimeSec)`. A
  filter on an alias whose expression is exactly the key column now counts; computed aliases still warn

## 0.13.2

- **Fix**: `$__timeFilter()`, `$__timeFilter_ms()`, `$__timeInterval()` and `$__timeInterval_ms()` without a column
  failed with `primary key not found` when the query read from a WITH alias (HDX-11394): the plugin looked up a table
  named like the alias. They now follow WITH aliases and inline subqueries, chained or nested, to the table underneath
  and use its primary key, also on the left side of a JOIN. When that path has no single table (a JOIN, table function
  or UNION inside an alias), or an alias or subquery renames or drops the time column, the macro fails naming the alias
  and asking for an explicit column instead of emitting SQL the cluster rejects.
- **Security**: Queries over HTTP now run through the Grafana plugin SDK's HTTP middlewares, so Grafana's
  `dataproxy.response_limit` is enforced on the cluster's answer, and the request carries the SDK's tracing, datasource
  metrics and error-source attribution. The middlewares wrap clickhouse-go's own transport, which keeps its dial and
  read timeouts, pooling and TLS; the two that write credentials from Grafana's HTTP settings are left out, so the
  token the forwarding modes put on the request is never overwritten. An answer over the limit fails the query with a
  message naming the limit; redirects stay refused. Flagged by Grafana's plugin review of 0.13.1
- **Security**: A redirect refused from the cluster is reported by status, scheme, host and path only. The `Location`
  a cluster, or whatever answers at its address, sends was logged and shown in the panel error verbatim, so userinfo,
  a query string and a fragment, any of which can carry a credential or a token, are now dropped before the value
  reaches Grafana's log or a dashboard. Flagged by Grafana's plugin review of 0.13.1
- **Security**: Bump the transitive Go dependency `klauspost/compress` to 1.20.2 (GO-2026-6865, CVE-2026-116591, panic
  on two-block frames)
- **Docs**: Drop the installation section from the README. The plugin catalog shows its own installation
  instructions next to the README, so the section was a duplicate; Grafana's plugin review of 0.13.1 asked for its
  removal. Configuration and usage stay
- **Chore**: Build with Go 1.27.4. Aikido reported twelve CVEs in the 1.27.3 standard library and toolchain
  (CVE-2026-97033, CVE-2026-97037, CVE-2026-97038, CVE-2026-97039, CVE-2026-97040 and others), all fixed in 1.27.4.
  The go directive, CI, release and nightly workflows, the pre-commit hooks, the dev container and the e2e exchange
  stub move together

## 0.13.1

- **Fix**: A dashboard query using `SELECT * EXCEPT <column>` with a keyword-named column (`timestamp`, `date`, `key`,
  `name`, `type`, `host`, ...) failed with `expected SELECT, WITH or (, got <keyword>` as soon as an ad hoc filter was
  applied, while the same query ran in Explore (HDX-12821). The plugin parses the statement to find the table behind a
  parameterless `$__adHocFilter()`, and its SQL parser accepted a bare `EXCEPT <column>` only for non-keyword names;
  `* except (timestamp)` and an explicit `$__adHocFilter(db.table)` were the only workarounds. The parser is upgraded to
  the build the Hydrolix HTTP proxy already runs, which also accepts `* EXCEPT STRICT <column>`, `db.table.*`, a SETTINGS
  item named by a reserved keyword (`limit`, `offset`, ...), `interval` as a plain column name, an `AS alias` inside
  `extract(...)`, and a subquery starting with `WITH` in expression position. Regressed in 0.11.0 with the move to
  parser v0.5.2
- **Security**: Bump transitive Go dependencies flagged by the plugin validator: `andybalholm/brotli` to 1.2.8
  (CVE-2026-111497, Brotli decoder out-of-bounds write), `golang.org/x/net` to 0.62.0 (GO-2026-6860, GO-2026-6866) and
  `golang.org/x/crypto` to the pseudo-version govulncheck names as the fix for GO-2026-5932 (no tagged release carries it
  yet; `openpgp` is not linked into the plugin). `govulncheck` now reports no advisories
- **Security**: Override `zod` to 4.6.7 (CVE-2023-54404, memory exhaustion) and `follow-redirects` to 1.16.2
  (CVE-2026-116689, `sensitiveHeaders` leak). Both are build-time or dev-only dependencies, not shipped in the plugin;
  the `follow-redirects` override was then dropped together with its only consumer, the unused `node-rest-client-promise`
  devDependency
- **Chore**: Rename four Go identifiers that Grafana's plugin review flagged under gosec G101 (hardcoded credentials)
  on 0.13.0 (HDX-12726). They held an environment variable name and RFC 8693 URNs, not secrets; the validator runs gosec
  with `-nosec`, so renaming was the only fix. `GF_PLUGIN_EXCHANGE_CLIENT_SECRETS` keeps its name, so configured
  instances need no change. The e2e exchange stub now writes its certificate files through `os.OpenRoot` (G703). G402
  (`InsecureSkipVerify`) is the *Skip TLS verify* datasource option, off by default, and is left as is
- **Chore**: Delete unreachable Go code: `stats_stripping_reader.go` (its only caller went with the disabled
  `hdx_query_streaming_result` path, and QH 6.2.x no longer appends the `X-HDX-Query-Stats` trailer, HDX-11371) and
  `connection_args.go` (`injectConnectionArgs` was never called; `MutateQueryData` writes `connectionArgs` through
  `jsonSet`). The `hdx-oauth-keyed-pooling` spec now names the `jsonSet` path. `pierrec/lz4` becomes an indirect
  dependency and the unused direct `monaco-editor` dependency is removed; the SQL editor comes from `@grafana/ui`
- **Chore**: Bump `grafana-plugin-sdk-go` to 0.298.0, `@grafana/sign-plugin` to 4.0.0, `@grafana/plugin-e2e` to 3.15.1,
  `@playwright/test` to 1.64.1, `sql-formatter` to 15.9.0, `webpack` to 5.112.2, `@swc/core` to 1.16.21, `sass` to
  1.106.1, `prettier` to 3.9.10, `@types/node` to 26.6.5, and the `is-compatible` plugin action to 1.1.0
- **Chore**: Configure Dependabot for npm, Go modules and GitHub Actions on `develop`

## 0.13.0

- **Fix**: No redirect from the cluster is followed, in any credentials mode. Go's HTTP client keeps `Authorization` on a
  redirect to a subdomain of the host and on an https-to-http downgrade, and copies every other header, the
  `X-ClickHouse-Key` password among them, to any host; a proxy in front of the cluster or a mis-pointed datasource
  could send the credential somewhere nobody configured. A 3xx now fails the query naming where it pointed.
- **Feature**: New credentials type *Forward OAuth + Exchange* (`forwardOAuthExchange`). The signed-in user's token is
  exchanged for a cluster token (RFC 8693) before querying, for a Grafana whose users sign in through an issuer the
  cluster does not accept directly. The data source stores no credential; the exchange endpoint and its per-cluster
  credentials come from a `[plugin.hydrolix-hydrolix-datasource]` section in Grafana's configuration file, which
  reaches the plugin as `GF_PLUGIN_EXCHANGE_URL` / `GF_PLUGIN_EXCHANGE_CLIENT_SECRETS`. The audience is always the data
  source's host. Requires verified TLS to the cluster, with *Skip TLS verify* refused, and an `https` exchange URL
  whose redirects are not followed. Requires the HTTP protocol — native binds its credential to the connection and cannot carry a
  refreshed token — and Grafana's `use_refresh_token`, since every exchange after the sign-in token expires is refused.
  A cluster token is reused only by the exact sign-in token it was minted from, so a refresh costs one exchange.
- **Fix**: A query's own `connectionArgs` no longer reach the connection cache in any credentials mode. sqlds reads
  them from the query JSON the browser sends, so a caller could otherwise choose which cached connection a query ran
  on
- **Fix**: `Save & test` on either forwarding mode reported a working data source as degraded. The check ran on the
  bootstrap connection, which has no signed-in user, so it tested something no query uses. Both modes now report what
  can actually be established at configuration time
- **Fix**: `$__adHocFilter(<table>)` with an explicit argument rejected a Hydrolix custom view or any quoted table name
  with `invalid $__adHocFilter argument` (HDX-12561): the argument allowed only bare `table` or `database.table`. It now
  also accepts backtick- and double-quoted names, so ``$__adHocFilter(akamai.`logs#redacted`)`` resolves the view's
  columns. As in the table variable, quote a view name; an unquoted `#` is still rejected
- **Fix**: Ad hoc filter values showed "No options found" when the ad hoc table variable named a Hydrolix custom view
  (``akamai.`logs#redacted` ``) or any backtick- or double-quoted table (HDX-12543). Plain-column keys still loaded,
  but the primary-key lookup behind the preloads searched `system.tables` for the quoted spelling and found no row: the
  value query was never sent, and `Map` columns (`attrs['…']`) dropped out of the key list because their key discovery
  runs a `$__timeFilter()` that failed on the backend. In any query whose FROM names such a table, `$__timeFilter()`,
  `$__timeFilter_ms()`, `$__timeInterval()` and `$__timeInterval_ms()` without a column argument failed the same way
  with `primary key not found`. The lookup now uses the bare name; each custom
  view has its own `system.tables` row carrying the base table's primary key. The table variable no longer needs a view
  name in backticks: the plugin quotes any part that is not a plain identifier, so `akamai.logs#redacted` is sent as
  ``akamai.`logs#redacted` ``. A name without a database (including a quoted one containing a dot, `` `my.logs` ``) now
  takes the datasource's default database for DESCRIBE and the value query too, not only for the key lookup
- **Fix**: Ad hoc filters on a table without a primary key (HDX-11399). The key and value dropdowns came up empty when
  `system.tables.primary_key` is `""`, and map-key discovery (`attrs['…']`) on such a table failed with a ClickHouse
  parse error because the empty key was interpolated into `$__timeFilter()`. Both now issue a keyless preload: no
  `$__timeFilter`, no time range, still bounded by `topK(100)`, `hdx_query_max_execution_time = 10` and the shared
  `SETTINGS` suffix. Keyed tables issue the same statements as before. A `system.tables` lookup that fails is no longer
  mistaken for a keyless table: it is not memoized and issues no preload. On the backend, `$__timeFilter()`,
  `$__timeFilter_ms()`, `$__timeInterval()` and `$__timeInterval_ms()` without a column argument on a keyless table now
  fail with `table has no primary key`, naming the table and hinting to pass the time column as the argument
- **Fix**: The signed-in user's auth header did not reach the plugin backend's metadata lookups. It is set on
  `HdxQuery.Headers`, which is `json:"-"`, so the `/interpolate` route marshalled it away and the QueryData path never
  filled it — the macros found nil. It now travels on the request context, set at both entry points. Two things were
  broken by that and are fixed together: the per-user metadata cache scope below, and a macro's metadata lookup, which
  on a cold cache reached the cluster with no credential at all (`forwardOAuth: missing OAuth token in connection args`)
- **Fix**: Schema metadata was cached across users in `forwardOAuth` mode. `MetadataProvider` keyed a table's primary
  key and a CTE's columns on the table or CTE alone, for an hour per datasource instance, and the caller's token
  reached the cluster only on a miss — so one user's lookup served every other user of that datasource that table's
  primary key and column name→type map, without the cluster authorizing them. Schema shape, not rows. Both caches now
  key on the forwarded identity; modes that forward no identity are unchanged. **Behaviour change for `forwardOAuth`
  users**: cache hit rate falls, so more metadata queries reach the cluster
- **Security**: Bump `source-map-js` to 1.2.2 to clear CVE-2026-93749 (GHSA-68fv-2mgg-jv7q, high severity), which fails the plugin validator. Build-time dependency only; it is not shipped in the plugin
- **Security**: Override `follow-redirects` to 1.16.1 to clear CVE-2026-111755 and CVE-2026-111756 (both high severity), which fail the plugin validator. Reached only through the dev dependency `node-rest-client-promise`; not shipped in the plugin
- **Chore**: Pin Go to 1.27.3 and `golang.org/x/net` to 0.61.0 — Grafana's govulncheck reported twelve reachable advisories against 1.26.8 and two against x/net 0.59.0; it now finds none. The go directive, CI, release and nightly workflows, the dev container and the e2e exchange stub move together
- **Chore**: Update create-plugin scaffold to 7.11.0
- **Chore**: Add a nightly compatibility canary (HDX-12393) that runs the e2e suite against every supported Grafana release with `datetime.useLuxon` off, and against the nightly image with the toggle off and on; a failing night files or updates a labelled GitHub issue naming the failing cells
- **Chore**: Let Dependabot and fork PRs pass CI without the signing token (HDX-12501) — PR CI packages the plugin unsigned when the token is absent, and the release workflow asserts that exactly one signed zip exists before publishing

- **Feature**: Validate queries in the query editor before they run (HDX-11409). About a second after you stop typing, a status bar under the SQL editor dry-runs the query with `EXPLAIN` on the cluster and reports *Query is valid* or the cluster's error (unknown column/table, syntax, permissions). It also warns when a table's primary (timestamp) key is not filtered in `WHERE` — add `$__timeFilter()` to avoid scanning the whole table; a filter in the query that reads a CTE or subquery counts. Validation never blocks running the query; only SELECT statements are dry-run. The `EXPLAIN` dry-run carries `query-validation` in `${__hydrolix.app}` / `${__hydrolix.query_source}`, so it can be told apart in query logs. Limitations: the primary-key warning covers single-table SELECTs (queries with a JOIN, comma join or `ARRAY JOIN` are not checked), it is a name match (any mention of the key column in `WHERE`/`PREWHERE` counts, while a key referenced only through a `SELECT` alias is reported as unfiltered), and tables without a primary key are never warned

- **Feature**: Replace the unused *Ad hoc filter default time range* setting with *Ad hoc filter suggestion lookback* (`adHocTimeRangeLookback`, default `24h`; supported units `s`, `m`, `h`). It sets the ad hoc key/value suggestion window, the cap applied to longer dashboard ranges, and the `hdx_query_max_timerange_sec` guardrail (lookback + 10m). In the config editor an invalid value is cleared when the field loses focus; an invalid value that bypasses the editor (e.g. provisioned) falls back to 24h at runtime. `d` is still accepted for existing values. A previously saved default time range is ignored; it was never applied

- **Breaking**: Drop Grafana 10.4 support — `grafanaDependency` is now `>=11.0.0`, and 10.4 is removed from the e2e, compatibility-check and nightly matrices. Instances on 10.4 will no longer be offered the plugin update

## 0.12.2 (compatible with Grafana <=13.3.0-34421444802 pre-release)

- **Fix**: Plugin fails to import on Grafana builds with the Luxon-backed datetime layer (`datetime.useLuxon` feature toggle, Grafana Cloud / upcoming 13.3) — `dateTime().subtract("5m")` in the default ad-hoc time range used an invalid argument that Moment silently ignored but Luxon rejects at module load
- **Fix**: An invalid round interval (query editor and datasource default) is now reset to empty when the field loses focus, instead of being saved and silently skipped by the backend; field descriptions now list the actually supported units (s, m, h — `ms` was never accepted)
- **Chore**: Pin `nwsapi` to 2.2.24 — 2.2.26+ hangs `@testing-library/user-event` on portal-rendered elements (Grafana renders tooltips in a portal), which stalled the frontend test suite (upstream `dperini/nwsapi#214`)
- **Chore**: Run every CI job on Node 24, matching `.nvmrc` and the `engines` floor — the e2e job had no Node setup and fell back to the runner default (Node 22 / npm 10), which cannot install an npm 11 lockfile
- **Chore**: Stop leaking the Grafana image tag into the e2e test process — `@grafana/plugin-e2e` reads `GRAFANA_VERSION` as a semver, so the `nightly` tag made every internal version comparison return `NaN` and the page models took their oldest code paths (13 nightly tests timed out clicking a Grafana-8 "Add new panel" testid)

## 0.12.1 (compatible with Grafana <=13.2)

- **Fix**: Plugin fails to load on Grafana ≥ 13.2 (React 19) — replace the bundled React 18 `jsx-runtime` with a version-agnostic shim built on `React.createElement`, so a single bundle works on React 16–19 / Grafana 10.4–13.2
- **Fix**: Drop `console.error` from the query error path; errors are reported via `@grafana/runtime` `logError`
- **Security**: Bump `grpc` (1.83.2), `fast-uri`, and `js-yaml` (4.3.2 / 3.15.2) to clear osv-scanner high-severity findings
- **Chore**: Pin Go to 1.26.8 (same security backports as 1.27.1) so Grafana's govulncheck source scan can run — its govulncheck is still built with Go 1.26
- **Chore**: Extend the compatibility-check matrix to current Grafana patch releases, including 13.1 and 13.2

## 0.12.0 (compatible with Grafana <=13.1)

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
