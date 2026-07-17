# hdx-query-attribution Specification

## Purpose
Per-query attribution surface for the Hydrolix datasource: the `hdx_query_*_comment` QuerySettings, their canonical default attribution template, the pick-time pre-fill, and the `${__hydrolix.{panel.id,panel.name,app,ref_id}}` synthetic variables wired into the frontend interpolation path. Frontend-only — backend-only query paths ship setting values verbatim without template expansion.
## Requirements
### Requirement: Catalog registers both `*_comment` settings with a canonical default template

The plugin SHALL register both `hdx_query_admin_comment` (existing) and `hdx_query_comment` (new) in `src/labels.ts`'s `components.querySettings.values` array. Each entry SHALL declare `type: "textarea"` and a `default` field whose value is the canonical attribution template `HDX_QUERY_COMMENT_DEFAULT`. The constant SHALL be defined exactly once in `src/queryCommentDefault.ts` and imported by `labels.ts` (and any test file requiring equality checks against it).

`HDX_QUERY_COMMENT_DEFAULT` SHALL equal:

```
grafana_user_email=${__user.email} grafana_user_login=${__user.login} grafana_panel_id=${__hydrolix.panel.id} grafana_panel_name=${__hydrolix.panel.name} grafana_dashboard_uid=${__dashboard.uid} grafana_dashboard_title=${__dashboard} grafana_app=${__hydrolix.app} grafana_ref_id=${__hydrolix.ref_id}
```

#### Scenario: hdx_query_comment is registered as a textarea with the canonical default

- **GIVEN** the QuerySettings catalog in `src/labels.ts`
- **WHEN** `labels.components.querySettings.values` is iterated
- **THEN** the array SHALL contain an entry where `setting === "hdx_query_comment"`, `type === "textarea"`, and `default === HDX_QUERY_COMMENT_DEFAULT`

#### Scenario: hdx_query_admin_comment gains the same canonical default

- **GIVEN** the existing `hdx_query_admin_comment` catalog entry
- **WHEN** the entry is read
- **THEN** it SHALL declare `default === HDX_QUERY_COMMENT_DEFAULT` (was previously undefined)

#### Scenario: canonical template is a single exported constant

- **GIVEN** the modules `src/queryCommentDefault.ts` and `src/labels.ts`
- **WHEN** the codebase is grepped
- **THEN** the canonical template literal SHALL appear exactly once, as the value of `export const HDX_QUERY_COMMENT_DEFAULT` in `src/queryCommentDefault.ts`; `labels.ts` SHALL reference it by import, not by re-declaring the literal

### Requirement: Picking either `*_comment` setting pre-fills the input with the canonical default

The QuerySettings UI (`src/components/QuerySettings.tsx`) SHALL set the input value to `HDX_QUERY_COMMENT_DEFAULT` when the user selects `hdx_query_admin_comment` or `hdx_query_comment` from the setting-name dropdown. For every other catalog setting the input value SHALL be the empty string (preserving existing behaviour). The pre-fill SHALL fire only on the `onNameChange` event — re-renders, parent-state updates, and dashboard-load SHALL NOT overwrite a stored value with the default.

#### Scenario: picking hdx_query_admin_comment pre-fills the canonical default

- **GIVEN** an empty QuerySettings row created via the "+" button
- **WHEN** the user selects `hdx_query_admin_comment` from the setting-name dropdown
- **THEN** the row's value SHALL equal `HDX_QUERY_COMMENT_DEFAULT`

#### Scenario: picking hdx_query_comment pre-fills the canonical default

- **GIVEN** an empty QuerySettings row
- **WHEN** the user selects `hdx_query_comment` from the setting-name dropdown
- **THEN** the row's value SHALL equal `HDX_QUERY_COMMENT_DEFAULT`

#### Scenario: picking a non-comment setting leaves the input empty

- **GIVEN** an empty QuerySettings row
- **WHEN** the user selects `hdx_query_pool_name` (or any setting other than the two `*_comment` settings) from the dropdown
- **THEN** the row's value SHALL equal the empty string

#### Scenario: stored values are not overwritten on render

- **GIVEN** a saved QuerySettings row with `setting === "hdx_query_admin_comment"` and `value === "custom note"`
- **WHEN** the editor component mounts and re-renders
- **THEN** the input SHALL display `"custom note"` and SHALL NOT be replaced by `HDX_QUERY_COMMENT_DEFAULT`

### Requirement: `${__hydrolix.panel.id}` and `${__hydrolix.panel.name}` resolve from the request, empty when undefined

`prepareTarget` in `src/datasource.ts` SHALL register two synthetic variables named `panel.id` and `panel.name`. The `panel.id` resolver SHALL return `String(request.panelId)` when `request.panelId` is a defined number, and `""` otherwise. The `panel.name` resolver SHALL return `request.panelName` when defined, and `""` otherwise. Both variables SHALL be expanded by the existing `replace` helper in `src/syntheticVariables.ts` without modification.

#### Scenario: panel.id expands to the request's panelId in a dashboard

- **GIVEN** a `DataQueryRequest` with `panelId === 12`
- **WHEN** a setting value containing `${__hydrolix.panel.id}` is interpolated through `prepareTarget`
- **THEN** the expanded value SHALL contain the substring `"12"` where the placeholder appeared, and the placeholder text SHALL NOT remain

#### Scenario: panel.name expands to the request's panelName

- **GIVEN** a `DataQueryRequest` with `panelName === "Throughput"`
- **WHEN** a setting value containing `${__hydrolix.panel.name}` is interpolated
- **THEN** the expanded value SHALL contain the substring `"Throughput"` where the placeholder appeared

#### Scenario: panel.id falls back to empty when panelId is undefined (Explore)

- **GIVEN** a `DataQueryRequest` with `panelId === undefined`
- **WHEN** a setting value `"a=${__hydrolix.panel.id};b=1"` is interpolated
- **THEN** the expanded value SHALL be exactly `"a=;b=1"`

#### Scenario: panel.name falls back to empty when panelName is undefined

- **GIVEN** a `DataQueryRequest` with `panelName === undefined`
- **WHEN** a setting value `"a=${__hydrolix.panel.name};b=1"` is interpolated
- **THEN** the expanded value SHALL be exactly `"a=;b=1"`

### Requirement: `${__hydrolix.app}` and `${__hydrolix.ref_id}` resolve from the request and the target

`prepareTarget` SHALL register two synthetic variables named `app` and `ref_id`. The `app` resolver SHALL return `request.app` (which is always defined as a `CoreApp` or string). The `ref_id` resolver SHALL return the per-target `t.refId` (which is always defined for any query that reaches `prepareTarget`).

#### Scenario: app expands to the request's app value

- **GIVEN** a `DataQueryRequest` with `app === "dashboard"` and a target with `refId === "A"`
- **WHEN** a setting value `"src=${__hydrolix.app};id=${__hydrolix.ref_id}"` is interpolated
- **THEN** the expanded value SHALL be exactly `"src=dashboard;id=A"`

#### Scenario: app reflects the annotation source for annotation queries

- **GIVEN** a `DataQueryRequest` with `app === "annotation"` and a target with `refId === "Anno"`
- **WHEN** a setting value `"src=${__hydrolix.app};id=${__hydrolix.ref_id}"` is interpolated
- **THEN** the expanded value SHALL be exactly `"src=annotation;id=Anno"`

### Requirement: Existing `${__hydrolix.raw_query}` and `${__hydrolix.query_source}` resolvers are preserved

`prepareTarget` SHALL continue to register `raw_query` (→ `t.rawSql`) and `query_source` (→ `request.app`) as it does today. The addition of `panel.id`, `panel.name`, `app`, and `ref_id` SHALL NOT change the behaviour or signature of the existing two resolvers.

#### Scenario: raw_query continues to expand to the target's rawSql

- **GIVEN** a target with `rawSql === "SELECT 1"`
- **WHEN** a setting value `"q=${__hydrolix.raw_query}"` is interpolated
- **THEN** the expanded value SHALL be exactly `"q=SELECT 1"`

#### Scenario: query_source continues to expand to the request's app

- **GIVEN** a `DataQueryRequest` with `app === "explore"`
- **WHEN** a setting value `"s=${__hydrolix.query_source}"` is interpolated
- **THEN** the expanded value SHALL be exactly `"s=explore"`