## 1. Backend converters

- [x] 1.1 Add `ipConverter` to `pkg/converters/converters.go`: assert `*net.IP`, return its text form, error naming expected and actual types on mismatch (the renderer became type-driven in 7.4)
- [x] 1.2 Add `nullableIPConverter`: assert `**net.IP`, return `(*string)(nil)` when the inner pointer is nil, otherwise a freshly allocated `*string`
- [x] 1.3 Add `UUID` and `Nullable(UUID)` entries to `convertersMap` using the `*string` / `**string` scan types and `defaultConvert`
- [x] 1.4 Add `IPv4`, `IPv6`, `Nullable(IPv4)`, `Nullable(IPv6)` entries wired to the two new converters with `FieldTypeString` / `FieldTypeNullableString`
- [x] 1.5 Confirm `go build ./pkg/...`, `go vet ./...`, and `golangci-lint run` are clean

## 2. Backend unit tests

- [x] 2.1 Add `TestUUID`, `TestNullableUUID`, `TestNullableUUIDShouldBeNil` to `pkg/converters/converters_test.go` following the existing `getConverter` helper style
- [x] 2.2 Add `TestIPv4` and `TestIPv6` asserting the returned `string` for a 4-byte and a 16-byte `net.IP`
- [x] 2.3 Add a test pinning the D5 decision: a 16-byte IPv4-mapped `net.IP` in an `IPv6` column keeps the `::ffff:` prefix, matching ClickHouse (retargeted in 7.7)
- [x] 2.4 Add `TestNullableIPv4` / `TestNullableIPv6` plus their `...ShouldBeNil` variants asserting `(*string)(nil)`
- [x] 2.5 Add a test asserting an IP converter errors on a mismatched input type
- [x] 2.6 Add a test asserting distinct values across several rows do not collapse (aliasing contract)
- [x] 2.7 Add a test asserting each of the six type names resolves to exactly one registry entry regardless of iteration order
- [x] 2.8 Run `go test -race ./pkg/converters/`

## 3. Backend integration tests

- [x] 3.1 Add `UUID`, `IPv4`, `IPv6` string entries to `testData` in `pkg/plugin/driver_conv_test.go` so `ConvertersTestSuite` covers them over both native and HTTP
- [x] 3.2 Run `go test -race ./pkg/plugin/ -run TestConvertersTestSuite` (needs Docker)

## 4. Ad-hoc filter eligibility

- [x] 4.1 Add `"UUID"`, `"IPv4"`, `"IPv6"` to `SUPPORTED_TYPES` in `src/constants.ts`
- [x] 4.2 Extend the describe mock in `src/__mocks__/tableDescribes.ts` with UUID/IPv4/IPv6/`Nullable(UUID)` columns
- [x] 4.3 Add a `getKeyMap` unit test in `src/editor/metadataProvider.test.ts` asserting those columns are returned as ad-hoc filter keys
- [x] 4.4 Confirm the existing `ARRAY_TYPES` length assertion still passes and extend the `toContain` cases to cover `Array(UUID)` / `Array(IPv4)`
- [x] 4.5 Verify against a live ClickHouse container that `col = '<literal>'` is accepted for UUID, IPv4, and IPv6 columns — passed against `clickhouse/clickhouse-server:24.8`, all three comparisons returned `count() = 1`, so `buildFilterCondition` needs no UUID/IP branch
- [x] 4.6 Run `npm run typecheck`, `npm run lint`, `npm test -- --ci`

## 5. E2E coverage

- [x] 5.1 Add an `e2e.datatypes` table to `testdata/containers/initdb.sql` with `datetime DateTime64(3, 'UTC')` plus `UUID` / `IPv4` / `IPv6` columns and their nullable counterparts
- [x] 5.2 Seed two deterministic rows: one fully populated, one with NULLs in every nullable column
- [x] 5.3 Add `tests/dataTypes.spec.ts` modeled on `tests/macroFunctions.spec.ts`, reusing `tests/helpers.ts`, `tests/dashboardBuilder.ts`, and `tests/queryEditorRow.ts`
- [x] 5.4 Assert rendered cell text for populated UUID/IP values and empty cells for the NULL row; start every Grafana select locator from `[data-value=""]`
- [x] 5.5 Run `npm run build`, bring up the stack, and run the suite via the docker-compose `playwright` service

## 6. E2E coverage for ad-hoc filtering

Added after review: tasks 4.5 and 8.2 only ever checked the filter path by hand
(a container query and a manual look at the key picker). The runtime path —
Grafana filter state through `applyTemplateVariables`, over the wire, into
`$__adHocFilter()`, out as SQL ClickHouse must accept — had no automated
coverage at any layer. The e2e skill's own audit lists ad-hoc filters (#13) as an
open gap.

- [x] 6.1 Add a `v6_mapped IPv6` column to `e2e.datatypes` holding IPv4-mapped addresses, mirroring how Hydrolix stores its `ip` type
- [x] 6.2 Add `DashboardBuilder.addAdHocVariable()` so a dashboard can be provisioned with filters already applied, avoiding the three chained react-selects of the filter pill
- [x] 6.3 Add `tests/adHocFilterDataTypes.spec.ts` asserting exact row counts and a row fingerprint per filter case, read from the `/api/ds/query` response frames
- [x] 6.4 Cover `=` on `UUID` / `IPv4` / `IPv6`, `!=`, the multi-value `=|`, and the `__null__` sentinel on `Nullable(UUID)` / `Nullable(IPv6)`
- [x] 6.5 Cover the panel-to-filter round trip: `=` with the padded `::ffff:` text a panel displays matches an IPv4-mapped `IPv6` column, and a dotted-quad literal is still accepted because `=` parses the literal before comparing
- [x] 6.6 Pin the `=~` text-comparison boundary: it matches the padded form the panel displays and does NOT match a dotted-quad literal, since `toString(key) LIKE` carries no implicit wildcards
- [x] 6.7 Assert each filter actually reached the backend, so a dropped filter (macro falls back to `1=1`, every row returned) fails rather than passing as a wider result
- [x] 6.8 Record in `design.md` (D6, D7) that `isString` correctly excludes UUID/IP from the string NULL branch, and that the backend's `QueryKeys` never type-filtered
- [x] 6.9 Verify every expected count against ClickHouse directly, then run both `tests/adHocFilterDataTypes.spec.ts` (12 cases) and `tests/dataTypes.spec.ts` (3 cases) green

## 7. Align IP rendering with ClickHouse (reverses the original D5)

Raised in review: the original D5 rendered via Go's value-driven
`net.IP.String()`, so an IPv4-mapped address in an `IPv6` column displayed as
`1.2.3.4` while ClickHouse renders `::ffff:1.2.3.4`. Since Hydrolix stores IPv4
mapped, that was the *common* case, and it made a panel value silently
unmatchable under the `=~` / `!~` operators. Rendering is now type-driven, as
ClickHouse does it.

- [x] 7.1 Verify ClickHouse's rendering rule per column type, including `::ffff:0.0.0.0`, `::`, `::1`, and the deprecated `::1.2.3.4` form
- [x] 7.2 Verify what the driver hands over per column type — `IPv4` is 4 bytes, `IPv6` is 16 bytes, identical on native and HTTP
- [x] 7.3 Confirm `netip.AddrFrom16(...).String()` matches ClickHouse for every case except the deprecated IPv4-compatible form — that exception was accepted here and is reversed in 10.1, where it turned out to break the panel-to-filter round trip
- [x] 7.4 Split the renderer in two: `ipv4Text` (dotted-quad via `To4`) and `ipv6Text` (IPv6 form via `netip.AddrFrom16`)
- [x] 7.5 Turn `ipConverter` / `nullableIPConverter` into factories taking a renderer, so the registry pairs each type name with its renderer and the nullable variants cannot drift
- [x] 7.6 Return a nil value alongside the error on an unrenderable length, consistent with the mismatched-type branch
- [x] 7.7 Update `TestIPv6IPv4MappedAddress` to assert `::ffff:1.2.3.4`, and add `TestIPv4ColumnStaysDottedQuad` proving the same bytes render differently per column type
- [x] 7.8 Add `TestIPv6RenderingMatchesClickhouse` pinning the full rendering table, and `TestNullableIPv6IPv4MappedAddress` for the nullable path
- [x] 7.9 Add `TestIPv6RenderingMatchesServer` integration test that compares the frame value against the server's own `toString()` over both protocols, so the test cannot drift from ClickHouse
- [x] 7.10 Add an e2e case asserting `v6_mapped` renders with the `::ffff:` prefix in a panel
- [x] 7.11 Retarget the ad-hoc e2e cases: `=` and `=~` both match the padded form the panel now displays; a dotted-quad literal still matches under `=` and not under `=~`
- [x] 7.12 Rewrite D5, fold D7 into it as resolved, and record the accepted costs (verbosity on Hydrolix's IPv4-heavy columns; the Hydrolix doc sentence needs updating)
- [x] 7.13 Route the documentation correction out of this change. The Hydrolix **IP Data Type** page (<https://docs.hydrolix.io/docs/ip-data-type>, also at `/latest/self-managed/advanced-configuration/transforms-and-write-schema/ip-data-type/`) has a section "Display IPv4 Addresses in Dotted-Quad Notation" asserting that "Grafana automatically represents IPv4 in typical dotted-quad form". This change renders `IPv6` columns as `::ffff:a.b.c.d`, so that sentence needs correcting; the page's `toIPv4OrDefault(ip)` + `toString()` recipe stays valid as the way to render dotted-quad on demand. Outside this repo — tracked on HDX-12151, ships with the release

## 8. Wrap-up

- [x] 8.1 Run the full gate set: `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`
- [x] 8.2 Manually confirm in Grafana that a `SELECT * FROM e2e.datatypes` table panel renders UUID/IP values, NULL cells are empty, and the columns appear in the ad-hoc filter key picker
- [x] 8.3 Note in the PR description that IPv4-mapped addresses in `IPv6` columns render with the `::ffff:` prefix, matching ClickHouse's `toString()` — and that Hydrolix's documentation states the opposite and must be corrected (7.13)

## 9. Review follow-ups

Raised in the two-axis review of the PR. The rendering decision was confirmed
unchanged; these are artifact, typing, and coverage items only.

- [x] 9.1 Drop the `D4` / `D5` pointers from shipped source (`pkg/converters/converters.go`, `pkg/converters/converters_test.go`, `testdata/containers/initdb.sql`, `tests/adHocFilterDataTypes.spec.ts`) — changes get archived under a date prefix, so the reference goes stale; the comments already state the reasoning
- [x] 9.2 Replace the `any` annotations in `tests/adHocFilterDataTypes.spec.ts` with local `Wire*` interfaces describing the `/api/ds/query` shapes the assertions actually read
- [x] 9.3 Add the two missing negated-operator e2e cases — `!~` on `v6_mapped` and `!=|` on `v4_col`, both asserting one row identified by fingerprint — bringing `tests/adHocFilterDataTypes.spec.ts` to 14 cases; record the matching `spec.md` scenarios
- [x] 9.4 Verify the two new expected counts against ClickHouse directly before writing the cases
- [x] 9.5 Re-run the gate set and the e2e suite — typecheck, lint (0 errors), Jest 171/171, `go vet`, `go test -race ./...` all green; e2e 19/19 on Grafana 13.0.1 against a `down -v` rebuilt stack so `initdb.sql` reapplied. `golangci-lint` reports one pre-existing `staticcheck` issue at `pkg/plugin/driver.go:49` (`UserAgentFromContext` deprecated) — that file is not in this change's diff
- [x] 9.6 Route the pre-existing `__empty__` defect out of this change. `buildFilterCondition` (`pkg/plugin/macros_adhoc.go:263-268`) emits `(k = '' OR k = '__empty__')`, which ClickHouse rejects for every non-string column — `UUID` code 376, `IPv4` 675, `IPv6` 676, and `Int32` code 32. The `Int32` case has no UUID/IP involvement, so the defect predates this change; adding UUID/IP to `SUPPORTED_TYPES` only widens the set of columns that can reach the branch. The `__null__` branch twelve lines up (`:250-259`) guards on `isString` and falls through to a plain `IS NULL`; the `__empty__` branch has no equivalent guard. Out of scope here — being implemented on `feature/HDX-11854_adhoc-filter-values-timeout`, which already touches this file

## 10. Second-round review follow-ups

Two findings from `hjpnam` on the PR, both valid and both reproduced against a
live `clickhouse-server` before being acted on. One is a real rendering defect
in shipped code; the other is a test that could not fail.

- [x] 10.1 Render the deprecated IPv4-compatible form (`::a.b.c.d`, no `ffff`) with the dotted-quad tail ClickHouse produces. `netip` prints `::10.0.0.1` as `::a00:1`, which is text the server never emits, so a value copied from a panel cell matched no rows under `=~` / `!~` — `toString(v6) LIKE '::a00:1'` returns 0 where `v6 = '::a00:1'` returns 1. Add `v4Compatible` (first 12 bytes zero, bytes 12-13 not both zero) and branch on it in `ipv6Text`
- [x] 10.2 Correct the justification the earlier revision recorded for leaving this diverged. ClickHouse *can* store the form distinctly — `hex(toIPv6('::1.2.3.4'))` is `0…0.01020304` against `0…0.ffff.01020304` for the mapped form — and RFC 4291's deprecation governs use of the form, not how a server prints one it holds. Rewrite the D5 paragraph in `design.md` and the comment on `TestIPv6RenderingMatchesClickhouse`
- [x] 10.3 Extend `TestIPv6RenderingMatchesClickhouse` with the four IPv4-compatible cases and the three low-bits-set boundary cases (`::2`, `::100`, `::ffff`) that must stay hex
- [x] 10.4 Extend `TestIPv6RenderingMatchesServer` from one address to every branch of the renderer, driven by `ipv6RenderCases`. This is the test that cannot drift, and the gap survived the first round precisely because it held only the mapped address — a branch pinned solely by a hardcoded expectation is a branch with no server-side guard
- [x] 10.5 Reshape the `=|` e2e case so it can fail. Two literals both matching gave 2 rows with fingerprint `ROW1_UUID` on a 2-row fixture — identical to the `1=1` fallback on both assertions, so the case was blind to the one failure mode the file exists to catch. Use one present and one absent literal (`['1.2.3.4','9.9.9.9']` → 1 row, `ROW1_UUID`), mirroring the `!=|` case
- [x] 10.6 Add a `v6_compat IPv6` column to `e2e.datatypes` holding `::10.0.0.1` / `::20.0.0.2`. A new column rather than a new row, so no existing case's expected count moves
- [x] 10.7 Pin the copy-from-panel round trip for the IPv4-compatible form across two tests anchored to the same literal: `tests/dataTypes.spec.ts` asserts the panel renders `::10.0.0.1`, and `tests/adHocFilterDataTypes.spec.ts` filters `v6_compat` with `=~` against it. The ad-hoc half alone cannot catch a renderer regression — its literal is typed by the test, not read from the panel — so the rendering assertion is what closes the loop
- [x] 10.8 Verify both new e2e expectations against ClickHouse directly before writing them — `toString(toIPv6(...)) LIKE '::10.0.0.1'` → 1, `toIPv4(...) IN ('1.2.3.4','9.9.9.9')` → 1
- [x] 10.9 Un-archive the change: move it back out of `openspec/changes/archive/`, drop the `hdx-column-type-converters` capability from `openspec/specs/`, and re-archive once these tasks land. Re-archived under `2026-08-28-`, superseding the `2026-08-27-` directory the first archive created; the `hdx-column-type-converters` capability (10 requirements) is back in `openspec/specs/`. Task 7.13 — the Hydrolix documentation sentence, outside this repo — is the one box still open, as it was at the first archive
- [x] 10.10 Re-run the gate set and the e2e suite. The fixture gained a column, so the stack needs `docker compose down -v` for `initdb.sql` to reapply — typecheck, lint (0 errors, 6 pre-existing deprecation warnings), Jest 171/171, `go vet`, `go test -race ./...`, `openspec validate --all` (10/10) all green; e2e **52/52** on Grafana 13.0.1, first attempt, no retries, against a `down -v` rebuilt stack confirmed to carry the new `v6_compat` column. `golangci-lint` still reports the one pre-existing `staticcheck` issue at `pkg/plugin/driver.go:49`, which is not in this change's diff
- [x] 10.11 Red-check both new guards before trusting them: with the `v4Compatible` branch removed, `TestIPv6RenderingMatchesClickhouse` fails on four rows (`::102:304` for `::1.2.3.4`, `::a00:1` for `::10.0.0.1`, `::1:0` for `::0.1.0.0`, `::ffff:ffff` for `::255.255.255.255`) and `TestIPv6RenderingMatchesServer` fails on both protocols. Restored and re-run green
