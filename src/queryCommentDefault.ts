// Canonical attribution template pre-filled into the two `*_comment`
// QuerySettings when picked from the dropdown. The placeholders are
// expanded at query time by templateSrv (Grafana built-ins) and by
// src/syntheticVariables.ts (the `${__hydrolix.*}` resolvers wired in
// `datasource.ts:prepareTarget`).
//
// Plain string concatenation (not a template literal) so the `${...}`
// placeholders survive verbatim into the runtime value.
export const HDX_QUERY_COMMENT_DEFAULT =
  "grafana_user_email=${__user.email} " +
  "grafana_user_login=${__user.login} " +
  "grafana_panel_id=${__hydrolix.panel.id} " +
  "grafana_panel_name=${__hydrolix.panel.name} " +
  "grafana_dashboard_uid=${__dashboard.uid} " +
  "grafana_dashboard_title=${__dashboard} " +
  "grafana_app=${__hydrolix.app} " +
  "grafana_ref_id=${__hydrolix.ref_id}";

export const COMMENT_SETTINGS_WITH_DEFAULT: ReadonlySet<string> = new Set([
  "hdx_query_admin_comment",
  "hdx_query_comment",
]);

export function defaultValueFor(setting: string): string {
  return COMMENT_SETTINGS_WITH_DEFAULT.has(setting)
    ? HDX_QUERY_COMMENT_DEFAULT
    : "";
}
