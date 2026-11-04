package cte

import (
	"testing"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sourcesOf returns the PKSource of every macro in sql.
func sourcesOf(t *testing.T, sql string) []PKSource {
	t.Helper()
	exprs, err := parser.NewParser(sql).ParseStmts()
	require.NoError(t, err)
	got, err := GetMacroCTEs(exprs)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	sources := make([]PKSource, 0, len(got))
	for _, c := range got {
		sources = append(sources, c.Source)
	}
	return sources
}

// sourceOf returns the PKSource of the only macro in sql.
func sourceOf(t *testing.T, sql string) PKSource {
	t.Helper()
	sources := sourcesOf(t, sql)
	require.Len(t, sources, 1)
	return sources[0]
}

func layerNames(s PKSource) []string {
	names := make([]string, 0, len(s.Layers))
	for _, l := range s.Layers {
		names = append(names, l.Name)
	}
	return names
}

func TestSource_PlainTable(t *testing.T) {
	s := sourceOf(t, "SELECT $__timeFilter() FROM sample.logs")
	require.NoError(t, s.Err)
	assert.Equal(t, "sample", s.Database)
	assert.Equal(t, "logs", s.Table)
	assert.Empty(t, s.Layers)
}

func TestSource_UnqualifiedTableLeavesDatabaseEmpty(t *testing.T) {
	s := sourceOf(t, "SELECT $__timeFilter() FROM logs")
	require.NoError(t, s.Err)
	assert.Equal(t, "", s.Database)
	assert.Equal(t, "logs", s.Table)
	assert.Empty(t, s.Layers)
}

func TestSource_TicketExampleFollowsWithAlias(t *testing.T) {
	sql := "WITH errors AS (SELECT * FROM sample.logs WHERE status >= 500)\n" +
		"SELECT $__timeInterval() AS t, count() FROM errors\n" +
		"WHERE $__timeFilter()\nGROUP BY t"
	sources := sourcesOf(t, sql)
	require.Len(t, sources, 2, "both macros resolve")
	for _, s := range sources {
		require.NoError(t, s.Err)
		assert.Equal(t, "sample", s.Database)
		assert.Equal(t, "logs", s.Table)
		assert.Equal(t, []string{"`errors`"}, layerNames(s))
	}
}

func TestSource_AliasChainResolvesToUnderlyingTable(t *testing.T) {
	s := sourceOf(t, "WITH b AS (SELECT * FROM sample.logs), a AS (SELECT * FROM b) "+
		"SELECT $__timeFilter() FROM a")
	require.NoError(t, s.Err)
	assert.Equal(t, "sample", s.Database)
	assert.Equal(t, "logs", s.Table)
	assert.Equal(t, []string{"`a`", "`b`"}, layerNames(s), "outermost first")
}

func TestSource_InlineSubquery(t *testing.T) {
	s := sourceOf(t, "SELECT $__timeFilter() FROM (SELECT * FROM sample.logs) AS s")
	require.NoError(t, s.Err)
	assert.Equal(t, "logs", s.Table)
	assert.Equal(t, []string{"subquery"}, layerNames(s))
}

func TestSource_AliasInsideSubquery(t *testing.T) {
	s := sourceOf(t, "WITH e AS (SELECT * FROM sample.logs) "+
		"SELECT $__timeFilter() FROM (SELECT * FROM e)")
	require.NoError(t, s.Err)
	assert.Equal(t, "logs", s.Table)
	assert.Equal(t, []string{"subquery", "`e`"}, layerNames(s))
}

func TestSource_SubqueryInsideAlias(t *testing.T) {
	s := sourceOf(t, "WITH e AS (SELECT * FROM (SELECT * FROM sample.logs)) "+
		"SELECT $__timeFilter() FROM e")
	require.NoError(t, s.Err)
	assert.Equal(t, "logs", s.Table)
	assert.Equal(t, []string{"`e`", "subquery"}, layerNames(s))
}

func TestSource_AliasBodySeesItsOwnWith(t *testing.T) {
	s := sourceOf(t, "WITH e AS (WITH inner_e AS (SELECT * FROM sample.logs) SELECT * FROM inner_e) "+
		"SELECT $__timeFilter() FROM e")
	require.NoError(t, s.Err)
	assert.Equal(t, "logs", s.Table)
	assert.Equal(t, []string{"`e`", "`inner_e`"}, layerNames(s))
}

func TestSource_AliasedReferenceToWithAlias(t *testing.T) {
	s := sourceOf(t, "WITH e AS (SELECT * FROM sample.logs) SELECT $__timeFilter() FROM e AS x")
	require.NoError(t, s.Err)
	assert.Equal(t, "logs", s.Table)
	assert.Equal(t, []string{"`e`"}, layerNames(s))
}

func TestSource_NearestScopeWins(t *testing.T) {
	s := sourceOf(t, "WITH x AS (SELECT * FROM db.outer_tbl) SELECT * FROM "+
		"(WITH x AS (SELECT * FROM db.inner_tbl) SELECT $__timeFilter() FROM x)")
	require.NoError(t, s.Err)
	assert.Equal(t, "inner_tbl", s.Table)
}

func TestSource_AliasShadowsSameNamedTable(t *testing.T) {
	s := sourceOf(t, "WITH logs AS (SELECT * FROM sample.real_logs) SELECT $__timeFilter() FROM logs")
	require.NoError(t, s.Err)
	assert.Equal(t, "sample", s.Database)
	assert.Equal(t, "real_logs", s.Table)
}

func TestSource_NoSingleSourceNamesTheLayer(t *testing.T) {
	tests := []struct {
		name, sql, layer string
	}{
		{"join in alias", "WITH e AS (SELECT * FROM a JOIN b ON a.k = b.k) SELECT $__timeFilter() FROM e", "`e`"},
		{"table function in alias", "WITH e AS (SELECT * FROM numbers(10)) SELECT $__timeFilter() FROM e", "`e`"},
		{"union in alias", "WITH e AS (SELECT * FROM a UNION ALL SELECT * FROM b) SELECT $__timeFilter() FROM e", "`e`"},
		{"no FROM in alias", "WITH e AS (SELECT now() AS ts) SELECT $__timeFilter() FROM e", "`e`"},
		{"join in subquery", "SELECT $__timeFilter() FROM (SELECT * FROM a JOIN b ON a.k = b.k)", "subquery"},
		{"self-reference", "WITH e AS (SELECT * FROM e) SELECT $__timeFilter() FROM e", "`e`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := sourceOf(t, tt.sql)
			require.ErrorIs(t, s.Err, ErrNoSingleSource)
			assert.Contains(t, s.Err.Error(), tt.layer)
		})
	}
}

func TestSource_DoesNotChangeExistingFields(t *testing.T) {
	// Table / Database / CTE are served by the /macroCTE route and used by
	// ad-hoc filters; resolving the PK source must leave them as they were.
	exprs, err := parser.NewParser("WITH e AS (SELECT * FROM sample.logs) SELECT $__timeFilter() FROM e").ParseStmts()
	require.NoError(t, err)
	got, err := GetMacroCTEs(exprs)
	require.NoError(t, err)
	require.Len(t, got, 1)
	for _, c := range got {
		assert.Equal(t, "e", c.Table)
		assert.Equal(t, "", c.Database)
		assert.Equal(t, "(SELECT * FROM sample.logs)", c.CTE)
	}
}

func TestLayer_Exposes(t *testing.T) {
	tests := []struct {
		name, body string
		want       bool
	}{
		{"star", "SELECT * FROM t", true},
		{"qualified star", "SELECT t.* FROM t", true},
		{"bare column", "SELECT ts, v FROM t", true},
		{"qualified column", "SELECT t.ts FROM t", true},
		{"database-qualified column", "SELECT db.t.ts FROM db.t", true},
		{"expression aliased to the name", "SELECT toStartOfMinute(x) AS ts FROM t", true},
		{"renamed away", "SELECT ts AS time FROM t", false},
		{"dropped", "SELECT v FROM t", false},
		{"only inside an expression", "SELECT toDate(ts) FROM t", false},
		{"different column", "SELECT tsx FROM t", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := sourceOf(t, "SELECT $__timeFilter() FROM ("+tt.body+")")
			require.NoError(t, s.Err)
			require.Len(t, s.Layers, 1)
			assert.Equal(t, tt.want, s.Layers[0].Exposes("ts"))
		})
	}
}

func TestSource_OuterJoinKeepsTodaysLeftTable(t *testing.T) {
	// Only alias and subquery bodies must have a single source. A JOIN in the
	// macro's own FROM keeps the left table it has always resolved to.
	s := sourceOf(t, "SELECT $__timeFilter() FROM sample.logs AS l JOIN sample.users AS u ON l.k = u.k")
	require.NoError(t, s.Err)
	assert.Equal(t, "sample", s.Database)
	assert.Equal(t, "logs", s.Table)
	assert.Empty(t, s.Layers)
}
