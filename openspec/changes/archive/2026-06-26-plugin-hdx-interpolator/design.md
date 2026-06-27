## Context

`sqlds.Interpolator` at `ef925e1`:

```go
type Interpolator interface {
    Interpolate(ctx context.Context, ds *SQLDatasource, query *sqlutil.Query, rawJSON json.RawMessage) (string, error)
}
```

A nil field resolves to `DefaultInterpolator{}`, which delegates to `sqlutil.Interpolate(query, ds.driver().Macros())`. This is byte-for-byte the pre-extension upstream behaviour — fine for plugins that only use the legacy regex-based `sqlutil.MacroFunc` macros.

The Hydrolix plugin's macros need more than that. The fork at `0f83082` (`interpolator.go`, 328 LOC + `macros.go`, 456 LOC) does three things `DefaultInterpolator` can't:

1. **Parses `RawSQL` into a ClickHouse AST** via `github.com/hydrolix/clickhouse-sql-parser`. Macros operate at AST positions (`parser.Pos`) so they can reason about lexical scope (e.g., is this `$__timeFilter` inside a CTE? Which CTE? What table?).
2. **Dispatches macros with positional context.** The fork's `MacroFunc` signature is `func(ctx, *HDXQuery, []string, parser.Pos, *MetaDataProvider) (string, error)` — distinct from `sqlutil.MacroFunc` which takes only `(*sqlutil.Query, []string) (string, error)`. The richer signature gives macros access to the AST position (for CTE-scoped table lookups), the metadata provider (for primary-key and column-type lookups), and the parsed `HdxQuery` (for typed access to user-supplied fields).
3. **Runs a post-rewrite mutation pass.** After macro expansion, the AST is walked again to apply Hydrolix-specific rewrites (table-reference fixups, CTE inlining). The fork called this `MutateInterpolatedQuery` in earlier drafts and folded it into the interpolator at `0f83082`.

`/macro-ctes` (the HTTP resource at `pkg/api/routes.go:85-117`) calls `sqlds.GetMacroCTEs(expr)` to extract the CTE map for the dashboard's macro-expansion preview. `CTE` is a public struct; `GetMacroCTEs` is a public function. Both disappear from `sqlds` at `ef925e1`; the plugin owns them now.

## Goals / Non-Goals

**Goals:**
- Implement `sqlds.Interpolator` in the plugin (`HdxInterpolator`) with the fork's AST passes preserved byte-for-byte (test corpus mirrors the fork's).
- Own the `Macros map[string]MacroFunc` registry inside the plugin so C6 and C7 can populate macros without modifying this change.
- Move `GetMacroCTEs` + `CTE` into the plugin so `pkg/api/routes.go:85-117` compiles against the new sqlds pin.
- Wire `ds.Interpolator = NewHdxInterpolator(...)` in `HdxSqlDatasource`.

**Non-Goals:**
- Defining or registering any individual macro. C6 (ClickHouse time/date) and C7 (ad-hoc filter + metadata provider) cover those.
- Reworking the AST visitor. The AST passes move verbatim modulo the new `sqlds.Interpolator` signature.
- Adopting `?`-style driver-level parameter binding instead of in-SQL macro expansion. Out of scope.
- Replacing `hydrolix/clickhouse-sql-parser` with a different ClickHouse SQL parser. The fork's choice; staying matches behaviour and tests.

## Decisions

### D1. `HdxInterpolator` is a concrete struct; constructor takes `(md, macros)`

```go
// pkg/plugin/interpolator.go
type HdxInterpolator struct {
    md     *MetadataProvider
    macros map[string]MacroFunc
}

func NewHdxInterpolator(md *MetadataProvider, macros map[string]MacroFunc) *HdxInterpolator {
    return &HdxInterpolator{md: md, macros: macros}
}

func (i *HdxInterpolator) Interpolate(
    ctx context.Context,
    ds *sqlds.SQLDatasource,
    query *sqlutil.Query,
    rawJSON json.RawMessage,
) (string, error) {
    hdxQuery := models.NewHdxQuery(query, rawJSON) // wraps query, parses JSON-only fields
    return i.interpolate(ctx, hdxQuery)
}

func (i *HdxInterpolator) interpolate(ctx context.Context, q *models.HdxQuery) (string, error) {
    // 1. Parse RawSQL into AST via clickhouse-sql-parser
    // 2. For each registered macro name, find call sites in the AST,
    //    dispatch the macro with (ctx, q, args, pos, i.md)
    // 3. Replace each call site's bytes with the macro's return value
    // 4. Walk the rewritten AST and apply post-rewrite mutations
    // 5. Return the final SQL string
}
```

**Why concrete struct, not interface.** `sqlds.Interpolator` is the interface; `HdxInterpolator` is the concrete implementation. The wrapper holds the concrete type via `ds.Interpolator = NewHdxInterpolator(...)` (assignment to the interface field). Tests can construct `HdxInterpolator` directly with a fake `MetadataProvider`.

**Why constructor takes the registry rather than reading a package-level var.** Constructor injection makes the interpolator testable without monkey-patching globals. The wiring site in `hdx_sqlds.go` passes the package-level `Macros` registry; tests can pass a fixture map. The package-level `Macros` exists so C6 and C7 have somewhere to add entries without referring back to the wrapper.

**Why `*MetadataProvider` and not an interface.** C7 defines `MetadataProvider` as a concrete struct. The interpolator holds a pointer to it. An interface (`MetadataLookup` or similar) would be more decoupled, but `MetadataProvider` is the only consumer and the only implementation; an interface buys nothing here. Re-evaluate if a second implementation surfaces.

**Why `Interpolate` parses `RawSQL` into an `HdxQuery` rather than taking `*models.HdxQuery` directly.** The `sqlds.Interpolator` interface signature is fixed — it takes `*sqlutil.Query` and `json.RawMessage`. The plugin's internal `interpolate` method takes the richer `*models.HdxQuery`; `Interpolate` is the SDK-shape adapter. `models.NewHdxQuery(query, rawJSON)` constructs the wrapper without copying SQL — it stores references.

### D2. `MacroFunc` signature matches the fork's

```go
// pkg/plugin/macros_registry.go
type MacroFunc func(
    ctx context.Context,
    query *models.HdxQuery,
    args []string,
    pos parser.Pos,
    md *MetadataProvider,
) (string, error)

// Macros is the package-level registry. C6 and C7 add entries; the
// interpolator reads from it via NewHdxInterpolator(md, Macros).
var Macros = map[string]MacroFunc{}
```

**Why match the fork's signature.** The macro implementations C6 and C7 port from the fork; matching the signature lets the port be mechanical. Macros that don't use `pos` or `md` ignore the parameters (Go's `_` convention works).

**Why a package-level map rather than per-instance.** Macros are stateless and identical across datasource instances. A per-instance map would require every instance to populate it; a package-level map populated via `init()` in C6 and C7 is simpler.

**Why `parser.Pos` from `clickhouse-sql-parser`, not `sqlds.MacroContext` or similar.** sqlds at `ef925e1` does not expose a context type for richer macro signatures (it was removed in `80cd2b4`). The plugin owns its dispatch surface entirely — it picks whatever type carries the right information. `parser.Pos` is already part of every AST node; no extra type needed.

### D3. `Macros` map mutation discipline: write at `init()`, read at runtime

C6 and C7 add to `Macros` via `init()` functions in their respective files:

```go
// pkg/plugin/macros_clickhouse.go (C6)
func init() {
    Macros["fromTimeFilter"] = FromTimeFilter
    Macros["toTimeFilter"] = ToTimeFilter
    // …
}

// pkg/plugin/macros_adhoc.go (C7)
func init() {
    Macros["adHocFilter"] = AdHocFilterMacro
}
```

After `init()` runs, `Macros` is read-only. The interpolator reads it (via the constructor-injected reference) without synchronisation.

**Why `init()` rather than explicit `RegisterMacro`.** sqlds's `RegisterMacro` was removed in `80cd2b4`. The plugin owns the registry; Go's `init()` is the simplest way to populate a package-level map at startup. No public registration API needed.

**Why no concurrency primitive on `Macros`.** Reads-after-init-only is the implicit Go convention for package-level data. Race detector under `go test -race ./...` catches any future code that violates this.

**Risk to flag**: a future change that mutates `Macros` at runtime (e.g., per-datasource macro registration) would need a mutex. Document the constraint in `macros_registry.go`'s package comment.

### D4. `pkg/plugin/cte.go` owns `CTE` and `GetMacroCTEs`

```go
// pkg/plugin/cte.go
package plugin

import "github.com/hydrolix/clickhouse-sql-parser/parser"

type CTE struct {
    // fields match the fork's CTE struct verbatim
}

func GetMacroCTEs(expr parser.Stmts) (map[string]CTE, error) {
    // implementation moves verbatim from sqlds@v5.0.1's interpolator.go
}
```

`pkg/api/routes.go:105` switches from `sqlds.GetMacroCTEs(expr)` to `plugin.GetMacroCTEs(expr)`; `pkg/api/routes.go:112`'s `Response[[]sqlds.CTE]` switches to `Response[[]plugin.CTE]`.

**Why a separate file.** `cte.go` is a small public surface (one type, one function) used by `pkg/api/`. Co-locating it with `interpolator.go` would mix the AST-rewrite pipeline (large, complex) with the CTE-extraction utility (small, simple). Separate files give the cross-package consumer a clean import target.

**Why expose `CTE` as a public type.** `pkg/api/routes.go` consumes it across package boundaries. Public is required.

### D5. Post-rewrite mutation runs after macro expansion, before return

Within `Interpolate`, the sequence is:

1. `parser.NewParser(query.RawSQL).ParseStmts()` → AST.
2. For each macro name in `i.macros`, find matches via `getMacroMatches(rawSQL, name, positions)`; dispatch each match's `MacroFunc`; record the byte-range replacement.
3. Apply byte-range replacements in reverse order (so earlier positions don't invalidate later positions). Result: macro-expanded SQL string.
4. Re-parse the expanded SQL into an AST and apply post-rewrite mutations (table-reference fixups, CTE inlining).
5. Return the final SQL string.

**Why re-parse after macro expansion.** Macro outputs can change the AST shape (a macro might emit `FROM t1 JOIN t2` where the original had just `FROM $__timeFilter`). The post-rewrite pass needs to see the post-expansion shape, not the pre-expansion shape.

**Why byte-range replacement rather than AST node substitution.** The fork's approach. Macro outputs are arbitrary SQL strings, not AST fragments; treating them as opaque text is simpler than parsing each output and splicing nodes. The re-parse step (step 4) catches any malformed output.

**Why reverse-order replacement.** Earlier positions' byte ranges anchor later positions. Replacing earlier ranges first shifts later byte offsets; reverse order avoids the bookkeeping.

### D6. `Interpolate` is safe for concurrent use across queries

`HdxInterpolator` has no mutable state. `md` is read-only; `macros` is read-only after init. Macros themselves are pure functions of their inputs (C6 and C7's implementations are stateless). Concurrent `Interpolate` calls on the same `*HdxInterpolator` are safe.

**Why this matters.** `sqlds.Interpolator` documents the contract: "Implementations MUST be safe for concurrent use across queries." The plugin's interpolator satisfies it without any locking.

**Test**: run `go test -race ./pkg/plugin/...` with a concurrent fixture that calls `Interpolate` from many goroutines on shared `HdxInterpolator`. Race detector catches any future code that adds mutable state.

### D7. CTE-extraction helper exposed as a plain function, not on `HdxInterpolator`

`GetMacroCTEs(expr parser.Stmts)` is a package-level function, not a method on `HdxInterpolator`. The HTTP resource at `pkg/api/routes.go:85-117` calls it with a parsed AST it built itself; the interpolator's state (macro registry, metadata provider) is irrelevant.

**Why a plain function.** Statelessness is the right shape. Adding it as a method on `HdxInterpolator` would push the resource handler to construct or look up the interpolator instance, which it doesn't need.

**Why named `GetMacroCTEs` (Go-stylistically would prefer `MacroCTEs`).** Matches the fork's name byte-for-byte. The resource handler at `routes.go` already calls `sqlds.GetMacroCTEs(expr)`; matching the name keeps the rename mechanical. Renaming to `MacroCTEs` is a future cleanup.

## Risks / Trade-offs

- **[AST visitor behaviour drift between fork and migrated `HdxInterpolator`]** → Mitigation: ported golden-output unit tests over the fork's existing test corpus (`interpolator_test.go`, 463 LOC). Any diff is a regression. Run the ported test suite against the migrated code; fail loudly on any byte difference.
- **[`Macros` registry race if a future change mutates it at runtime]** → Mitigation: package comment on `macros_registry.go` documents read-only-after-init; `go test -race` catches violations. If a deployment surfaces a need for dynamic registration, swap the map to `sync.Map` then.
- **[`clickhouse-sql-parser` parses fail on edge-case SQL the fork accepted]** → Mitigation: ported tests include the fork's full corpus, which exercises the edge cases. Any parse failure on previously-working SQL fails a test. Parser version pin in `go.mod` matches the fork's pin (verified during the C5 PR).
- **[Re-parse step (D5 step 4) fails on macro-expanded SQL]** → Acceptable: if a macro emits malformed SQL, that's a macro bug, not an interpolator bug. The error surfaces with `"failed to re-parse after macro expansion: …"` which names the failing macro from the call site that produced the output. Unit test covers the failure mode.
- **[`init()`-time macro registration leaks across test binaries]** → Acceptable: every Go test binary runs `init()` afresh. The shared `Macros` map is a per-binary global, not a cross-binary global. Tests that need a clean registry can use `t.Cleanup` to restore the map; the test in `interpolator_test.go` does this for fixture-only macros.
- **[`*MetadataProvider` is nil until C7 lands; ad-hoc filter macro from C7 dispatches against a nil pointer]** → Mitigation: C5's wiring passes a nil-safe sentinel value to `NewHdxInterpolator(md, Macros)` until C7 lands. C6's macros (time/date) ignore `md`. When C7 lands, the sentinel becomes the real `NewMetadataProvider(ds)`. The merge window ensures C7's macro never dispatches against a nil sentinel in production.

## Migration Plan

- **Forward**: ships in the C2-C7 coordinated merge window. Sequence inside its PR commit (or PR if stacked):
  1. Add `pkg/plugin/macros_registry.go` with `MacroFunc` and the empty `Macros` map. Package-comment documents read-only-after-init.
  2. Add `pkg/plugin/cte.go` with `CTE` and `GetMacroCTEs` ported verbatim from the fork.
  3. Add `pkg/plugin/interpolator.go` with `HdxInterpolator`. Implement `Interpolate` per D1–D6.
  4. Add `pkg/plugin/interpolator_test.go` and `pkg/plugin/cte_test.go`, ports of the fork's tests modulo type renames.
  5. Update `pkg/plugin/hdx_sqlds.go` (from C2) to assign `ds.Interpolator = NewHdxInterpolator(md, Macros)`. `md` is a nil-safe sentinel until C7's `MetadataProvider` exists.
  6. Update `pkg/api/routes.go:105,112` to call `plugin.GetMacroCTEs` and use `plugin.CTE`.
  7. `go mod tidy` (promotes `clickhouse-sql-parser` to direct).
  8. Run quality gates: `npm run typecheck`, `npm run lint`, `npm test -- --ci`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`.
- **Rollback**: revert this change's commit/PR. `ds.Interpolator` field falls back to `DefaultInterpolator`, which dispatches only legacy `sqlutil.MacroFunc` macros (none registered) — `pkg/api/routes.go` won't compile against missing `plugin.GetMacroCTEs`. Rollback requires reverting C2 too.
- **Sequencing**: depends on `pin-sqlds-extension-revision` (C2). Independent of C3 (cache) and C4 (OAuth) at the file level; ships in the same coordinated merge window. C6 and C7 each `init()` macros into the registry this change establishes.

## Open Questions

- Should `HdxInterpolator.Interpolate` also call back into the macro registry via `sqlds.DefaultInterpolator` for legacy `sqlutil.MacroFunc` macros registered via `Driver.Macros()`? Defer — the plugin currently has no `sqlutil.MacroFunc` macros (everything is the richer `MacroFunc`). If a future macro is best expressed as `sqlutil.MacroFunc`, add a delegation pass; otherwise the simpler single-dispatch is preferable.
- Should `parser.Pos` references in `MacroFunc` be wrapped in a plugin-local position type so the parser dependency stays in the interpolator only? Defer — the macros use `parser.Pos` directly (C6's time-bucket macros need it; C7's ad-hoc filter needs it). Wrapping adds a type with no clear win.
- Should the post-rewrite mutation pass be opt-in or always-on? Always-on matches the fork; always-on is correct for every query shape the plugin sees. Re-evaluate only if a deployment surfaces a need for opt-out.
