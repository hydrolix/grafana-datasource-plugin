package cte

import (
	"testing"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/stretchr/testify/assert"
)

func TestGetMacroCTEs_NoMacros(t *testing.T) {
	exprs, err := parser.NewParser("SELECT 1 FROM t").ParseStmts()
	assert.NoError(t, err)
	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestGetMacroCTEs_SimpleMacro(t *testing.T) {
	sql := "SELECT $__timeFilter FROM mydb.events"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 1)

	for id, c := range got {
		assert.Equal(t, "$__timeFilter", id.Name)
		assert.Equal(t, "$__timeFilter", c.Macro)
		assert.Equal(t, "events", c.Table)
		assert.Equal(t, "mydb", c.Database)
	}
}

func TestGetMacroCTEs_TableWithoutDatabase(t *testing.T) {
	sql := "SELECT $__timeFilter FROM events"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	for _, c := range got {
		assert.Equal(t, "events", c.Table)
		assert.Equal(t, "", c.Database)
	}
}

func TestGetMacroCTEs_MultipleMacros(t *testing.T) {
	sql := "SELECT $__timeFilter, $__adHocFilter FROM events"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 2)

	names := make(map[string]bool)
	for id := range got {
		names[id.Name] = true
	}
	assert.True(t, names["$__timeFilter"])
	assert.True(t, names["$__adHocFilter"])
}

func TestMacroPositions_FindsAllSites(t *testing.T) {
	sql := "SELECT $__timeFilter, $__dateFilter FROM events"
	positions, err := MacroPositions(sql)
	assert.NoError(t, err)
	assert.Len(t, positions, 2)
}

func TestMacroPositions_NoMacros(t *testing.T) {
	positions, err := MacroPositions("SELECT * FROM events")
	assert.NoError(t, err)
	assert.Empty(t, positions)
}

func TestMacroPositions_InvalidSQL(t *testing.T) {
	_, err := MacroPositions("SELEC FRO :: bad")
	assert.Error(t, err)
}

func TestGetMacroCTEs_WithAliasResolvesToSubquery(t *testing.T) {
	sql := "WITH x AS (SELECT a FROM events) SELECT $__adHocFilter() FROM x"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	for _, c := range got {
		assert.Equal(t, "(SELECT a FROM events)", c.CTE, "WITH alias must resolve to its subquery, not the bare name")
	}
}

func TestGetMacroCTEs_NonAliasIdentifierStaysTable(t *testing.T) {
	// FROM y matches no WITH alias, so it stays an identifier.
	sql := "WITH x AS (SELECT a FROM events) SELECT $__adHocFilter() FROM y"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	for _, c := range got {
		assert.Equal(t, "y", c.CTE)
	}
}

func TestGetMacroCTEs_InlineSubqueryUnchanged(t *testing.T) {
	sql := "SELECT $__adHocFilter() FROM (SELECT a FROM events)"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	for _, c := range got {
		assert.Contains(t, c.CTE, "events")
		assert.Contains(t, c.CTE, "SELECT")
	}
}

func TestGetMacroCTEs_ShadowedAliasResolvesNearestScope(t *testing.T) {
	sql := "WITH x AS (SELECT o FROM outer_tbl) SELECT * FROM " +
		"(WITH x AS (SELECT i FROM inner_tbl) SELECT $__adHocFilter() FROM x)"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	for _, c := range got {
		assert.Contains(t, c.CTE, "inner_tbl", "nearest-scope alias must win")
		assert.NotContains(t, c.CTE, "outer_tbl")
	}
}

func TestGetMacroCTEs_AliasShadowsTableName(t *testing.T) {
	// The WITH alias is named the same as a real table; the in-scope alias
	// must win, so the CTE resolves to the subquery, not the table.
	sql := "WITH events AS (SELECT a FROM real_events) SELECT $__adHocFilter() FROM events"
	exprs, err := parser.NewParser(sql).ParseStmts()
	assert.NoError(t, err)

	got, err := GetMacroCTEs(exprs)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	for _, c := range got {
		assert.Equal(t, "(SELECT a FROM real_events)", c.CTE, "in-scope WITH alias must win over a same-named table")
		assert.NotEqual(t, "events", c.CTE)
	}
}
