## Why

The plugin exposes `hdx_query_admin_comment` as a free-form text setting that ships into ClickHouse's `SETTINGS` clause for cluster-side observability (query log, audit). Three gaps:

1. **A second attribution setting** — `hdx_query_comment` — that the Hydrolix cluster treats as a separate field. Not exposed from the plugin's QuerySettings UI today.

2. **No canonical template.** Picking `hdx_query_admin_comment` from the dropdown leaves the value empty. Every team invents their own attribution shape; comparing across teams is impossible. Nothing in the UI surfaces what variables are available.

3. **Panel / app / refID context unreachable on the frontend.** `${__user.email}`, `${__user.login}`, `${__dashboard.uid}`, `${__dashboard}` already work via Grafana's `templateSrv`. `${__panel.id}`, `${__panel.name}`, `${__app}`, `${__ref_id}` do not — even though the corresponding fields are present on `DataQueryRequest` (`panelId`, `panelName`, `app`) and on each target (`refId`). A user composing a comment by hand cannot reach panel-level context today.

This change adds the new setting, a canonical default template covering everything the cluster wants for attribution, the pre-fill on setting selection, and the four missing synthetic variables wired into the existing interpolation path.

## What Changes

- Add `hdx_query_comment` to the `src/labels.ts` querySettings catalog with `type: "textarea"` and a `default` field pointing at the canonical attribution template.
- Add the same `default` field to the existing `hdx_query_admin_comment` catalog entry. Both settings share the canonical template — the Hydrolix cluster differentiates them; the plugin does not.
- `src/components/QuerySettings.tsx`: when the user picks either `*_comment` setting from the dropdown, the input pre-fills with the catalog's default template. No "reset" gesture beyond removing the setting and re-picking it.
- `src/syntheticVariables.ts` + `src/datasource.ts` (`prepareTarget`): register four new plugin synthetic variables — `${__hydrolix.panel.id}`, `${__hydrolix.panel.name}`, `${__hydrolix.app}`, `${__hydrolix.ref_id}` — resolved against `DataQueryRequest.panelId` / `.panelName` / `.app` and the per-target `refId`.
- Unit + Playwright e2e coverage. (Specific cases in `tasks.md`.)

Not breaking for the plugin's frontend, HTTP wire format, dashboards, or query semantics. Existing dashboards keep whatever value they have stored for `hdx_query_admin_comment` — the pre-fill applies only at the moment the user picks the setting from the dropdown; it never overwrites stored state. New variables expand to empty strings in contexts where the corresponding `DataQueryRequest` field is undefined (Explore: no `panelId` / `panelName`).

## Capabilities

### New Capabilities

- `hdx-query-attribution`: Per-query attribution surface — the `hdx_query_*_comment` settings, their canonical default template, the pick-time pre-fill, and the four `${__hydrolix.{panel.id,panel.name,app,ref_id}}` synthetic variables. Frontend-only — the backend `MutateQueryData` path ships setting values verbatim without template expansion.

### Modified Capabilities

<!-- None. The query-settings catalog and the synthetic-variable interpolator predate OpenSpec adoption; this change is the first spec that codifies any of their behaviour. -->

## Impact

- **Frontend**: `src/labels.ts`, `src/components/QuerySettings.tsx`, `src/syntheticVariables.ts`, `src/datasource.ts`. New module `src/queryCommentDefault.ts` holding the canonical template constant.
- **Backend (Go)**: none.
- **Tests**: new unit tests for the pre-fill behavior on setting selection, the four new synthetic variables, and the catalog defaults. New Playwright e2e covering pick → pre-fill → wire-level expansion.
- **Dependencies**: none.
- **User-visible**: picking either `*_comment` setting now pre-fills the canonical default template into the input. Existing stored values are untouched.
- **Security**: no new injection surfaces. Both `*_comment` settings are passed verbatim into `SETTINGS … = '…'` on the wire as they are today; Hydrolix is responsible for parsing them.
- **Sequencing**: independent of the in-flight sqlds-fork retirement arc. Can ship at any time.

### Known limitation (not addressed)

Backend-only query paths (alerting, public-dashboard SSR, recording rules) bypass `prepareTarget` and ship `${__hydrolix.*}` placeholders unexpanded to the cluster. This pre-existing constraint already affects `${__hydrolix.raw_query}` and `${__hydrolix.query_source}`; this change does not fix it. Documented in `design.md`.
