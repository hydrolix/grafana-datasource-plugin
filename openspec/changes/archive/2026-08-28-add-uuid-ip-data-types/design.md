## Context

`pkg/converters/converters.go` holds a `map[string]Converter` keyed by ClickHouse
type name; each entry declares a `fieldType` (the `data.Frame` field type), a
`scanType` (the `database/sql` scan target), and an optional `convert` func that
defaults to `defaultConvert`. `Converters` flattens the map into
`[]sqlutil.Converter` at package init.

`sqlutil.MakeScanRow` matches a column's `DatabaseTypeName()` against each
converter's `InputTypeName`, then its `InputTypeRegex`. On no match it calls
`sqlutil.NewDefaultConverter`, which inspects the driver's scan type: for `UUID`
that is `uuid.UUID` (a `[16]byte` array) and for `IPv4`/`IPv6` it is `net.IP`
(a byte slice). Neither is a valid `data.Frame` field type, so the fallback
swaps in a `sql.NullString` scan target. `database/sql`'s `convertAssign` has no
path from an array or a named byte slice into `*string`, so the query fails with
`unsupported Scan, storing driver.Value type ... into type *string`.

The plugin queries through `database/sql`, not the ClickHouse native API, so the
driver's per-column `ScanRow` (which does understand `*string`) is bypassed:
`clickhouse_std.go` `stdRows.Next` fills `[]driver.Value` from
`column.Row(i, nullable)` and `convertAssign` does the rest. Any design has to
work with what lands in `driver.Value`.

Constraints: Grafana has no native UUID or IP field type; `sqlutil.FrameConverter`
documents an aliasing contract (the scan buffer is reused across rows, so a
converter must never return a pointer into its input); no new dependency should
be added.

## Goals / Non-Goals

**Goals:**
- `UUID`, `IPv4`, `IPv6` and their `Nullable(...)` forms return data instead of
  erroring, as string frame fields.
- SQL `NULL` in a nullable column reaches the frame as a nil `*string`, so
  Grafana renders an empty cell rather than a placeholder.
- UUID/IP columns become eligible ad-hoc filter keys.
- Coverage at three layers: Go unit, testcontainer integration over both native
  and HTTP protocols, and Playwright e2e.

**Non-Goals:**
- A dedicated Grafana field type or display hint for UUID/IP values. They are
  strings; existing string transformations and panel options apply.
- `LowCardinality(...)`, `Tuple(...)`, or other wrappers not already supported.
- IP-aware ad-hoc filter operators (CIDR containment, subnet matching).
- Changing how `Array(...)` / `Map(...)` columns are converted.

## Decisions

### D1. Render UUID and IP values as string frame fields

`data.FieldType` has no UUID or IP member, and adding a JSON field (as
`Array()`/`Map()` do) would quote the value and break string transformations,
sorting, and ad-hoc filter round-tripping. Strings are what every panel, filter,
and transformation already handles.

*Alternative considered:* map `IPv4` to `FieldTypeUint32` since the driver can
scan it as one. Rejected — the numeric form is unreadable in a panel and cannot
be fed back into an ad-hoc filter.

### D2. UUID reuses the existing `String` / `Nullable(String)` mechanism verbatim

`google/uuid`'s `UUID.Value()` implements `driver.Valuer` and returns the textual
form, and `stdRows.Next` resolves `driver.Valuer` before handing the value to
`database/sql`. A UUID column therefore already arrives as a `string`. Entries
for `UUID` and `Nullable(UUID)` reusing the `*string` / `**string` scan types and
`defaultConvert` are sufficient — no custom converter, no `google/uuid` import in
`pkg/converters`.

*Alternative considered:* scan into `*uuid.UUID`, which works because
`*uuid.UUID` implements `sql.Scanner`. Rejected — it adds a direct dependency and
a parse-then-reformat round trip to reach the same string.

### D3. IP columns scan as `net.IP` and are stringified by dedicated converters

`net.IP` is not a `driver.Valuer`, so the driver value is the raw slice.
`convertAssign` assigns it into a `*net.IP` target, and reaches `**net.IP` through
its `reflect.Pointer` recursion (which also yields a nil inner pointer for SQL
`NULL`). Two converter factories sit alongside the existing `jsonConverter` —
`ipConverter` for the non-nullable case, `nullableIPConverter` for the nullable
case, which returns `(*string)(nil)` on NULL and otherwise a freshly allocated
`*string`. The fresh allocation is what satisfies the `sqlutil.FrameConverter`
aliasing contract. Both return an error on an unexpected input type rather than
substituting a zero value, so a driver-behavior change surfaces as a failed query
instead of silently blank cells.

Each factory takes the renderer for its column type — `ipv4Text` or `ipv6Text`
(see D5) — so the registry states the type-to-rendering pairing at the point the
entry is declared, and the nullable and non-nullable variants of a type cannot
drift apart.

### D4. Match on exact type name; no regex

`converterMatches` tests `InputTypeName` before `InputTypeRegex`, and
`DatabaseTypeName()` yields exactly `UUID`, `IPv4`, `IPv6`, `Nullable(UUID)`,
`Nullable(IPv4)`, `Nullable(IPv6)`. No existing regex entry (`^Date\(?`,
`^Nullable\(Date\(?`, `^Nullable\(String`, `^Array\(.*\)`, `^Map\(.*\)`) matches
any of those names, so the registry's random map iteration order cannot produce a
nondeterministic match.

ClickHouse's `INET4` / `INET6` aliases need no separate entries: they are
case-insensitive aliases in `system.data_type_families`, and the server
canonicalizes them, so a column declared `INET4` or `inet4` is reported by
`DESCRIBE` — and therefore by `DatabaseTypeName()` — as `IPv4`. Verified against
`clickhouse-server:24.8`. There is no plain `IP` type family in ClickHouse
(`CREATE TABLE t (x IP)` fails with `Unknown data type family: IP`).

### D5. Rendering follows the column type, matching ClickHouse

An `IPv4` column renders dotted-quad; an `IPv6` column renders in IPv6 notation,
keeping the `::ffff:` prefix for an IPv4-mapped address. That is ClickHouse's own
rule — formatting is driven by the declared column type, not by the value:

| Column | Stored value      | ClickHouse `toString()` |
|--------|-------------------|-------------------------|
| `IPv4` | `1.2.3.4`         | `1.2.3.4`               |
| `IPv6` | `::ffff:1.2.3.4`  | `::ffff:1.2.3.4`        |
| `IPv6` | `2001:db8::1`     | `2001:db8::1`           |
| `IPv6` | `::`              | `::`                    |

Go's `net.IP.String()` is *value*-driven instead: handed the 16 bytes of an
IPv4-mapped address it prints `1.2.3.4`, discarding the fact that the column is
IPv6. So the IPv6 path cannot use it. `netip.AddrFrom16(...).String()` can —
`AddrFrom16` never unmaps, and its output matches the table above exactly.

This matters most on Hydrolix. Hydrolix's own `ip` column type exists only in the
transform / write schema; at query time it is a native ClickHouse IP type, and
IPv4 addresses are stored IPv4-mapped. So on Hydrolix the mapped case is the
*common* case, not an edge case, and value-driven rendering would silently
disagree with the server on the majority of rows.

Concretely, type-driven rendering buys three things:

1. **One text form per value, across every operator.** A value copied out of a
   panel cell works as an ad-hoc filter literal under both `=` (which parses the
   literal) and `=~` / `!~` (which compare `toString(...)` output). Under
   value-driven rendering the displayed `1.2.3.4` worked under `=` and silently
   matched nothing under `=~` — a wrong answer, not a cosmetic difference.
2. **Agreement with every other view of the same data** — `clickhouse-client`,
   the Hydrolix query UI, `toString()` in a user's own SQL, and any transformation
   or value mapping a user writes against panel text.
3. **No information loss.** `::ffff:1.2.3.4` says the value lives in an IPv6
   column; `1.2.3.4` does not.

*Costs, accepted:* on a Hydrolix cluster whose IP column is mostly IPv4, every
cell carries an `::ffff:` prefix, which is noisier than dotted-quad for the
dominant case. Users who prefer the short form can strip the prefix with a
standard string transformation — the reverse is not possible, because dotted-quad
has already discarded which family the column holds. Separately, the Hydrolix
**IP Data Type** page (<https://docs.hydrolix.io/docs/ip-data-type>) states that
"Grafana automatically represents IPv4 in typical dotted-quad form". That
sentence already contradicts the rest of its own page — which says the `ip` type
maps to `IPv6` at storage and query time and that "IPv4 addresses are returned in
IPv4-mapped IPv6 form" — and it contradicts this decision, so it should be
corrected alongside the release. The page's `toIPv4OrDefault(ip)` + `toString()`
recipe stays valid as the way to render dotted-quad on demand.

*Alternative considered:* keep dotted-quad and make `=~` consistent by rewriting
the backend's regex branch to parse IP literals. Rejected — it would break the
operator's documented "match against the rendered text" contract for every other
column type, and it cannot be done without either unmapping in SQL (lossy for
genuine IPv6 values) or duplicating Go's formatting rules in ClickHouse
expressions.

*Special-cased:* the deprecated IPv4-compatible form (`::1.2.3.4`, no `ffff`).
`netip.AddrFrom16(...).String()` prints `::102:304` where ClickHouse prints
`::1.2.3.4`, so `ipv6Text` renders the dotted-quad tail itself when the first 12
bytes are zero and bytes 12-13 are not both zero. That predicate is the
equivalent of ClickHouse's own `formatIPv6` branch (`best.base == 0 &&
best.len == 6`): with words 0-5 zero, a non-zero word 6 is exactly what pins the
best zero-run at length 6, which is why `::`, `::1`, `::2`, `::100` and `::ffff`
stay in hex on both sides.

An earlier revision left this divergence in place on the grounds that RFC 4291
deprecated the form and that ClickHouse could not store it distinctly. The second
half was wrong: `::1.2.3.4` stores as `0…0.01020304` and `::ffff:1.2.3.4` as
`0…0.ffff.01020304` — distinct bytes, distinct renderings. And the deprecation
governs whether the form should be *used*, not how the server prints one it
holds, whereas this decision's whole premise is that the plugin prints what the
server prints. Leaving it diverged broke that premise for real data: `=~` and
`!~` compare `toString(column)`, so a cell reading `::a00:1` was text the server
never emits, and copying it into a filter returned zero rows.

The renderer is therefore verified against the server rather than against a table
this repo wrote: `TestIPv6RenderingMatchesServer` round-trips every branch —
ordinary, mapped, IPv4-compatible, and the low-bits-set boundary — through
`toString()`. That is the test that cannot drift, and the reason this gap
survived the first round is that it held only the mapped address, so the one case
that disagreed was pinned solely by a hardcoded expectation.

### D6. Ad-hoc filter eligibility is driven by the frontend supported-type list

`getKeyMap` in `src/editor/metadataProvider.ts` filters DESCRIBE output against
`SUPPORTED_TYPES` and its derived `NULLABLE_TYPES` / `ARRAY_TYPES` / `MAP_TYPES`
in `src/constants.ts`. Adding the three base names to `SUPPORTED_TYPES` is the
whole change; the wrapper lists are derived from it.

`buildFilterCondition` in `pkg/plugin/macros_adhoc.go` emits `key = 'value'` for
scalar operators and already wraps `=~` / `!~` in `toString(...)`. ClickHouse
implicitly converts a string literal when comparing against UUID/IPv4/IPv6, so no
backend macro change is needed — verified against `clickhouse-server:24.8`, where
each of `uuid_col = '<uuid>'`, `v4_col = '1.2.3.4'`, and `v6_col = '2001:db8::1'`
returns the matching row.

Two details of that dispatch matter for the newly eligible keys, and both fall
out correctly without a code change:

- `isString` is `strings.Contains(lower, "string)") || lower == "string"`, so
  UUID/IP types are *not* strings to it. That is the behaviour we want: the
  `__null__` sentinel therefore takes the plain `IS NULL` / `IS NOT NULL` branch
  rather than the string branch, which would also emit `= '__null__'` — a
  literal no UUID or IP column can parse.
- The backend's own key set comes from `MetadataProvider.QueryKeys`, which keeps
  every column DESCRIBE returns without filtering by type. So `SUPPORTED_TYPES`
  gates only what the *picker offers*; the backend never had an eligibility gap
  to close. Worth stating explicitly, because a filter whose key misses the key
  set is skipped and the macro returns `1=1` — a silent widening that looks like
  success.

### D7. `=` and `=~` agree because rendering agrees — resolved by D5

`=` compares *parsed values*: ClickHouse turns the literal into an IPv6 value and
compares binary, so `v6_mapped = '1.2.3.4'` matches a stored `::ffff:1.2.3.4`.
`=~` / `!~` compare *rendered text* via `toString(key) LIKE '...'`.

Those two only agree if the plugin renders a value the same way `toString()`
does. Under the value-driven rendering originally chosen they did not: the
displayed `1.2.3.4` matched under `=` and returned nothing under `=~`, with no
error to indicate why. D5's type-driven rendering removes the divergence — the
text the panel shows is the text `toString()` produces, so both operator families
accept it.

A dotted-quad literal still does not match under `=~`, because `=~` is a text
comparison and the pattern carries no wildcards. That is now a straightforward
consequence of the operator's contract rather than a trap, since dotted-quad is
no longer what the panel displays. Both directions are pinned by e2e cases.

## Risks / Trade-offs

- [ClickHouse rejects `uuid_col = '<literal>'` or the IP equivalent, leaving
  ad-hoc filters broken for the newly eligible keys] → **Resolved.** All three
  comparisons return the matching row against `clickhouse-server:24.8`, including
  a dotted-quad literal against an IPv4-mapped `IPv6` column. No branch in
  `buildFilterCondition` is needed.
- [Every IPv4 address in a Hydrolix `IPv6` column now displays with an `::ffff:`
  prefix, which is noisier than dotted-quad for the dominant case] → Accepted per
  D5, and reversible by the user with a string transformation. Pinned by unit,
  integration, and e2e tests.
- [Hydrolix's documentation states Grafana renders IPv4 dotted-quad, which this
  change contradicts] → Real, and the one item requiring action outside this
  repo: the doc sentence needs updating alongside the release. No shipped
  release renders these columns at all, so no dashboard depends on the old
  behavior.
- [A future driver bump changes what `IPv6.row()` returns, or drops
  `uuid.UUID`'s `driver.Valuer`, silently degrading these columns] → The
  converters error on an unexpected input type instead of returning a zero value,
  and the testcontainer suite exercises both native and HTTP protocols, so a
  bump that breaks either surfaces as a red test.
- [`IPv6.row()` returns a slice into the column's internal block buffer, so the
  scanned `net.IP` aliases driver-owned memory] → Both converters stringify
  immediately and never retain the slice, and the returned `*string` is freshly
  allocated per row, so no frame value outlives the buffer.
- [A user copies a value from a panel cell and it fails to match under some
  operator] → **Resolved** by D5: the plugin renders exactly what `toString()`
  renders, so the copied text works under `=`, `!=`, `=|`, `=~`, and `!~`.
  Pinned by e2e cases covering `=` and `=~` against the displayed form.
- [Widening `SUPPORTED_TYPES` grows the derived `ARRAY_TYPES` / `MAP_TYPES` lists
  and the ad-hoc key picker] → Bounded growth (three base names), and the
  existing length assertion in `src/editor/metadataProvider.test.ts` keeps the
  derivation honest.
