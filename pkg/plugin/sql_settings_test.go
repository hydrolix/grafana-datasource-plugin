package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/stretchr/testify/assert"
)

func TestRewriteAdminCommentInSettings(t *testing.T) {
	managed := "grafana_meta_start; user_email=alice@example.com; user_login=alice; grafana_meta_end"

	t.Run("rejects unparseable sql", func(t *testing.T) {
		out, ok := rewriteAdminCommentInSettings("SELECT FROM WHERE", managed)
		assert.False(t, ok)
		assert.Equal(t, "", out)
	})

	t.Run("rejects sql with no SETTINGS clause", func(t *testing.T) {
		out, ok := rewriteAdminCommentInSettings("SELECT 1 FROM t", managed)
		assert.False(t, ok)
		assert.Equal(t, "", out)
	})

	t.Run("appends admin comment when SETTINGS lacks the key", func(t *testing.T) {
		out, ok := rewriteAdminCommentInSettings("SELECT 1 FROM t SETTINGS max_rows=1000", managed)
		assert.True(t, ok)
		assert.Contains(t, out, "max_rows=1000")
		assert.Contains(t, out, "hdx_query_admin_comment='"+managed+"'")
	})

	t.Run("merges into existing admin comment with user prefix", func(t *testing.T) {
		out, ok := rewriteAdminCommentInSettings(
			"SELECT 1 FROM t SETTINGS hdx_query_admin_comment='custom=foo'", managed)
		assert.True(t, ok)
		// custom=foo is preserved exactly once, separated from the managed
		// block by the canonical "; " separator.
		assert.Equal(t, 1, strings.Count(out, "custom=foo"))
		assert.Contains(t, out, "custom=foo; grafana_meta_start; ")
		assert.True(t, strings.HasSuffix(out, "grafana_meta_end'"))
	})

	t.Run("strips stale managed block before appending fresh one", func(t *testing.T) {
		stale := "grafana_meta_start; user_email=stale@example.com; grafana_meta_end"
		in := "SELECT 1 FROM t SETTINGS hdx_query_admin_comment='" + stale + "'"
		out, ok := rewriteAdminCommentInSettings(in, managed)
		assert.True(t, ok)
		assert.NotContains(t, out, "stale@example.com")
		assert.Contains(t, out, "user_email=alice@example.com")
		assert.Equal(t, 1, strings.Count(out, "grafana_meta_start"))
	})

	t.Run("repeated invocation does not duplicate managed block", func(t *testing.T) {
		sql := "SELECT 1 FROM t SETTINGS max_rows=10"
		first, ok := rewriteAdminCommentInSettings(sql, managed)
		assert.True(t, ok)
		second, ok := rewriteAdminCommentInSettings(first, managed)
		assert.True(t, ok)
		third, ok := rewriteAdminCommentInSettings(second, managed)
		assert.True(t, ok)
		assert.Equal(t, 1, strings.Count(third, "grafana_meta_start"))
		assert.Equal(t, 1, strings.Count(third, "grafana_meta_end"))
		assert.Equal(t, 1, strings.Count(third, "user_email=alice@example.com"))
	})

	t.Run("case-insensitive match on existing admin comment key", func(t *testing.T) {
		in := "SELECT 1 FROM t SETTINGS HDX_QUERY_ADMIN_COMMENT='custom=foo'"
		out, ok := rewriteAdminCommentInSettings(in, managed)
		assert.True(t, ok)
		assert.Contains(t, out, "custom=foo")
		assert.Contains(t, out, "user_email=alice@example.com")
		// The existing key spelling is replaced when we re-emit via the AST.
		// Ensure only one admin-comment entry remains regardless of casing.
		assert.Equal(t, 1, strings.Count(strings.ToLower(out), "hdx_query_admin_comment"))
	})

	t.Run("subquery SETTINGS does not match", func(t *testing.T) {
		// A SETTINGS clause inside a subquery binds to the inner SELECT;
		// rewriting it would alter semantics. The helper should target the
		// outer SELECT's SETTINGS, and if that's absent return ok=false.
		in := "SELECT * FROM (SELECT 1 SETTINGS y=1) AS t"
		_, ok := rewriteAdminCommentInSettings(in, managed)
		assert.False(t, ok)
	})

	t.Run("preserves sql preceding the SETTINGS clause byte-for-byte", func(t *testing.T) {
		// Whitespace, comments, and case outside the SETTINGS region must be
		// preserved by the splice — only the SETTINGS clause itself is re-emitted.
		in := "  SELECT\n  *\n  FROM t  /* note */  SETTINGS max_rows=1"
		out, ok := rewriteAdminCommentInSettings(in, managed)
		assert.True(t, ok)
		assert.True(t, strings.HasPrefix(out, "  SELECT\n  *\n  FROM t  /* note */  "))
	})

	t.Run("existing '' escape in source is preserved through round-trip", func(t *testing.T) {
		// User wrote 'it''s mine' in their SQL (source form). The parsed
		// Literal is `it''s mine` (with the doubled ' preserved verbatim).
		// After merge + re-emit the output must still be valid SQL — and the
		// content semantically still says "it's mine".
		in := "SELECT 1 FROM t SETTINGS hdx_query_admin_comment='it''s mine'"
		out, ok := rewriteAdminCommentInSettings(in, managed)
		assert.True(t, ok)
		// The output is well-formed SQL that re-parses cleanly.
		_, err := parser.NewParser(out).ParseStmts()
		assert.NoError(t, err)
		// The user's content shows up once with proper '' encoding.
		assert.Contains(t, out, "it''s mine")
		assert.Equal(t, 1, strings.Count(out, "it''s mine"))
	})

	t.Run("apostrophe in managed value is encoded so SQL stays valid", func(t *testing.T) {
		// This is the bug scenario: a Grafana user named "Alice O'Brien" or
		// a panel titled "Bob's queries". Without encoding, the raw '
		// character would terminate the string literal early and either
		// produce malformed SQL or — worse — open a SQL injection vector.
		managedWithApostrophe := "grafana_meta_start; user_name=Alice O'Brien; grafana_meta_end"
		in := "SELECT 1 FROM t SETTINGS max_rows=10"
		out, ok := rewriteAdminCommentInSettings(in, managedWithApostrophe)
		assert.True(t, ok)
		// The emitted SQL must re-parse — proves the apostrophe didn't break
		// quoting.
		_, err := parser.NewParser(out).ParseStmts()
		assert.NoError(t, err, "rewritten SQL must be valid; got %q", out)
		// The apostrophe is doubled in source form.
		assert.Contains(t, out, "Alice O''Brien")
		// And the raw apostrophe does NOT appear unescaped (only the doubled
		// form should be present inside the value).
		assert.NotRegexp(t, `O'[^']`, out)
	})

	t.Run("injection-style apostrophe in user fragment is neutralised", func(t *testing.T) {
		// A user with edit-dashboard permission could attempt to break out of
		// the SETTINGS string by setting a panel title or query-level admin
		// comment value containing apostrophes. The rewrite must keep the
		// SQL syntactically intact rather than letting the attacker's tokens
		// land at the SQL level.
		hostile := "x'; SELECT 1 FROM secrets; --"
		managedWithHostile := "grafana_meta_start; panel_name=" + hostile + "; grafana_meta_end"
		in := "SELECT 1 FROM t SETTINGS max_rows=10"
		out, ok := rewriteAdminCommentInSettings(in, managedWithHostile)
		assert.True(t, ok)
		stmts, err := parser.NewParser(out).ParseStmts()
		assert.NoError(t, err, "rewritten SQL must re-parse; got %q", out)
		// The whole hostile payload remains a single SETTINGS value, not a
		// separate statement — re-parsing yields exactly one statement.
		assert.Len(t, stmts, 1)
		// Doubled escape is present; raw apostrophe followed by non-' is
		// never emitted.
		assert.Contains(t, out, "x''; SELECT 1 FROM secrets; --")
		assert.NotRegexp(t, `x'[^']`, out)
	})
}

func TestExtractManagedAdminCommentFragment(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty input", in: "", want: ""},
		{name: "value without markers", in: "custom=foo", want: ""},
		{name: "managed block alone", in: "grafana_meta_start; user_email=a@b; grafana_meta_end", want: "grafana_meta_start; user_email=a@b; grafana_meta_end"},
		{name: "managed block with user prefix", in: "custom=foo; grafana_meta_start; user_email=a@b; grafana_meta_end", want: "grafana_meta_start; user_email=a@b; grafana_meta_end"},
		{name: "managed block with user suffix", in: "grafana_meta_start; user_email=a@b; grafana_meta_end; custom=foo", want: "grafana_meta_start; user_email=a@b; grafana_meta_end"},
		{name: "unterminated managed block returns empty", in: "grafana_meta_start; user_email=a@b", want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, extractManagedAdminCommentFragment(c.in))
		})
	}
}

func TestMutateInterpolatedQuery(t *testing.T) {
	plugin := &Hydrolix{querySettingsContextHandler: testContextHandler}
	managed := "grafana_meta_start; user_email=alice@example.com; grafana_meta_end"

	t.Run("no managed fragment in context — sql unchanged", func(t *testing.T) {
		in := "SELECT 1 FROM t SETTINGS max_rows=1"
		_, out := plugin.MutateInterpolatedQuery(context.Background(), in)
		assert.Equal(t, in, out)
	})

	t.Run("managed fragment in context and SETTINGS present — sql rewritten", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), managedAdminCommentCtxKey{}, managed)
		in := "SELECT 1 FROM t SETTINGS max_rows=1"
		_, out := plugin.MutateInterpolatedQuery(ctx, in)
		assert.NotEqual(t, in, out)
		assert.Contains(t, out, "max_rows=1")
		assert.Contains(t, out, "user_email=alice@example.com")
	})

	t.Run("no SETTINGS clause — sql unchanged (driver-context path is the safety net)", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), managedAdminCommentCtxKey{}, managed)
		in := "SELECT 1 FROM t"
		_, out := plugin.MutateInterpolatedQuery(ctx, in)
		assert.Equal(t, in, out)
	})

	t.Run("unparseable sql — sql unchanged", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), managedAdminCommentCtxKey{}, managed)
		in := "SELECT FROM WHERE"
		_, out := plugin.MutateInterpolatedQuery(ctx, in)
		assert.Equal(t, in, out)
	})
}
