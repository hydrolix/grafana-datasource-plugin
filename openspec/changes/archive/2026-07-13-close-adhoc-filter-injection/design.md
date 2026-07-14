## Context

`$__adHocFilter` expands per-filter conditions in `pkg/plugin/macros_adhoc.go`. Filters arrive as raw JSON on `/api/ds/query`, so `filter.Key`, `filter.Operator`, and `filter.Value` are untrusted — the UI operator dropdown is not a server-side control. PR #152 routed every *value* through `escape()` and dropped `$$` dollar-quoting. Two inputs still reach SQL verbatim:

- **Operator** — the `default` branch at `macros_adhoc.go:220` emits `fmt.Sprintf("%s %s '%s'", key, operator, escape(value))`. Any operator string is accepted, so `= 'x' OR 1=1 -- ` comments out the rest of the `WHERE` clause.
- **Map key subscript** — `mapTypeFilterKey` (`:29`, `^(.*)\['.*']$`) extracts the base column, which is schema-validated at `:116`; but the full `filter.Key` (including `['...']`) is what flows into every `key` sink. `col['a'] OR 1=1 --']` validates on base `col` yet injects via the subscript.

The array path (`buildArrayCondition`, `:226-250`) already models the correct posture: a `switch` over a fixed operator set with a `default` that returns an error. This change brings the scalar/map path to the same posture and fixes the key construction.

## Goals / Non-Goals

**Goals:**
- Reject any operator not on an explicit allowlist on the scalar/map path, with a typed, wrapped error.
- Ensure the map key reaching SQL is composed only of a schema-validated column identifier and an escaped subscript literal — never raw `filter.Key`.
- Preserve identical output for all honest inputs (existing scenarios in `hdx-adhoc-filter-macro-secure` keep passing).

**Non-Goals:**
- Metadata-query identifier quoting (`QueryPK`/`QueryKeys`) and the `params[0]` explicit-arg check — owned by `secure-metadata-identifiers`.
- Frontend validation. The server is the trust boundary; adding a UI check would not close the raw-JSON vector and would duplicate logic.
- Changing the wire format of `AdHocFilter` or the set of operators the UI offers.

## Decisions

**D1 — Allowlist operators, don't blocklist.** Define the permitted scalar operators (`=`, `!=`, `<`, `<=`, `>`, `>=`, plus the already-cased `=|`, `!=|`, `=~`, `!~`) and validate before the `default` interpolation. The `default` branch becomes an error return, mirroring `buildArrayCondition:248`. Rationale: an allowlist fails closed — new/unknown operators are rejected rather than silently passed. A blocklist of injection strings is unbounded and fails open.
- _Alternative considered:_ escape/neutralize the operator string. Rejected — an operator is not a literal; there is no escaping that makes an arbitrary operator safe, and comparison operators are a small fixed set.

**D2 — Reuse the array path's error shape.** Return `fmt.Errorf("operator %q unsupported for scalar value", operator)` (wrapped, `%w` where a sentinel is warranted) so `AdHocFilterMacro`'s existing `err != nil` handling at `:119-122` surfaces it as a filter-construction error. Rationale: one error contract across both paths; no new control flow.

**D3 — Rebuild the map key from trusted pieces.** When `mapTypeFilterKey` matches, extract the base column (validated against `keyNames` as today) and the subscript, then emit `` `col`['<escape(subscript)>'] `` — backtick-quote the column, single-quote-escape the subscript via the existing `escape()`. The raw `filter.Key` is never used as the SQL `key` for map columns. Non-map keys continue to use the validated column identifier. Rationale: the subscript is a string literal in ClickHouse map access, so `escape()` is the correct tool; the column is an identifier, so backtick-quoting is correct.
- _Alternative considered:_ regex-validate the subscript and pass it through. Rejected — validation without quoting still trusts the exact bytes; quoting is the actual boundary.

**D4 — Regex tightening is validation, not the boundary.** `mapTypeFilterKey` may be made stricter for clarity, but security rests on quoting/escaping (D3), not on the regex. Rationale: regex-only defenses are brittle against edge cases (nested quotes, unicode).

## Risks / Trade-offs

- **[A honest operator is missing from the allowlist and gets rejected]** → Enumerate the allowlist directly from the operators the frontend emits and the existing `switch` cases; add a unit test asserting every currently-supported operator produces a condition (no error). Proving signal: table test over the full operator set in `macros_adhoc_test.go`.
- **[Backtick-quoting a map column changes emitted SQL for honest map filters]** → This is an intended, ClickHouse-valid change; add a scenario asserting the exact quoted form so the diff is reviewed and pinned. Proving signal: golden-string assertion for a map filter.
- **[Escaping the subscript double-escapes an already-safe key]** → `escape()` is idempotent-safe for literals but is applied exactly once to the extracted subscript; a round-trip test asserts `'`+`escape(sub)`+`'` recovers the input. Proving signal: reuse of the existing `TestEscape_QuotedRoundTrip` style over subscripts.
