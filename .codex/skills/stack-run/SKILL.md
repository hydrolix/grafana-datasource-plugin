---
name: stack-run
description: Use when starting, stopping, or inspecting the dev stack for this Hydrolix Grafana datasource plugin — Grafana (with the plugin bind-mounted), ClickHouse, Keycloak (OAuth), and the on-demand playwright runner. Covers `docker compose up` semantics, the `dist/` prerequisite, serving plugin artifacts from an alternate folder (any path, live or frozen, via an ephemeral compose override), switching Grafana versions, hot-reload behaviour, delve attach, log streaming, OAuth/anonymous auth, and ClickHouse fixture access. Apply whenever a task needs the plugin running locally, needs to reset the stack, or needs to debug why Grafana won't come up.
version: 0.2.0
---

@../../../.config/AGENTS/skills/stack-run.md
