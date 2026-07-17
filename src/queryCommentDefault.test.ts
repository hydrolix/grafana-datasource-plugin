import {
  COMMENT_SETTINGS_WITH_DEFAULT,
  HDX_QUERY_COMMENT_DEFAULT,
  defaultValueFor,
} from "./queryCommentDefault";

// Byte-for-byte regression guard. The literal below is the spec's
// canonical attribution template (see
// `openspec/changes/plugin-query-comment-defaults/specs/hdx-query-attribution/spec.md`).
// Any change to the template must update both the constant and this
// literal in the same commit — they are intentionally duplicated to
// catch accidental edits in either direction.
describe("HDX_QUERY_COMMENT_DEFAULT", () => {
  it("equals the spec'd canonical 8-key attribution template", () => {
    expect(HDX_QUERY_COMMENT_DEFAULT).toBe(
      "grafana_user_email=${__user.email} " +
        "grafana_user_login=${__user.login} " +
        "grafana_panel_id=${__hydrolix.panel.id} " +
        "grafana_panel_name=${__hydrolix.panel.name} " +
        "grafana_dashboard_uid=${__dashboard.uid} " +
        "grafana_dashboard_title=${__dashboard} " +
        "grafana_app=${__hydrolix.app} " +
        "grafana_ref_id=${__hydrolix.ref_id}"
    );
  });
});

describe("defaultValueFor", () => {
  it("returns the canonical template for hdx_query_admin_comment", () => {
    expect(defaultValueFor("hdx_query_admin_comment")).toBe(
      HDX_QUERY_COMMENT_DEFAULT
    );
  });

  it("returns the canonical template for hdx_query_comment", () => {
    expect(defaultValueFor("hdx_query_comment")).toBe(
      HDX_QUERY_COMMENT_DEFAULT
    );
  });

  it("returns the empty string for any other setting name", () => {
    expect(defaultValueFor("hdx_query_pool_name")).toBe("");
    expect(defaultValueFor("hdx_query_max_rows")).toBe("");
    expect(defaultValueFor("hdx_query_streaming_result")).toBe("");
    expect(defaultValueFor("")).toBe("");
    expect(defaultValueFor("unknown_setting")).toBe("");
  });
});

describe("COMMENT_SETTINGS_WITH_DEFAULT", () => {
  it("contains exactly the two *_comment setting names", () => {
    expect(Array.from(COMMENT_SETTINGS_WITH_DEFAULT).sort()).toEqual([
      "hdx_query_admin_comment",
      "hdx_query_comment",
    ]);
  });
});
