## Why

Grafana Assistant cannot query Hydrolix. Its native SQL tools gate on exact
plugin-ID string equality in the closed-source frontend bundle — both at tool
registration and again at execution — so a live stack answers "find logs in
Hydrolix" with `Unsupported SQL datasource type: hydrolix-hydrolix-datasource`.
No plugin-side change passes those gates; only Grafana Labs can extend the
allowlists. Users on Grafana 13.x increasingly expect Assistant to work against
every datasource they have, and Hydrolix is conspicuously absent.

Two things make a workaround available now: Assistant supports custom remote MCP
servers, and Hydrolix already ships one (`mcp-hydrolix`, a separate repository).
What is missing is the glue between them — the context that tells Assistant what
the user is looking at, and the guidance that makes it write correct Hydrolix SQL.

## What Changes

- Add `@grafana/assistant` as a frontend dependency and integrate its context
  API — the one Assistant surface not keyed on plugin ID.
- QueryEditor gains an Assistant entry point and publishes page context: current
  SQL, resolved database and table, time range, column schema, and primary time
  column.
- The datasource query error path gains an "explain this error" action that
  hands the failing SQL and the cluster's error text to Assistant.
- Ship an Assistant Skill document carrying Hydrolix dialect rules and a macro
  cheat-sheet, with auto-approval for the read-only MCP tools.
- Ship operator documentation for registering `mcp-hydrolix` as an Assistant
  custom MCP server, including the identity and scope trade-offs.
- All Assistant UI degrades to nothing when the Assistant app is absent, so
  OSS Grafana installs are unaffected. Not breaking; no dashboard, query model,
  or backend behavior changes.
- Coverage: frontend unit tests for the context builders and availability
  gating, plus Playwright e2e for the QueryEditor entry point.

## Capabilities

### New Capabilities
- `hdx-assistant-context`: what the plugin publishes to Grafana Assistant —
  page context items, entry points, the error-explanation action, and the
  requirement that every surface is inert when Assistant is unavailable.
- `hdx-assistant-skill`: the Assistant Skill document contract — the dialect
  rules it must state, the macro translations it must carry, and which MCP
  tools it auto-approves.

### Modified Capabilities

None. No existing requirement changes; the query pipeline, interpolator, and
connection handling are untouched.

## Impact

- **Frontend**: `src/QueryEditor` (entry point and context publication),
  `src/datasource.ts` (error-path action alongside the existing `logError`),
  optionally `src/ConfigEditor`. New dependency `@grafana/assistant`, whose peer
  range must be checked against the repo's `@grafana/*` `^13.1.0`.
- **Backend**: none. Go gates should be unaffected.
- **External dependency**: `mcp-hydrolix` provides all query execution. It
  connects directly to the cluster and never calls the Grafana API, so Grafana
  datasource permissions and the plugin's `forwardOAuth` per-user identity do
  not apply on that path — an auth-model difference operators must understand
  before enabling it.
- **Deployment prerequisite**: Grafana must be able to reach the MCP server over
  the network. A Grafana Cloud stack fronting a private Hydrolix deployment may
  not satisfy this.
- **Docs**: new operator guide; README pointer.
