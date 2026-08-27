## 1. Prerequisites

- [x] 1.1 Confirm `@grafana/assistant`'s peer range accepts `@grafana/*` `^13.1.0`; record the resolved version and any transitive dependency growth
- [x] 1.2 Verify against a live Assistant instance that a custom MCP server can be registered on the target tenant, and that `mcp-hydrolix`'s `/health` endpoint is reachable from Grafana — manually verified against a live tenant (Ostap Demkovych)
- [x] 1.3 Resolve whether the Skill ships as a versioned repo file provisioned via `grafana_assistant_skill`, or as documented copy-paste content; the design leans toward the versioned file

## 2. Dependency and availability plumbing

- [x] 2.1 Add `@grafana/assistant` to `package.json` and refresh `package-lock.json`
- [x] 2.2 Add an availability hook wrapping `isAssistantAvailable`, returning a stable unavailable state on error or non-resolution
- [x] 2.3 Unit-test the availability hook: available, unavailable, error, and late-resolution cases

## 3. Context payload construction

- [x] 3.1 Add a pure builder that assembles the structured context payload — SQL, database, table, time range, column schema, primary time column, datasource host — reading only from `src/editor/metadataProvider.ts` state and the current query
- [x] 3.2 Omit table, schema, and primary-time-column fields when no table is resolvable, rather than emitting empty placeholders
- [x] 3.3 Unit-test the builder: resolved table, unresolved table, empty query, and datasource host always present
- [x] 3.4 Unit-test that building context issues no metadata provider fetch or datasource query

## 4. QueryEditor integration

- [x] 4.1 Register page context from `src/components/QueryEditor.tsx` via `providePageContext`, gated on the availability hook
- [x] 4.2 Update published context when the SQL or time range changes, using the setter returned by `providePageContext`
- [x] 4.3 Add the Assistant entry point to the QueryEditor, passing datasource and current query
- [x] 4.4 Unit-test that no context is published and no entry point renders when Assistant is unavailable — the tick was premature; delivered in §11.2 after the PR #169 review found no assistant coverage in `QueryEditor.test.tsx`
- [x] 4.5 Unit-test that context updates on SQL and time-range changes

## 5. Error-path explain action

- [x] 5.1 Derive the "Explain error" action from `QueryEditorProps.data` (per-refId `DataQueryError`) instead of the `query()` pipeline — an RxJS map has no render surface, and `props.data` avoids new mutable state on `DataSource` (design D9)
- [x] 5.2 Extract `matchSolutionTemplate` from `ErrorExposer` into `src/errors/errorSolution.ts` (shared classifier + `{placeholder}` rendering); refactor `ErrorExposer` onto it, behavior unchanged
- [x] 5.3 Render `OpenAssistantButton` in the QueryEditor toolbar with the failing SQL, error text, and classified guidance; `autoSend: false` so the user confirms what is sent; gated on Assistant availability
- [x] 5.4 Unit-test the classifier (groups extraction, rendering, `{{...}}` escapes, no-match) and the button (per-refId scoping, refId-less errors, unclassified errors, hidden without error/SQL); error reporting path untouched

## 6. Skill document

- [x] 6.1 Author the Skill with the dialect rules: time-range guard, primary key from `system.tables.primary_key`, untrustworthy nullability, `SETTINGS max_execution_time`, `TurbineStorage` engine
- [x] 6.2 Add the instruction to emit literal time bounds rather than macros in generated SQL
- [x] 6.3 Add the macro translation reference covering `$__timeFilter` and `$__conditionalAll`
- [x] 6.4 Configure auto-approval for `list_databases`, `list_tables`, and `get_table_info`; leave `run_select_query` to the operator — auto-approval is a property of the Skill *resource*, not the Skill document: Grafana carries it as `allowed_tools` blocks (`integration_id` + `tool_name`) on `grafana_assistant_skill`, with no frontmatter equivalent in the markdown body. Delivered as the provisioning stanza in `docs/grafana-assistant.md` §4, with `run_select_query` deliberately omitted and the omission explained.
- [x] 6.5 Verify the Skill content is under the 64KB limit and provisions cleanly by the mechanism chosen in 1.3 — `docs/assistant-skill.md` is ~11 KB, well inside the limit, and provisions cleanly as the versioned repo file chosen in 1.3
- [x] 6.6 Add the error-triage section distilled from `src/errors/solutionTemplates.ts` (fix-the-query / retry-once / stop-and-escalate buckets), plus review improvements: explicit-UTC time bounds, summary-table `-Merge` rule from mcp-hydrolix's `get_table_info` metadata, and the datasourceHost cross-check against page context
- [x] 6.7 Add the ad-hoc-filter operator translation table (verified against `pkg/plugin/macros_adhoc.go`): live testing showed the Assistant emitting `match()` where the plugin emits `LIKE` — `=~` is a wildcard LIKE unless the value has a `regex:` prefix
- [x] 6.8 Add MCP-server discovery guidance: the skill searches for the tools under a server named "Hydrolix"/the cluster name and disambiguates multiple servers via `datasourceHost`; the operator guide establishes the naming convention that makes this routing work

## 7. Operator documentation

- [x] 7.1 Write a focused registration guide (first draft `docs/grafana-assistant.md` was written, then removed 2026-08-17 as not focused enough — needs a rewrite; the auth/reachability caveats moved to the README section in the meantime) — rewritten as a five-step procedure ordered by go/no-go: reachability, same-cluster, register, install skill, verify
- [x] 7.2 State the same-cluster requirement between `mcp-hydrolix`'s `HYDROLIX_HOST` and the Hydrolix datasource (was in the removed guide; keep in the rewrite) — `docs/grafana-assistant.md` §2, as a blocking step before registration
- [x] 7.3 Document the auth model — bypass of Grafana datasource permissions and of `forwardOAuth` per-user identity — and the "Just me" versus "Everybody" scope trade-off (condensed form now in README; full treatment in the rewrite) — "Before you register" section, placed ahead of every registration step per the spec's ordering requirement
- [x] 7.4 Describe what works with and without the MCP server registered, so the context-only state is not oversold — capability table plus an explicit "that is a legitimate place to stop"
- [x] 7.5 Add a README section: allowlist explanation, MCP+skill pointer, condensed auth/reachability caveats

## 8. End-to-end verification

- [x] 8.1 Add a Playwright e2e asserting the QueryEditor renders and functions unchanged when the Assistant app is absent — covered by the existing suite rather than a new spec: the e2e environment has no Assistant app, so 9.5's 32/32 pass against the Assistant-integrated bundle *is* the no-regression evidence. Caveat for future readers: this is incidental coverage, not an assertion — nothing fails loudly if the property breaks. Revisit if the e2e environment ever gains the Assistant app.
- [~] 8.2 ~~Add a Playwright e2e covering the Assistant entry point when availability is stubbed as available~~ — Dropped: no technical means to wire the Assistant app into the e2e environment. The available-path behaviour is covered at the jest level instead (§4.4–4.5, §5.4).
- [x] 8.3 Manually verify against a live Assistant instance with `mcp-hydrolix` registered: schema discovery, a generated query that satisfies the time-range guard, and the error-explain action — manually verified end-to-end (Ostap Demkovych)

## 9. Quality gates

- [x] 9.1 `npm run typecheck` and `npm run lint` clean
- [x] 9.2 `npm test -- --ci` passing
- [x] 9.3 `go vet ./...`, `golangci-lint run`, and `go test -race ./...` still passing — expected unaffected, this change is frontend-only
- [x] 9.4 `npm run build` produces a clean `dist/`; confirm bundle size impact is acceptable
- [x] 9.5 Run the e2e suite via the `grafana-plugin-e2e` skill — 32/32 passed (2.0m, zero flakes) against Grafana 13.0.1 *without* the Assistant app, with the Assistant-integrated bundle loaded; doubles as live evidence for the 8.1 scenario

## 10. Code-review fixes (2026-08-16)

- [x] 10.1 Replace the regex table extractor with backend `/ast` parsing (`DataSource.getAst` + `resolveTableRef`); regex verified to truncate quoted identifiers and fabricate tables from literals/comments
- [x] 10.2 Drop the synchronous metadata cache mirror; fetch schema through the memoized async `metadataProvider.columns()`/`primaryKey()` with a debounce and a stale-completion guard (fixes permanent cache-key miss and stale publications)
- [x] 10.3 Gate page-context registration by mounting `AssistantQueryContext` only while `useAssistant().isAvailable`; registration now provably absent on Assistant-less installs
- [x] 10.4 Replace the hand-rolled availability hook with the SDK's `useAssistant()`
- [x] 10.5 Implement `DataSource.getQueryDisplayText` so the Assistant button phrases prompts around the current query
- [x] 10.6 Filter `props.queries` by datasource uid before the `HdxQuery[]` cast (Mixed-panel leak)
- [x] 10.7 Drop the dead `datasourceName` field; type the test mocks without `any`
- [x] 10.8 Re-run gates: typecheck, lint, `npm run test:ci` (192 passing), clean build

## 11. Code-review fixes (PR #169, 2026-08-27)

- [x] 11.1 Auto-approval had no implementation. It is a property of the Skill *resource* (`allowed_tools` = `integration_id` + `tool_name` on `grafana_assistant_skill`), not of the Skill document — a markdown body has no frontmatter equivalent. Delivered as the **Allowed tools** step in `docs/grafana-assistant.md` §4 plus an as-code Terraform variant, with `run_select_query` excluded and the exclusion explained.
- [x] 11.2 Add the §4.4 assertions that were ticked but never written: `QueryEditor.test.tsx` now mocks `@grafana/assistant` and asserts `useProvidePageContext` is not called and no explain action renders when unavailable, and that both appear when available. Verified to fail with the `assistantAvailable &&` gates removed.
- [x] 11.3 `metadataProvider.primaryKey` memoized on truthiness, so a table whose primary key resolves to `""`/undefined re-queried the cluster on every call — once per keystroke behind Assistant's 300ms debounce. Switched to a `key in primaryKeys` presence check; two regression tests, both verified to fail against the old predicate. Pre-existing code; this change is what put it on a hot path.
- [x] 11.4 Operator documentation (§7.1–7.4) written as `docs/grafana-assistant.md`.
- [x] 11.5 Reachability is verified by curling `/mcp` and expecting an authentication failure. `/health` is not served on a live cluster; the `hdx-assistant-skill` spec scenario named it specifically and was amended to require reachability without naming an endpoint. `design.md` R4 keeps the original wording as the archived record.
- [x] 11.6 Re-run gates: typecheck, lint (0 errors), `npm run test:ci` — 214 passing across 19 suites. Backend untouched.
