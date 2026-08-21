---
name: build-plugin
description: Use when building this Hydrolix Grafana datasource plugin — frontend webpack bundle (`dist/module.js*`) or Go backend binary (`dist/gpx_plugin_*`). Covers the split between host-built frontend and container-built Go, when each needs an explicit rebuild, watch vs one-shot, and the verification chain (typecheck / lint / unit tests). Apply for any task that edits `src/**`, `pkg/**`, `Magefile.go`, `webpack.config.ts`, or that needs to confirm `dist/` matches the working tree before running e2e or the dev stack.
version: 0.1.0
---

@../../../.config/AGENTS/skills/build-plugin.md
