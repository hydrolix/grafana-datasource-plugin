## Context

`hdx_query_admin_comment` is a Hydrolix cluster setting that propagates a free-form annotation into the cluster's query log and audit trail. The plugin surfaces it through the QuerySettings UI (`src/components/QuerySettings.tsx`) as a single-line `<Input>` of width 80. Picking the setting from the dropdown sets `value: ""`. Users craft attribution strings by hand, e.g. `db=${__dashboard}; du=${__user.login}`.

The plugin's interpolation path is the chain:

```
request → prepareTarget → querySettingsBuilder.addSettings
   → templateSrv.replace                     ← Grafana built-ins (${__user.*}, ${__dashboard*}, …)
   → syntheticVariables.replace              ← Hydrolix-private vars (${__hydrolix.<name>})
   → SETTINGS clause on the wire
```

`syntheticVariables.replace` (`src/syntheticVariables.ts:1-10`) uses literal `String.replaceAll` keyed by `${__hydrolix.${k}}`. Today only two synthetic vars exist: `raw_query` and `query_source`.

Frontend interpolation is the only point where user / panel / app context is available — backend-only query paths (alerting SSR, recording rules) ship setting values raw, with no template expansion. Finding `2026-06-12-querysettings-interpolation.md` documents the constraint. This change inherits it.

## Goals / Non-Goals

**Goals:**
- Expose `hdx_query_comment` with the same UX shape as `hdx_query_admin_comment`.
- Define a canonical default template covering every attribution variable the Hydrolix cluster wants; pre-fill the input with it when either `*_comment` setting is picked.
- Wire four new synthetic variables — `${__hydrolix.panel.id}`, `${__hydrolix.panel.name}`, `${__hydrolix.app}`, `${__hydrolix.ref_id}` — into `prepareTarget` so the template expands to useful data at query time.

**Non-Goals:**
- Generalising the pre-fill behaviour to every catalog entry with a `default` field. `hdx_query_streaming_result` (`default: true`) and `hdx_query_max_concurrent_partitions` (`default: 3`) stay as they are. (Locked at O3 = A — special-case the two `*_comment` settings.)
- Extending interpolation to backend-only query paths. The pre-existing limitation for `raw_query` / `query_source` applies to the new vars too. Documented in proposal.md; not fixed here.
- Adding a "reset to default" gesture. Deleting the setting and re-picking it is the affordance.

## Decisions

### D1. Synthetic-variable naming: `${__hydrolix.panel.id}` (dotted sub-path under the plugin namespace)

Existing convention: `${__hydrolix.raw_query}`, `${__hydrolix.query_source}` — single-segment.

New names use a dotted sub-path: `${__hydrolix.panel.id}`, `${__hydrolix.panel.name}`, `${__hydrolix.app}`, `${__hydrolix.ref_id}`.

**Why dotted.** Reads more naturally alongside Grafana's built-in `${__user.email}` / `${__dashboard.uid}`. The default template ends up legible: `grafana_panel_id=${__hydrolix.panel.id}` over `grafana_panel_id=${__hydrolix.panel_id}`.

**Why namespaced under `__hydrolix.` and not bare `${__panel.id}`.** Grafana may add `${__panel.id}` as an official built-in in a future release; keeping the plugin's names under `__hydrolix.` avoids the collision.

**Compatibility with existing `replace()`.** `src/syntheticVariables.ts:5` uses literal `String.replaceAll` keyed on `${__hydrolix.${k}}`. Dots in `k` are matched literally — no regex escaping, no implementation change required.

**Alternative considered.** Pass `__panel`, `__app`, `__ref_id` as scoped vars to `templateSrv.replace(value, scopedVars)`. Rejected — squats on Grafana's namespace and behaves inconsistently in contexts where Grafana defines a same-name variable with different semantics.

### D2. Canonical default template lives in a dedicated module

```ts
// src/queryCommentDefault.ts
export const HDX_QUERY_COMMENT_DEFAULT =
  "grafana_user_email=${__user.email} " +
  "grafana_user_login=${__user.login} " +
  "grafana_panel_id=${__hydrolix.panel.id} " +
  "grafana_panel_name=${__hydrolix.panel.name} " +
  "grafana_dashboard_uid=${__dashboard.uid} " +
  "grafana_dashboard_title=${__dashboard} " +
  "grafana_app=${__hydrolix.app} " +
  "grafana_ref_id=${__hydrolix.ref_id}";
```

Imported by `src/labels.ts` (referenced twice in the catalog) and by tests / e2e specs needing to assert the pre-filled value.

**Why a dedicated module instead of inlining in `labels.ts`.** Tests for the pre-fill behaviour need to compare against the constant; the constant deserves its own importable name. Keeps `labels.ts` from accumulating a multi-line string literal that scrolls the catalog out of focus.

**Why fetched only at pick time, not on every input render.** Avoid the failure mode where editing the catalog default later silently rewrites a user's already-stored value. Pick-time copy makes the binding explicit: pick → fill → store; subsequent renders reflect the stored value, not the catalog.

**Trade-off accepted (per O2):** existing dashboards baked at template v1 keep their literal copy forever. Updating the template in code is cosmetic for new pick events only. The cluster tolerates either version (the comment is free-form text) so this is acceptable.

### D3. Pre-fill on `onNameChange`, not on `newSetting`

```tsx
const onNameChange = useCallback(
  (index: number, selected: string) => {
    const def = labels.components.querySettings.values.find(v => v.setting === selected);
    let copy = [...settingsArray];
    copy[index] = {
      setting: selected,
      value: defaultValueFor(selected, def),
      type: settingTypes[selected],
    };
    updateSettings(copy);
  },
  [...]
);

// Special-case the two *_comment settings (per O3 = A).
function defaultValueFor(name: string, def?: { default?: unknown }): string {
  if (name === "hdx_query_admin_comment" || name === "hdx_query_comment") {
    return HDX_QUERY_COMMENT_DEFAULT;
  }
  return "";
}
```

`newSetting` keeps `value: ""` because the setting name is empty — there's no catalog entry to look up yet.

**Why pre-fill in `onNameChange` rather than via a `useEffect` watching `settingsArray`.** The pre-fill is a single user action's consequence; binding to the name-selection event is direct. A `useEffect` would re-fire on every state update and would need an extra "have I already pre-filled this index" flag.

**Why special-case rather than catalog-driven.** Locked at O3 = A. Generic rollout (pre-fill for every catalog entry with `default`) would change visible behaviour for `hdx_query_streaming_result` (`true` shown by default) and `hdx_query_max_concurrent_partitions` (`3` shown by default) — small but not free. Deferred to a future change if customer demand emerges.

### D4. Muted-while-default styling — **dropped**

Originally scoped to render the input text in `theme.colors.text.secondary` while the input value strictly equalled `HDX_QUERY_COMMENT_DEFAULT`. Implementation used a `defaultValue` emotion style with a nested `input { color: … }` selector applied via `className`.

**Why dropped.** The nested selector didn't override Grafana's `Input` component's own text color in the runtime DOM (`@grafana/ui` applies its own color rules with higher specificity than an emotion-injected child selector). Debugging the specificity wins for one cosmetic cue isn't worth the surface area; the pre-fill behaviour (D3) is the load-bearing UX and works.

**What remains.** Pre-fill still happens on `onNameChange`; the user sees the template appear when they pick `hdx_query_*_comment`. If they edit, they edit. No visual indicator of "still on default vs customized" — readers infer that from the text itself.

**Future option.** Revisit if a cleaner styling hook becomes available (e.g. `@grafana/ui` exposes a `variant="muted"` prop). Track in a follow-up change; do not block this one.

### D5. New synthetic variables wired into `prepareTarget`

```ts
// src/datasource.ts:prepareTarget
const builder = this.querySettingsBuilder({
  raw_query:    () => t.rawSql,
  query_source: () => request.app,
  "panel.id":   () => String(request.panelId ?? ""),
  "panel.name": () => request.panelName ?? "",
  "app":        () => request.app,
  "ref_id":     () => t.refId,
});
```

`__hydrolix.app` is a duplicate resolver of `__hydrolix.query_source` (both → `request.app`). Kept distinct because the user-facing template reads more naturally as `grafana_app=...` than `grafana_query_source=...`; existing consumers of `${__hydrolix.query_source}` continue to work unchanged.

**Empty-string fallback for missing `panelId` / `panelName`.** Explore queries lack both. The rendered comment becomes `grafana_panel_id= grafana_panel_name=` — ugly but truthful. Alternatives considered:

| Fallback | Pro | Con |
|---|---|---|
| `""` (chosen) | Mirrors how `${__user.email}` already behaves for anonymous users — var expands, just to empty. | Comment has dangling `key=` pairs. |
| `"unknown"` | More readable. | Invents a string that doesn't appear elsewhere; could be confused with a real panel name. |
| Skip the whole `key=value` pair | Cleanest output. | Requires post-processing the expanded template to remove empty values — extra mechanism. |
| Leave `${__hydrolix.panel.id}` unexpanded | Surfaces the failure. | Pollutes the cluster's query log with placeholder syntax. |

### D6. Backend-only query paths: known leak, not addressed

Backend-only query paths (alerting, public-dashboard SSR, recording rules) bypass `prepareTarget`. Their `SETTINGS` clauses ship `${__hydrolix.panel.id}` and friends unexpanded — the cluster receives the literal placeholder string.

**Why not fix.** The same gap exists today for `${__hydrolix.raw_query}` and `${__hydrolix.query_source}`. Addressing it requires extending `MutateQueryData` (Go backend) to perform Grafana-equivalent template expansion — a substantially larger change that requires pulling user-context and panel-context plumbing into the backend. The catalog-attribution piece doesn't justify pulling that in. Documented in proposal.md "Known limitation".

### D7. Keep `"textarea"` rendering as single-line `<Input>` (no `<TextArea>` swap)

The default template is ~290 characters, and the existing `<Input width={80}>` scrolls horizontally to accommodate it. Switching `type: "textarea"` to render as `@grafana/ui`'s multi-line `<TextArea>` would be a small UX improvement (wrap-friendly, multi-row visible), but it changes the layout of the existing `hdx_query_admin_comment` row in every dashboard's editor — including dashboards that have never touched the default template.

**Why skip.** Out of scope for this change. The horizontal scroll is functional; the layout change is the kind of thing that warrants its own small UI proposal so it can be evaluated independently of the attribution-defaults work. If demand emerges later, swap is one-line.

## Risks

- **[Risk]** Default template changes in a future release; existing dashboards keep the old template literally.
  → **Mitigation.** Documented in D2. If a future change requires a migration, dashboards can re-pick the setting to refresh; the cluster tolerates either version of the comment.

- **[Risk]** `request.panelName` contains characters that break the wire-level `SETTINGS` encoding (e.g. an apostrophe in a panel title).
  → **Mitigation.** Out of scope for this change — the same risk exists for `${__dashboard}` (dashboard title) today, and ad-hoc filter escaping (C7) already covers the user-value path. Proving signal: e2e test that sets `request.panelName` to `Foo's Panel` and asserts the query runs without a cluster-side `SYNTAX_ERROR`.
