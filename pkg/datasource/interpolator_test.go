package datasource

import (
	"context"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"maps"
	"slices"
	"testing"
	"time"
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
		{input: "SELECT * FROM (Select * from table2 where 1=1) WHERE $__macro()", result: "(SELECT * FROM table2 WHERE 1 = 1)", name: "should return the subquery for filter"},
		{input: "SELECT * FROM (Select * from table2 where l in (select * from table2)) WHERE $__macro()", result: "(SELECT * FROM table2 WHERE l IN (SELECT * FROM table2))", name: "should return subqueries for filter"},
		{input: "WITH\n  top_50_reqPath AS (\n    SELECT\n      topK (50) (reqPath)\n    FROM\n      table\n    WHERE\n  $__macro() \n  )\nSELECT\n  *\nFROM\n  top_50_reqPath", result: "table", name: "should return the table and ignore with alias for filter"},

		{input: "SELECT $__macro() FROM table WHERE 1=1", result: "table", name: "should return the table for value"},
		{input: "SELECT $__macro() FROM schema.table WHERE 1=1", result: "schema.table", name: "should return the table with schema for value"},
		{input: "SELECT $__macro() FROM schema.table as t1 WHERE 1=1", result: "schema.table AS t1", name: "should return the table and schema with alias for value"},
		{input: "SELECT $__macro() FROM (Select * from table2 where 1=1) WHERE 1=1", result: "(SELECT * FROM table2 WHERE 1 = 1)", name: "should return the subquery for value"},
		{input: "SELECT $__macro() FROM (Select * from table2 where l in (select * from table2)) WHERE 1=1", result: "(SELECT * FROM table2 WHERE l IN (SELECT * FROM table2))", name: "should return subqueries for value"},
		{input: "WITH\n  top_50_reqPath AS (\n    SELECT\n      $__macro()\n    FROM\n      table\n    WHERE\n  1=1\n  )\nSELECT\n  *\nFROM\n  top_50_reqPath", result: "table", name: "should return the table and ignore with alias for value"},
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
	expected := []string{"logs AS subquery", "logs AS subquery", "akamai.logs AS main_query", "akamai.logs AS main_query", "akamai.logs AS subquery", "akamai.logs AS subquery"}
	sql := "SELECT\n  main_query.reqTimeSec,\n  (\n    SELECT COUNT(*)\n    FROM logs AS subquery\n    WHERE $__timeFilter(reqTimeSec) AND $__adHocFilter() \n  )\nFROM\n  akamai.logs AS main_query\nWHERE\n$__timeFilter(reqTimeSec) AND $__adHocFilter() AND\n  reqId IN (\n    SELECT\n      reqId\n    FROM\n      akamai.logs AS subquery\n    WHERE\n      statusCode = 404\n      AND reqMethod = 'GET'\n      AND $__timeFilter(reqTimeSec) AND $__adHocFilter() \n  );"
	expr, _ := parser.NewParser(sql).ParseStmts()
	res, err := GetMacroCTEs(expr)
	require.NoError(t, err)
	fmt.Println(res)
	for i, v := range slices.SortedFunc(maps.Values(res), func(a, b CTE) int { return int(a.MacroPos) - int(b.MacroPos) }) {
		assert.Equal(t, expected[i], v.CTE, fmt.Sprintf("For macro %s at index %d", v.Macro, v.MacroPos))
	}
}

func TestGetMacroMatches(t *testing.T) {
	type test struct {
		name     string
		input    string
		macro    string
		expected []macroMatch
	}

	tests := []test{
		{
			name:  "should match unescaped macro",
			input: "SELECT * FROM table WHERE $__timeFilter(timestamp)",
			macro: "timeFilter",
			expected: []macroMatch{
				{
					full:    "$__timeFilter(timestamp)",
					name:    "timeFilter",
					args:    []string{"timestamp"},
					escaped: false,
					pos:     parser.Pos(26),
				},
			},
		},
		{
			name:  "should match escaped macro with double dollar sign",
			input: "SELECT * FROM table WHERE $$__timeFilter(timestamp)",
			macro: "timeFilter",
			expected: []macroMatch{
				{
					full:    "$$__timeFilter(timestamp)",
					name:    "timeFilter",
					args:    []string{"timestamp"},
					escaped: true,
					pos:     parser.Pos(26),
				},
			},
		},
		{
			name:  "should match multiple unescaped macros",
			input: "SELECT $__timeInterval(value) FROM table WHERE $__timeFilter(timestamp)",
			macro: "timeFilter",
			expected: []macroMatch{
				{
					full:    "$__timeFilter(timestamp)",
					name:    "timeFilter",
					args:    []string{"timestamp"},
					escaped: false,
					pos:     parser.Pos(47),
				},
			},
		},
		{
			name:  "should match macro with no arguments",
			input: "SELECT * FROM table WHERE $__adHocFilter()",
			macro: "adHocFilter",
			expected: []macroMatch{
				{
					full:    "$__adHocFilter()",
					name:    "adHocFilter",
					args:    []string{""},
					escaped: false,
					pos:     parser.Pos(26),
				},
			},
		},
		{
			name:  "should match escaped macro with no arguments",
			input: "SELECT * FROM table WHERE $$__adHocFilter()",
			macro: "adHocFilter",
			expected: []macroMatch{
				{
					full:    "$$__adHocFilter()",
					name:    "adHocFilter",
					args:    []string{""},
					escaped: true,
					pos:     parser.Pos(26),
				},
			},
		},
		{
			name:  "should match macro with multiple arguments",
			input: "SELECT * FROM table WHERE $__dateTimeFilter(timestamp, created_at)",
			macro: "dateTimeFilter",
			expected: []macroMatch{
				{
					full:    "$__dateTimeFilter(timestamp, created_at)",
					name:    "dateTimeFilter",
					args:    []string{"timestamp", "created_at"},
					escaped: false,
					pos:     parser.Pos(26),
				},
			},
		},
		{
			name:  "should match escaped macro with multiple arguments",
			input: "SELECT * FROM table WHERE $$__dateTimeFilter(timestamp, created_at)",
			macro: "dateTimeFilter",
			expected: []macroMatch{
				{
					full:    "$$__dateTimeFilter(timestamp, created_at)",
					name:    "dateTimeFilter",
					args:    []string{"timestamp", "created_at"},
					escaped: true,
					pos:     parser.Pos(26),
				},
			},
		},
		{
			name:     "should not match macro without dollar sign",
			input:    "SELECT * FROM table WHERE __timeFilter(timestamp)",
			macro:    "timeFilter",
			expected: []macroMatch{},
		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			matches, err := getMacroMatches(tc.input, tc.macro, nil)
			require.NoError(t, err)
			assert.Equal(t, len(tc.expected), len(matches))
			for j, expected := range tc.expected {
				assert.Equal(t, expected.full, matches[j].full, "full macro text should match")
				assert.Equal(t, expected.name, matches[j].name, "macro name should match")
				assert.Equal(t, expected.args, matches[j].args, "macro args should match")
				assert.Equal(t, expected.escaped, matches[j].escaped, "escaped flag should match")
				assert.Equal(t, expected.pos, matches[j].pos, "position should match")
			}
		})
	}
}

func TestGetMacroMatches_ErrorCases(t *testing.T) {
	type test struct {
		name      string
		input     string
		macro     string
		expectErr bool
	}

	tests := []test{
		{
			name:      "should return error for unclosed macro",
			input:     "SELECT * FROM table WHERE $__timeFilter(timestamp",
			macro:     "timeFilter",
			expectErr: true,
		},
		{
			name:      "should return error for unclosed nested parenthesis",
			input:     "SELECT * FROM table WHERE $__macro(func(arg)",
			macro:     "macro",
			expectErr: true,
		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			_, err := getMacroMatches(tc.input, tc.macro, nil)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInterpolateMacroEscaping(t *testing.T) {
	type test struct {
		name   string
		input  string
		output string
	}

	tests := []test{
		{
			name:   "should escape macro with double dollar sign",
			input:  "SELECT * FROM table WHERE $$__timeFilter(timestamp)",
			output: "SELECT * FROM table WHERE $__timeFilter(timestamp)",
		},
		{
			name:   "should escape multiple macros",
			input:  "SELECT $$__timeInterval(value) FROM table WHERE $$__timeFilter(timestamp)",
			output: "SELECT $__timeInterval(value) FROM table WHERE $__timeFilter(timestamp)",
		},
		{
			name:   "should handle mix of escaped and unescaped macros",
			input:  "SELECT * FROM table WHERE $__timeFilter(timestamp) AND $$__adHocFilter()",
			output: "SELECT * FROM table WHERE timestamp >= toDateTime(1415792726) AND timestamp <= toDateTime(1447328726) AND $__adHocFilter()",
		},
		{
			name:   "should escape macro with no arguments",
			input:  "SELECT * FROM table WHERE $$__adHocFilter()",
			output: "SELECT * FROM table WHERE $__adHocFilter()",
		},
		{
			name:   "should escape macro with multiple arguments",
			input:  "SELECT * FROM table WHERE $$__dateTimeFilter(timestamp, created_at)",
			output: "SELECT * FROM table WHERE $__dateTimeFilter(timestamp, created_at)",
		},
		{
			name:   "should process unescaped macro normally",
			input:  "SELECT * FROM table WHERE $__fromTime",
			output: "SELECT * FROM table WHERE toDateTime(1415792726)",
		},
		{
			name:   "should escape fromTime and toTime macros",
			input:  "SELECT * FROM table WHERE $$__fromTime AND $$__toTime",
			output: "SELECT * FROM table WHERE $__fromTime AND $__toTime",
		},
		{
			name:   "should handle escaped timeFilter_ms",
			input:  "SELECT * FROM table WHERE $$__timeFilter_ms(timestamp)",
			output: "SELECT * FROM table WHERE $__timeFilter_ms(timestamp)",
		},
		{
			name:   "should handle complex query with mix of escaped and unescaped",
			input:  "SELECT * FROM table WHERE $__timeFilter(timestamp) AND status = 'active' OR $$__timeFilter_ms(created_at)",
			output: "SELECT * FROM table WHERE timestamp >= toDateTime(1415792726) AND timestamp <= toDateTime(1447328726) AND status = 'active' OR $__timeFilter_ms(created_at)",
		},
		{
			name:   "should handle multiple escaped macros in different positions",
			input:  "SELECT $$__fromTime, $$__toTime, $__interval_s FROM table WHERE $$__timeFilter(timestamp)",
			output: "SELECT $__fromTime, $__toTime, 1 FROM table WHERE $__timeFilter(timestamp)",
		},
		{
			name:   "should handle macros escaped multiple times",
			input:  "SELECT * FROM table WHERE $$$$__timeFilter(timestamp)",
			output: "SELECT * FROM table WHERE $$$__timeFilter(timestamp)",
		},
		{
			name:   "should handle macros escaped multiple times",
			input:  "SELECT * FROM table WHERE $$$__timeFilter(timestamp)",
			output: "SELECT * FROM table WHERE $$__timeFilter(timestamp)",
		},
	}

	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.123Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.456Z")

	for i, tc := range tests {
		interpolator := NewInterpolator(&HydrolixDatasource{
			Connector: &MockConnector{
				uid: "uid-123",
			},
		})
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			query := &HDXQuery{
				RawSQL: tc.input,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				Interval: time.Duration(1000000000),
			}
			interpolatedQuery, err := interpolator.Interpolate(query, context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.output, interpolatedQuery)
		})
	}
}

func TestInterpolateMacroInStringLiteral(t *testing.T) {
	type test struct {
		name   string
		input  string
		output string
	}

	tests := []test{
		{
			name:   "should not interpolate macro inside string literal",
			input:  "SELECT '$__fromTime' FROM table",
			output: "SELECT '$__fromTime' FROM table",
		},
		{
			name:   "should not interpolate macro with brackets inside string literal",
			input:  "SELECT '$__timeFilter(timestamp)' FROM table",
			output: "SELECT '$__timeFilter(timestamp)' FROM table",
		},
		{
			name:   "should interpolate real macro but not one inside string literal",
			input:  "SELECT '$__fromTime' FROM table WHERE $__fromTime",
			output: "SELECT '$__fromTime' FROM table WHERE toDateTime(1415792726)",
		},
		{
			name:   "should interpolate real macro but not one inside string literal with brackets",
			input:  "SELECT '$__timeFilter(timestamp)' FROM table WHERE $__timeFilter(timestamp)",
			output: "SELECT '$__timeFilter(timestamp)' FROM table WHERE timestamp >= toDateTime(1415792726) AND timestamp <= toDateTime(1447328726)",
		},
		{
			name:   "should not interpolate macro inside line comment",
			input:  "SELECT * FROM table -- $__fromTime",
			output: "SELECT * FROM table -- $__fromTime",
		},
		{
			name:   "should not interpolate macro with brackets inside line comment",
			input:  "SELECT * FROM table -- $__timeFilter(timestamp)",
			output: "SELECT * FROM table -- $__timeFilter(timestamp)",
		},
		{
			name:   "should not interpolate macro inside block comment",
			input:  "SELECT * FROM table /* $__fromTime */",
			output: "SELECT * FROM table /* $__fromTime */",
		},
		{
			name:   "should interpolate real macro but not one inside line comment",
			input:  "SELECT * FROM table WHERE $__fromTime -- $__fromTime",
			output: "SELECT * FROM table WHERE toDateTime(1415792726) -- $__fromTime",
		},
		{
			name:   "should interpolate real macro but not one inside block comment",
			input:  "SELECT * FROM table WHERE $__fromTime /* $__fromTime */",
			output: "SELECT * FROM table WHERE toDateTime(1415792726) /* $__fromTime */",
		},
	}

	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.123Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.456Z")

	for i, tc := range tests {
		interpolator := NewInterpolator(&HydrolixDatasource{
			Connector: &MockConnector{
				uid: "uid-123",
			},
		})
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			query := &HDXQuery{
				RawSQL: tc.input,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
				Interval: time.Duration(1000000000),
			}
			interpolatedQuery, err := interpolator.Interpolate(query, context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.output, interpolatedQuery)
		})
	}
}
