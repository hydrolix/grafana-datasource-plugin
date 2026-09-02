# Registering Hydrolix with Grafana Assistant

How to give Grafana Assistant the ability to query a Hydrolix cluster, and what
you are agreeing to when you do.

Grafana Assistant cannot reach Hydrolix through its built-in SQL tools: those
match datasources against a hard-coded allowlist that does not include this
plugin. Assistant support comes instead from two independent pieces —
the [Hydrolix MCP server](https://github.com/hydrolix/mcp-hydrolix) registered
as a custom MCP server, and the Hydrolix skill installed into Assistant.

## What works with and without the MCP server

The plugin half needs no registration. Install or upgrade the plugin and the
query editor publishes context to Assistant — datasource, cluster host, SQL,
time range, and the resolved table's columns and primary time column — plus an
Assistant button and an "Explain error" action on failed queries.

| | Plugin only | Plugin + MCP server + skill |
| --- | --- | --- |
| Write and explain Hydrolix SQL | Yes | Yes |
| Sees your current query, schema, time range | Yes | Yes |
| Knows the Hydrolix dialect rules | Only with the skill installed | Yes |
| Discover databases and tables | No | Yes |
| **Execute a query and return rows** | **No** | **Yes** |

Without the MCP server, Assistant is a query-authoring helper, not an agent. It
will draft SQL you can paste into the editor; it cannot run anything. That is a
legitimate place to stop, and it involves none of the trade-offs below.

## Before you register: read this first

Registering the MCP server changes the effective auth model for Hydrolix data
in your Grafana stack. Three consequences, none of which have a mitigation
beyond deciding you accept them.

**Grafana datasource permissions stop applying.** Queries through the MCP
server do not pass through Grafana, so the datasource permissions that decide
who may query Hydrolix today are not consulted. Access is governed by the MCP
server's own registration scope and its Hydrolix credential.

**Per-user identity is lost.** If your datasource uses `forwardOAuth`, each
user's queries currently reach the cluster as that user. MCP queries instead
authenticate with the single Hydrolix credential the server holds. Per-user
attribution and any cluster-side per-user access rules do not survive the MCP
path. Grafana exposes no user-scoped token that would let this be fixed, so
treat it as a property of the feature rather than a bug to wait out.

**Scope decides the blast radius.** When registering, Grafana asks who the
integration is for:

- **Just me** — only your Assistant sessions can use it. Your Hydrolix
  credential, your queries. This is the higher-isolation option and the right
  default for evaluating the feature.
- **Everybody** — anyone in the org with Assistant access can query the
  cluster as the server's identity, *including users who cannot see the
  Hydrolix datasource in Grafana at all*. Choose this only when that is what
  you want, and pair it with a least-privilege Hydrolix service account rather
  than an administrator credential.

## Step 1 — verify reachability (go/no-go)

Grafana must be able to reach the MCP server over the network. This is the step
most likely to end the exercise, so do it first.

From an environment with the same network position as your Grafana instance:

```bash
curl -s https://<your-mcp-hydrolix-host>/mcp
```

Expect an **authentication failure** response. That is the success case: an
unauthenticated `curl` is not supposed to get further, and a reply of any kind
proves the endpoint is resolvable and reachable. What you are ruling out is a
DNS failure, a connection refused, or a timeout — any of those means stop here,
because nothing downstream will work.


## Step 2 — confirm it targets the same cluster

`mcp-hydrolix` is configured with its own `HYDROLIX_HOST`, chosen independently
of any Grafana datasource. Nothing enforces that the two agree, and a mismatch
produces confident answers about the wrong cluster — the worst available
failure mode.

Before registering, confirm the server's `HYDROLIX_HOST` matches the **Host**
field of the Hydrolix datasource your dashboards use. If you intentionally run
several clusters, see the naming convention in step 4.

## Step 3 — register the MCP server

In Grafana, go to **Assistant → Integrations → MCP servers**, add the server
with the URL you verified in step 1, and select the scope you decided on above.

Name it **`Hydrolix`**, or after the cluster it serves (`hydrolix-prod`, or the
cluster hostname). This is not cosmetic: the skill routes by this name. With
several Hydrolix clusters registered, a name matching the datasource host is
what lets Assistant pick the right server instead of guessing.

## Step 4 — install the skill

The skill is [`docs/assistant-skill.md`](./assistant-skill.md) in this
repository. It teaches Assistant the Hydrolix dialect rules — the time-filter
guard, primary keys from `system.tables`, untrustworthy nullability,
summary-table `-Merge` handling — without which generated queries fail or
silently return wrong results.

In Grafana, go to **Assistant → Skills** and create a new skill:

1. Paste the **entire contents** of `assistant-skill.md` as the skill body.
   Paste it verbatim — do not trim or reword it. Each rule exists because
   omitting it produces a specific failure, and the file is kept in step with
   the plugin.
2. In the **Allowed tools** section, click **Add tool** and add three tools
   from the Hydrolix integration you registered in step 3:
   - `list_databases`
   - `list_tables`
   - `get_table_info`
3. Leave `run_select_query` out — see below.
4. Save.

Those three are read-only discovery: they list what exists and describe it.
Auto-approving them is what makes Assistant usable, because otherwise every
schema lookup interrupts the user for an approval that is never interesting.

When you update the plugin, re-paste the skill body if `assistant-skill.md` has
changed; the dialect rules travel with it.

### Why `run_select_query` is not auto-approved

It runs with `readonly = 1` and cannot modify data, so the omission is not
about destructiveness. It is the tool that consumes cluster resources and
returns data to the model, so the approval decision belongs to you. Leaving it
out means each execution is subject to your deployment's approval setting.
Add it to **Allowed tools** if you have decided otherwise.

### Optional: provision as code

If you manage Grafana with Terraform, the same skill and tool allowlist can be
provisioned from the repository file, so it tracks dialect changes without a
re-paste:

```terraform
resource "grafana_assistant_skill" "hydrolix" {
  name  = "Querying Hydrolix"
  body  = file("${path.module}/assistant-skill.md")
  scope = "tenant" # or "user" to match a "Just me" registration

  dynamic "allowed_tools" {
    for_each = ["list_databases", "list_tables", "get_table_info"]
    content {
      integration_id = var.hydrolix_mcp_integration_id
      tool_name      = allowed_tools.value
    }
  }
}
```

`integration_id` is the UUID of the MCP integration from step 3, available from
its entry under **Assistant → Integrations**.

Skill content is capped at 64KB whichever route you use. `assistant-skill.md`
is currently well inside that; if you extend it, the API rejects an oversized
body with `skill content exceeds the maximum size`.

## Step 5 — verify

In a Grafana panel using the Hydrolix datasource, open Assistant and ask it to
list the tables in your database. A correct setup:

1. Runs `list_tables` **without prompting for approval** — if you are asked to
   approve, the `allowed_tools` configuration from step 4 did not take effect.
2. Returns tables from the cluster you expect.
3. When asked for data, produces SQL carrying an explicit time filter, and asks
   before executing it.

## Limitations

- Queries through the MCP server bypass Grafana datasource permissions.
- `forwardOAuth` per-user identity is not preserved; queries use the server's
  credential.
- Nothing enforces that the MCP server and the datasource target the same
  cluster; the skill cross-checks the `datasourceHost` from page context and
  flags a mismatch, but it can only do so when page context is present.
