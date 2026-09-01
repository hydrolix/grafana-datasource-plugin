## Context

Grafana Assistant's SQL tooling is closed-source and gates datasources on exact
plugin-ID equality in two independent places in the frontend bundle: a
`shouldRegister` predicate that decides tool registration, and an execution-time
adapter factory whose `switch (t.type)` ends in
`default: throw new Error("Unsupported SQL datasource type: " + t.type)`. A live
stack reproduces the second gate verbatim for `hydrolix-hydrolix-datasource`.
Neither consults `plugin.json`, a capability flag, or the `@grafana/sql`
contract, so no plugin-side change can pass them. The undocumented
`grafana-assistant-app/extension/v1` extension point exists but is gated behind
a Cloud-controlled flag that defaults off. Full evidence:
`.claude/findings/2026-08-11-grafana-assistant-datasource-gating.md`.

Assistant does support custom remote MCP servers (Settings → Integrations → MCP
servers → Add Custom Server) and Skills, both GA. Hydrolix already ships
`mcp-hydrolix`: FastMCP 2.14, HTTP/SSE transport with a `/health` endpoint, four
tools (`list_databases`, `list_tables`, `get_table_info`, `run_select_query`
with `readonly = 1` forced), authenticating by per-request
`Authorization: Bearer <service-account-token>`, `?token=` parameter, or
environment credentials. It reaches the cluster directly via `clickhouse-connect`
and never calls the Grafana API.

That leaves a specific gap. The MCP server can execute queries but knows nothing
about what the user is looking at in Grafana, and the model knows nothing about
Hydrolix's dialect quirks. This change closes both from the plugin side.

Dialect behavior below was verified against Hydrolix v25.8.23.1 (`cicd`
database) on 2026-08-12.

## Goals / Non-Goals

**Goals:**

- Assistant receives enough context from the plugin to write correct Hydrolix
  SQL and to call the MCP tools with the right database and table, without the
  user restating them.
- A user hitting a Hydrolix query error can get it explained and corrected in
  one action from the QueryEditor.
- Every Assistant surface is invisible and inert when the Assistant app is not
  installed or the user lacks access.
- Operators can register `mcp-hydrolix` with Assistant from documentation
  alone, understanding the identity model they are opting into.

**Non-Goals:**

- Changing `mcp-hydrolix` — it lives in a separate repository and already has
  the tool surface this change assumes.
- Any Go backend change. The query pipeline, interpolator, connection cache,
  and OAuth-keyed pooling are untouched.
- Passing Assistant's native SQL gates, or implementing the `@grafana/sql`
  `SqlDatasource` contract. Verified dead ends.
- Routing MCP traffic through Grafana's `/api/ds/query` so the plugin backend
  executes Assistant queries. Rejected below.
- Preserving per-user identity through the MCP path. Out of reach given how
  `mcp-hydrolix` authenticates; documented rather than solved.

## Decisions

### D1 — Integrate via `@grafana/assistant` context, not a query-tool shim

The plugin publishes context and entry points; it does not attempt to register
executable tools. Context injection is the only Assistant surface not keyed on
plugin ID, and the SDK is published, documented, and works today.

*Alternative considered:* the `grafana-assistant-app/extension/v1` extension
point, which does accept skill and prompt contributions from plugins. Rejected
because it is undocumented and gated behind `assistant.frontend.plugin-extensions`,
a Cloud-controlled flag defaulting to false — unusable without Grafana Labs
enabling it per tenant. Worth revisiting if that flag opens up.

### D2 — `mcp-hydrolix` talks to the cluster directly; no Grafana loopback

Query execution stays entirely inside `mcp-hydrolix` over `clickhouse-connect`.

*Alternative considered:* having the MCP server call back into Grafana
`/api/ds/query` with the datasource UID and a service account token, so the
plugin backend executes the query and macros, cluster credentials, and the TTL
connection cache are all reused. This is a real pattern — Grafana's own
`mcp-grafana` ClickHouse tool works exactly this way. Rejected for three
reasons: `mcp-hydrolix` is already built the other way and rebuilding it is out
of scope; the loopback requires the MCP server to hold a Grafana credential and
reach Grafana, which a cluster-side deployment may not be able to do; and the
loopback is permanently service-account-only, since Grafana will not mint a
user-scoped token to call back with, so it forecloses per-user identity rather
than enabling it.

### D3 — Structured context carries the schema the blocked SQL tool would have fetched

The SDK offers three context item types: dashboard, datasource, and structured
(arbitrary JSON). The structured item is where the substance goes — current SQL,
resolved database and table, time range, column schema from
`src/editor/metadataProvider.ts`, and the primary time column. The model never
needs to know the datasource is Hydrolix; it receives a schema and a set of
constraints and writes against them.

Including the datasource's configured host in this payload is deliberate: it is
the only signal available for detecting the cluster mismatch in R1.

### D4 — Context is resolved asynchronously through the backend parser and the memoized metadata fetchers

Publication is debounced (the same 300ms the interpolation preview already
uses) and runs fully off the editor's input path. The table is resolved by the
backend `/ast` resource — the real ClickHouse parser, already exposed and
otherwise unused by the frontend — never by pattern-matching raw SQL. Schema
comes from `metadataProvider.columns()`/`primaryKey()`, whose internal
memoization bounds the cost to one cluster round-trip per table per session. A
generation counter discards out-of-order completions.

*Alternative considered:* a synchronous read of the metadata provider's
internal cache, so publication issues no cluster queries at all. Implemented
first and rejected after review: the cache keys are written by plugin-ui's
autocomplete with its own identifier normalization, so a parallel key
derivation misses permanently for unqualified and quoted table names, and a
mutable cache is invisible to React's dependency tracking, so publications go
stale. Awaiting the fetchers removes both failure modes for the cost of one
bounded query per table, issued only while Assistant is available.

*Alternative considered:* a frontend regex over the raw SQL for table
extraction. Rejected after review: verified to truncate quoted identifiers,
fabricate tables from string literals and comments, and mistake comma joins
for single-table queries.

Verified live against the dev stack: `sqlds` opens the datasource connection
before dispatching any `callResource`, so `/ast` fails on an unreachable or
misconfigured cluster even though the parse itself issues no query. Table
resolution therefore degrades to basic context (SQL, host, range) exactly when
the editor is already non-functional for other reasons — `/interpolate` has
the same requirement — so this adds no new class of dependency.

### D5 — Availability gates by mount, using the SDK's own hook

The SDK's `useAssistant()` already exposes `{isLoading, isAvailable}` with the
right never-emits and error semantics, so the plugin does not wrap or
reimplement it. The QueryEditor mounts the context-publisher component only
while `isAvailable` is true; `useProvidePageContext` registers globally on
mount and unregisters on unmount, so mounting is the gate. An OSS Grafana
install with no Assistant app registers nothing and runs nothing extra.

### D6 — The Skill carries dialect rules the model cannot infer

Verified against v25.8.23.1: queries without a `WHERE` time filter are rejected
by `hdx_query_timerange_required`; `is_in_primary_key` and `is_in_sorting_key`
are false for every column in `system.columns`, so the primary key must be read
from `system.tables.primary_key`; `default_kind != 'DEFAULT'` makes every column
report as nullable, which is cosmetic misinformation to the model;
`SETTINGS max_execution_time = N` is accepted; the engine reports as
`TurbineStorage`. Each of these produces a wrong or failing query if the model
assumes stock ClickHouse.

The Skill also carries a macro cheat-sheet — `$__timeFilter(col)` to its
two-sided comparison, `$__conditionalAll` to its condition-or-`1=1` expansion
depending on the variable's selection — because `mcp-hydrolix`
performs no macro expansion, so a saved panel query pasted into Assistant is
otherwise unreadable to it.

Two additions distilled from elsewhere in the stack: an error-triage section
condensed from `src/errors/solutionTemplates.ts` (fix-the-query vs retry-once
vs stop-and-escalate — the split that keeps the model from rewriting SQL in
response to an S3 rate limit), and a summary-table rule sourced from
`mcp-hydrolix`'s `get_table_info` enrichment (aggregate columns hold partial
states and must be wrapped in their per-column `-Merge` functions). The
triage knowledge intentionally lives in both the Skill (covers chat and the
MCP self-correction loop) and the explain-error prompt (covers stacks where
no Skill is provisioned; deterministic where skill auto-discovery is
semantic) — the code is the source of truth if they drift.

### D7 — Auto-approve the read-only tools only

`list_databases`, `list_tables`, and `get_table_info` are auto-approved in the
Skill; `run_select_query` is left to per-deployment choice. It is `readonly = 1`
and therefore not destructive, but it is the tool that consumes cluster
resources and returns data, so the approval decision belongs to the operator.

### D8 — Explicit time bounds over macros in generated SQL

The Skill instructs the model to emit literal time bounds rather than macros.
Assistant generates fresh SQL and has the conversation's time range; macros
exist for panel templating and `mcp-hydrolix` cannot expand them anyway. The
cheat-sheet in D6 covers the reverse direction — reading existing queries.

### D9 — The explain-error action derives from panel data and grounds itself in the solution templates

The QueryEditor receives every failure for its own query through
`QueryEditorProps.data` — Grafana delivers the panel's latest response with
each `DataQueryError` carrying its `refId`. The action is therefore a pure
derivation: when `props.data` holds an error for this query's `refId` and
Assistant is available, an "Explain error" button renders in the toolbar.

The prompt is grounded in `src/errors/solutionTemplates.ts` — the ~50 curated
regexps with named capture groups and remediation text that the error-exposer
variable already matches against. A shared `matchSolutionTemplate` classifier
(extracted from `ErrorExposer`, behavior unchanged) contributes the template
name, the captured facts, and the rendered guidance to the prompt, so the
model applies a documented fix instead of diagnosing from raw text.
Classification runs against `error.data.message` first because `query()`
beautifies `error.message` before it reaches the panel. The prompt is opened
with `autoSend: false` so the user sees what leaves the editor — errors can
embed data fragments.

*Alternative considered:* recording a per-refId last-error map on the
`DataSource` instance from `query()`'s error path. Rejected: it adds mutable
per-instance request state — the exact pattern the `this.options` /
`this.filters` caches are already flagged for — and `props.data` makes it
unnecessary. The original task wording ("add the action alongside `logError`
in `query()`") was unimplementable as written: an RxJS pipeline has no render
surface.

## Risks / Trade-offs

**[R1] The MCP server and the Grafana datasource may point at different
clusters.** `mcp-hydrolix` is configured with its own `HYDROLIX_HOST`,
independent of any datasource. A user could get confident answers about a
cluster other than the one their panel queries. → Publish the datasource's
configured host in the structured context (D3) and state the same-cluster
requirement in the operator docs. Proving signal: a unit test asserting the host
appears in the built context payload; docs list host verification as a
registration step.

**[R2] RBAC divergence.** With Grafana out of the query path, Grafana
datasource permissions no longer apply. An "Everybody"-scoped MCP server lets
anyone with Assistant access query the cluster as whatever identity the server
holds — including users who cannot see the Hydrolix datasource in Grafana. This
is a genuine change to the effective auth model, not a detail. → Document
prominently, present "Just me" scope as the higher-isolation option, and
recommend a least-privilege Hydrolix service account. Proving signal: the
operator guide states the exposure explicitly before the registration steps.

**[R3] `forwardOAuth` per-user identity is not preserved.** Deployments using
`forwardOAuth` (`pkg/plugin/driver.go:154-157`, `:280-282`) get per-user
identity on the cluster today; MCP queries authenticate with the server's own
credential instead. → Document as a known limitation of the MCP path. Grafana
does not expose a user-scoped token that would let any design fix this, so
there is no mitigation beyond disclosure. Proving signal: named in the docs'
limitations section.

**[R4] Inbound reachability may make the feature impossible for some
deployments.** Grafana requires the MCP server to be "accessible from your
Grafana instance over the network". A Grafana Cloud stack fronting a private
Hydrolix deployment may not be able to satisfy this, and the docs say nothing
about private connectivity for MCP servers the way they do for datasources.
→ Make reachability the first verification step in the operator guide, using the
`/health` endpoint. Proving signal: `curl <mcp-url>/health` returning the
ClickHouse-compatible version string, documented as the go/no-go check.

**[R5] Context without MCP is a query-authoring helper, not an agent.** If an
operator installs the plugin but never registers the MCP server, Assistant can
write and explain Hydrolix SQL but cannot run anything. → State this plainly in
the docs and keep the entry-point copy modest, so the capability is not
oversold. Proving signal: docs describe both states separately.

**[R6] Assistant degrades past sixteen tools.** Grafana's guidance is one to
five tools for good behavior. `mcp-hydrolix` has four, but any future addition
eats the margin, and Assistant users typically have other servers registered
too. → Record the constraint here so future tool additions to `mcp-hydrolix` are
a deliberate trade rather than an accident.

**[R7] SDK version coupling.** `@grafana/assistant` is a new dependency against
a repo pinned to `@grafana/*` `^13.1.0`, and it ships into Grafana's process.
→ Verify the peer range before committing and confirm the bundle builds clean;
the availability gating in D5 contains the blast radius if the app is missing.
Proving signal: `npm run build` produces a clean `dist/`, and unit tests cover
the unavailable path.

## Migration Plan

Additive and non-breaking: no dashboard, query-model, provisioning, or backend
change. Rollout is a normal plugin release plus a separate, optional operator
step to register the MCP server and Skill. The two halves are independent — the
plugin ships whether or not any given stack registers the server, and the server
works (without page context) whether or not the plugin is upgraded.

Rollback is a plugin downgrade; nothing persists in dashboards or datasource
settings. Removing the MCP server registration in Assistant reverts the auth
exposure in R2 immediately.

## Open Questions

- Does `@grafana/assistant`'s peer range accept `@grafana/*` `^13.1.0`, and does
  it pull transitive dependencies that affect bundle size? Resolve before the
  dependency lands.
- Is Assistant's custom-MCP-server feature available on all tenants and plans,
  or gated? Unverified without a live Assistant instance.
- Can a Grafana Cloud stack reach a private MCP endpoint by any supported
  mechanism (R4)? If not, the feature is on-prem and public-endpoint only, which
  should be stated up front in the docs.
- Should the Skill ship as a versioned file in this repo for Terraform
  provisioning via `grafana_assistant_skill`, or as copy-paste content in the
  docs? Leaning toward a versioned file so it can track dialect changes.
