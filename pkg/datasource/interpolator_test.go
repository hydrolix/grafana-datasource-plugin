package datasource

import (
	"fmt"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"maps"
	"slices"
	"testing"
)

func TestGetMacroCTEs(t *testing.T) {
	type test struct {
		name   string
		input  string
		result string
	}

	tests := []test{
		{input: "SELECT * FROM table WHERE $__macro()", result: "table", name: "should return the table for filter"},
		{input: "SELECT * FROM schema.table WHERE $__macro()", result: "schema.table", name: "should return the table with schema for filter"},
		{input: "SELECT * FROM schema.table as t1 WHERE $__macro()", result: "schema.table AS t1", name: "should return the table and schema with alias for filter"},
		{input: "SELECT * FROM (Select * from table2 where 1=1) WHERE $__macro()", result: "(SELECT * FROM table2 WHERE 1 = 1)", name: "should return the table and schema with alias for filter"},
		{input: "SELECT * FROM (Select * from table2 where l in (select * from table2)) WHERE $__macro()", result: "(SELECT * FROM table2 WHERE l IN (SELECT * FROM table2))", name: "should return the table and schema with alias for filter"},

		{input: "SELECT $__macro() FROM table WHERE 1=1", result: "table", name: "should return the table for value"},
		{input: "SELECT $__macro() FROM schema.table WHERE 1=1", result: "schema.table", name: "should return the table with schema for value"},
		{input: "SELECT $__macro() FROM schema.table as t1 WHERE 1=1", result: "schema.table AS t1", name: "should return the table and schema with alias for value"},
		{input: "SELECT $__macro() FROM (Select * from table2 where 1=1) WHERE 1=1", result: "(SELECT * FROM table2 WHERE 1 = 1)", name: "should return the table and schema with alias for value"},
		{input: "SELECT $__macro() FROM (Select * from table2 where l in (select * from table2)) WHERE 1=1", result: "(SELECT * FROM table2 WHERE l IN (SELECT * FROM table2))", name: "should return the table and schema with alias for value"},
	}

	for i, tc := range tests {

		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			expr, _ := parser.NewParser(tc.input).ParseStmts()
			res, err := GetMacroCTEs(expr)
			require.NoError(t, err)
			fmt.Println(res)
			assert.Equal(t, len(res), 1)
			require.Nil(t, err)
			v := slices.Collect(maps.Values(res))[0]
			assert.Equal(t, tc.result, v.CTE)
		})
	}

}

func TestGetMacroCTEsForComplexQuery(t *testing.T) {
	expected := []string{"akamai.logs AS main_query", "akamai.logs AS main_query", "akamai.logs AS main_query", "akamai.logs AS main_query", "akamai.logs AS subquery", "akamai.logs AS subquery"}
	sql := "SELECT\n  main_query.reqTimeSec,\n  (\n    SELECT COUNT(*)\n    FROM logs AS subquery\n    WHERE $__timeFilter(reqTimeSec) AND $__adHocFilter() \n  )\nFROM\n  akamai.logs AS main_query\nWHERE\n$__timeFilter(reqTimeSec) AND $__adHocFilter() AND\n  reqId IN (\n    SELECT\n      reqId\n    FROM\n      akamai.logs AS subquery\n    WHERE\n      statusCode = 404\n      AND reqMethod = 'GET'\n      AND $__timeFilter(reqTimeSec) AND $__adHocFilter() \n  );"
	expr, _ := parser.NewParser(sql).ParseStmts()
	res, err := GetMacroCTEs(expr)
	require.NoError(t, err)
	fmt.Println(res)

	for i, v := range slices.SortedFunc(maps.Values(res), func(a, b CTE) int { return int(a.MacroPos) - int(b.MacroPos) }) {
		assert.Equal(t, expected[i], v.CTE, fmt.Sprintf("For macro %s at index %d", v.Macro, v.MacroPos))
	}
}
