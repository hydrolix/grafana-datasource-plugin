---
name: e2e-run-zip
description: Use when running the dev Grafana stack and/or the Playwright e2e suite against a *packaged* plugin zip (a release/RC artifact like `hydrolix-hydrolix-datasource-<ver>.zip`) rather than the working-tree `dist/`. Unpacks the package into a timestamped `dist_<timestamp>/dist` sandbox alongside copies of the compose artifacts, so the run is reproducible and the folder stays available to experiment with afterwards. Delegates stack bring-up to `stack-run` (frozen mode: read-only mount + `DEV=false` so the mage-watcher can't rebuild the packaged Go binary), verifies signature/version, runs the suite, and treats cleanup as optional — the sandbox is deleted only with the user's approval. Apply whenever the task is "test this plugin package / zip" or "run e2e against the built artifact".
version: 2.0.0
---

@../../../.config/AGENTS/skills/e2e-run-zip.md
