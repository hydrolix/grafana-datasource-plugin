---
name: validate-plugin
description: Use when validating a packaged build of this Hydrolix Grafana datasource plugin against Grafana's official plugin-validator — the pre-release gate that catches missing metadata, unsigned/mis-signed archives, non-executable backend binaries, and packaging errors. Builds the plugin, zips `dist/` under the plugin ID, and runs `@grafana/plugin-validator` (npx, or docker as fallback). Apply for "validate the plugin", "is this zip release-ready", or before publishing / submitting a release artifact.
version: 0.1.0
---

@../../../.config/AGENTS/skills/validate-plugin.md
