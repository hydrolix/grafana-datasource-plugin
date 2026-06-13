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
