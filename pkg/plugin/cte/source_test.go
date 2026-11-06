package cte

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
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
	for _, entry := range got {
		sources = append(sources, entry.Source)
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

func layerNames(source PKSource) []string {
	names := make([]string, 0, len(source.layers))
	for _, body := range source.layers {
		names = append(names, body.name)
	}
	return names
}

func TestSource_PlainTable(t *testing.T) {
	source := sourceOf(t, "SELECT $__timeFilter() FROM sample.logs")
	require.NoError(t, source.Err)
	assert.Equal(t, "sample", source.Database)
	assert.Equal(t, "logs", source.Table)
	assert.Empty(t, source.layers)
}

func TestSource_UnqualifiedTableLeavesDatabaseEmpty(t *testing.T) {
	source := sourceOf(t, "SELECT $__timeFilter() FROM logs")
	require.NoError(t, source.Err)
	assert.Equal(t, "", source.Database)
	assert.Equal(t, "logs", source.Table)
	assert.Empty(t, source.layers)
}

func TestSource_TicketExampleFollowsWithAlias(t *testing.T) {
	sql := "WITH errors AS (SELECT * FROM sample.logs WHERE status >= 500)\n" +
		"SELECT $__timeInterval() AS t, count() FROM errors\n" +
		"WHERE $__timeFilter()\nGROUP BY t"
	sources := sourcesOf(t, sql)
	require.Len(t, sources, 2, "both macros resolve")
	for _, source := range sources {
		require.NoError(t, source.Err)
		assert.Equal(t, "sample", source.Database)
		assert.Equal(t, "logs", source.Table)
		assert.Equal(t, []string{"`errors`"}, layerNames(source))
	}
}

func TestSource_AliasChainResolvesToUnderlyingTable(t *testing.T) {
	source := sourceOf(t, "WITH b AS (SELECT * FROM sample.logs), a AS (SELECT * FROM b) "+
		"SELECT $__timeFilter() FROM a")
	require.NoError(t, source.Err)
	assert.Equal(t, "sample", source.Database)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`a`", "`b`"}, layerNames(source), "outermost first")
}

func TestSource_InlineSubquery(t *testing.T) {
	source := sourceOf(t, "SELECT $__timeFilter() FROM (SELECT * FROM sample.logs) AS s")
	require.NoError(t, source.Err)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"subquery"}, layerNames(source))
}

func TestSource_AliasInsideSubquery(t *testing.T) {
	source := sourceOf(t, "WITH e AS (SELECT * FROM sample.logs) "+
		"SELECT $__timeFilter() FROM (SELECT * FROM e)")
	require.NoError(t, source.Err)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"subquery", "`e`"}, layerNames(source))
}

func TestSource_SubqueryInsideAlias(t *testing.T) {
	source := sourceOf(t, "WITH e AS (SELECT * FROM (SELECT * FROM sample.logs)) "+
		"SELECT $__timeFilter() FROM e")
	require.NoError(t, source.Err)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`e`", "subquery"}, layerNames(source))
}

func TestSource_AliasBodySeesItsOwnWith(t *testing.T) {
	source := sourceOf(t, "WITH e AS (WITH inner_e AS (SELECT * FROM sample.logs) SELECT * FROM inner_e) "+
		"SELECT $__timeFilter() FROM e")
	require.NoError(t, source.Err)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`e`", "`inner_e`"}, layerNames(source))
}

func TestSource_AliasedReferenceToWithAlias(t *testing.T) {
	source := sourceOf(t, "WITH e AS (SELECT * FROM sample.logs) SELECT $__timeFilter() FROM e AS x")
	require.NoError(t, source.Err)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`e`"}, layerNames(source))
}

func TestSource_NearestScopeWins(t *testing.T) {
	source := sourceOf(t, "WITH x AS (SELECT * FROM db.outer_tbl) SELECT * FROM "+
		"(WITH x AS (SELECT * FROM db.inner_tbl) SELECT $__timeFilter() FROM x)")
	require.NoError(t, source.Err)
	assert.Equal(t, "inner_tbl", source.Table)
}

func TestSource_AliasShadowsSameNamedTable(t *testing.T) {
	source := sourceOf(t, "WITH logs AS (SELECT * FROM sample.real_logs) SELECT $__timeFilter() FROM logs")
	require.NoError(t, source.Err)
	assert.Equal(t, "sample", source.Database)
	assert.Equal(t, "real_logs", source.Table)
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
		{"except in alias", "WITH e AS (SELECT * FROM a EXCEPT SELECT * FROM b) SELECT $__timeFilter() FROM e", "`e`"},
		{"intersect in alias", "WITH e AS (SELECT * FROM a INTERSECT SELECT * FROM b) SELECT $__timeFilter() FROM e", "`e`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := sourceOf(t, tt.sql)
			require.ErrorIs(t, source.Err, ErrNoSingleSource)
			assert.Contains(t, source.Err.Error(), tt.layer)
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
	for _, entry := range got {
		assert.Equal(t, "e", entry.Table)
		assert.Equal(t, "", entry.Database)
		assert.Equal(t, "(SELECT * FROM sample.logs)", entry.CTE)
	}
}

func TestLayer_exposes(t *testing.T) {
	tests := []struct {
		name, body string
		want       bool
	}{
		{"star", "SELECT * FROM t", true},
		{"qualified star", "SELECT t.* FROM t", true},
		{"bare column", "SELECT ts, v FROM t", true},
		{"qualified column", "SELECT t.ts FROM t", true},
		{"database-qualified column", "SELECT db.t.ts FROM db.t", true},
		{"expression over the column aliased to the name", "SELECT toStartOfMinute(ts) AS ts FROM t", true},
		{"other column aliased to the name", "SELECT created AS ts FROM t", false},
		{"table-alias-qualified column", "SELECT l.ts FROM t AS l", true},
		{"tuple or JSON subcolumn", "SELECT j.ts FROM t", false},
		{"star except the column", "SELECT * EXCEPT (ts) FROM t", false},
		{"star except without parens", "SELECT * EXCEPT ts FROM t", false},
		{"star except another column", "SELECT * EXCEPT (v) FROM t", true},
		{"qualified star except the column", "SELECT t.* EXCEPT (v, ts) FROM t", false},
		{"star except strict the column", "SELECT * EXCEPT STRICT ts FROM t", false},
		{"star except strict a list with the column", "SELECT * EXCEPT STRICT (ts, v) FROM t", false},
		{"star except strict another column", "SELECT * EXCEPT STRICT (v) FROM t", true},
		{"database-qualified star", "SELECT db.t.* FROM db.t", true},
		{"database-qualified star except the column", "SELECT db.t.* EXCEPT (ts) FROM db.t", false},
		{"star except a regex matching the column", "SELECT * EXCEPT ('^t') FROM t", false},
		{"star except a regex matching part of the column", "SELECT * EXCEPT ('s') FROM t", false},
		{"star except a regex not matching the column", "SELECT * EXCEPT ('^time') FROM t", true},
		{"star replace over the column", "SELECT * REPLACE (ts + 1 AS ts) FROM t", true},
		{"star replace with another column", "SELECT * REPLACE (created AS ts) FROM t", false},
		{"renamed away", "SELECT ts AS time FROM t", false},
		{"dropped", "SELECT v FROM t", false},
		{"only inside an expression", "SELECT toDate(ts) FROM t", false},
		{"different column", "SELECT tsx FROM t", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := sourceOf(t, "SELECT $__timeFilter() FROM ("+tt.body+")")
			require.NoError(t, source.Err)
			require.Len(t, source.layers, 1)
			assert.Equal(t, tt.want, source.layers[0].exposes("ts"))
		})
	}
}

func TestSource_OuterJoinResolvesLeftTable(t *testing.T) {
	// Only alias and subquery bodies must have a single source. A JOIN in the
	// macro's own FROM takes the primary key from its left table.
	source := sourceOf(t, "SELECT $__timeFilter() FROM sample.logs AS l JOIN sample.users AS u ON l.k = u.k")
	require.NoError(t, source.Err)
	assert.Equal(t, "sample", source.Database)
	assert.Equal(t, "logs", source.Table)
	assert.Empty(t, source.layers)
}

func TestSource_AliasBodyReadsSameNamedTable(t *testing.T) {
	// Inside its own body an alias's name is the real table (ClickHouse).
	source := sourceOf(t, "WITH logs AS (SELECT * FROM logs WHERE v = 1) SELECT $__timeFilter() FROM logs")
	require.NoError(t, source.Err)
	assert.Equal(t, "", source.Database)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`logs`"}, layerNames(source))
}

func TestSource_MacroInsideAliasBodyReadsSameNamedTable(t *testing.T) {
	source := sourceOf(t, "WITH logs AS (SELECT * FROM logs WHERE $__timeFilter()) SELECT count() FROM logs")
	require.NoError(t, source.Err)
	assert.Equal(t, "logs", source.Table)
	assert.Empty(t, source.layers)
}

func TestSource_OwnNameHidesOuterSameNamedAlias(t *testing.T) {
	// The body's `logs` skips the outer alias of the same name too.
	source := sourceOf(t, "WITH logs AS (SELECT * FROM db.u) SELECT * FROM "+
		"(WITH logs AS (SELECT * FROM logs) SELECT $__timeFilter() FROM logs)")
	require.NoError(t, source.Err)
	assert.Equal(t, "", source.Database)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`logs`"}, layerNames(source))
}

func TestSource_AliasMayReadLaterSibling(t *testing.T) {
	source := sourceOf(t, "WITH a AS (SELECT * FROM b), b AS (SELECT * FROM sample.logs) SELECT $__timeFilter() FROM a")
	require.NoError(t, source.Err)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`a`", "`b`"}, layerNames(source))
}

func TestSource_TooDeepIsItsOwnError(t *testing.T) {
	var with strings.Builder
	with.WriteString("WITH a0 AS (SELECT * FROM sample.logs)")
	for level := 1; level <= 40; level++ {
		fmt.Fprintf(&with, ", a%d AS (SELECT * FROM a%d)", level, level-1)
	}
	source := sourceOf(t, with.String()+" SELECT $__timeFilter() FROM a40")
	require.ErrorIs(t, source.Err, ErrSourceTooDeep)
	assert.NotErrorIs(t, source.Err, ErrNoSingleSource)
}

func TestSource_OuterJoinWithAliasOrSubqueryOnLeft(t *testing.T) {
	tests := []struct {
		name, from string
		layers     []string
	}{
		{"join", "e JOIN t ON e.k = t.k", []string{"`e`"}},
		{"aliased join", "e AS l JOIN t ON l.k = t.k", []string{"`e`"}},
		{"array join", "e ARRAY JOIN arr", []string{"`e`"}},
		{"comma join", "e, t", []string{"`e`"}},
		{"subquery on the left", "(SELECT * FROM sample.logs) AS s JOIN t ON s.k = t.k", []string{"subquery"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := sourceOf(t, "WITH e AS (SELECT * FROM sample.logs) SELECT $__timeFilter() FROM "+tt.from)
			require.NoError(t, source.Err)
			assert.Equal(t, "sample", source.Database)
			assert.Equal(t, "logs", source.Table)
			assert.Equal(t, tt.layers, layerNames(source))
		})
	}
}

func TestSource_AliasBodySeesOnlyItsDefiningScope(t *testing.T) {
	// y is defined in the outer scope, so the x it reads is the outer x even
	// though the macro's own scope defines another x.
	source := sourceOf(t, "WITH x AS (SELECT * FROM db.outer_tbl), y AS (SELECT * FROM x) SELECT * FROM "+
		"(WITH x AS (SELECT * FROM db.inner_tbl) SELECT $__timeFilter() FROM y)")
	require.NoError(t, source.Err)
	assert.Equal(t, "outer_tbl", source.Table)
	assert.Equal(t, []string{"`y`", "`x`"}, layerNames(source))
}

func TestSource_ResolvingDoesNotDisturbLaterLookups(t *testing.T) {
	// The IN subquery is visited after the macro's SELECT resolved y; the
	// resolution must not have overwritten the scope stack it shares.
	source := sourceOf(t, "WITH x AS (SELECT * FROM db.t) SELECT * FROM "+
		"(WITH y AS (SELECT * FROM db.u) SELECT $__timeFilter() FROM y WHERE k IN (SELECT k FROM x))")
	require.NoError(t, source.Err)
	assert.Equal(t, "db", source.Database)
	assert.Equal(t, "u", source.Table)
	assert.Equal(t, []string{"`y`"}, layerNames(source))
}

func TestSource_QualifiedTableIsNeverAnAlias(t *testing.T) {
	source := sourceOf(t, "WITH e AS (SELECT * FROM db.other) SELECT $__timeFilter() FROM sample.e")
	require.NoError(t, source.Err)
	assert.Equal(t, "sample", source.Database)
	assert.Equal(t, "e", source.Table)
	assert.Empty(t, source.layers)
}

func TestSource_MacrosInUnionBranchesResolveSeparately(t *testing.T) {
	exprs, err := parser.NewParser("WITH a AS (SELECT * FROM db.t1), b AS (SELECT * FROM db.t2) " +
		"SELECT $__timeFilter() FROM a UNION ALL SELECT $__timeFilter_ms() FROM b").ParseStmts()
	require.NoError(t, err)
	got, err := GetMacroCTEs(exprs)
	require.NoError(t, err)
	tables := map[string]string{}
	for id, entry := range got {
		require.NoError(t, entry.Source.Err)
		tables[id.Name] = entry.Source.Table
	}
	assert.Equal(t, map[string]string{"$__timeFilter": "t1", "$__timeFilter_ms": "t2"}, tables)
}

func TestCTE_SourceIsNotSerialized(t *testing.T) {
	// /macroCTE serves CTE as JSON; Source is internal and must stay out.
	exprs, err := parser.NewParser("WITH e AS (SELECT * FROM sample.logs) SELECT $__timeFilter() FROM e").ParseStmts()
	require.NoError(t, err)
	got, err := GetMacroCTEs(exprs)
	require.NoError(t, err)
	for _, entry := range got {
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		var keys map[string]any
		require.NoError(t, json.Unmarshal(raw, &keys))
		assert.ElementsMatch(t, []string{"macro", "macroPos", "cte", "table", "database", "pos"}, slices.Collect(maps.Keys(keys)))
	}
}

func TestSource_RedefinitionInsideOwnBodyStillMeansTheTable(t *testing.T) {
	// ClickHouse ignores a redefinition of an alias's name inside its own
	// body too: the inner `logs` here is the real table (24.8).
	source := sourceOf(t, "WITH logs AS (WITH logs AS (SELECT * FROM sample.inner_t) SELECT * FROM logs) "+
		"SELECT $__timeFilter() FROM logs")
	require.NoError(t, source.Err)
	assert.Equal(t, "", source.Database)
	assert.Equal(t, "logs", source.Table)
	assert.Equal(t, []string{"`logs`"}, layerNames(source))
}
