package parser

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestParser_Compatible(t *testing.T) {
	dir := "./testdata/query/compatible/1_stateful"
	entries, err := os.ReadDir(dir)
	if err != nil {
		require.NoError(t, err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			fileBytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			require.NoError(t, err)
			parser := Parser{
				lexer: NewLexer(string(fileBytes)),
			}
			_, err = parser.ParseStmts()
			require.NoError(t, err)
		})
	}
}

func TestParser_ParseStatements(t *testing.T) {
	for _, dir := range []string{"./testdata/dml", "./testdata/ddl", "./testdata/query", "./testdata/basic"} {
		outputDir := dir + "/output"
		entries, err := os.ReadDir(dir)
		if err != nil {
			require.NoError(t, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			t.Run(entry.Name(), func(t *testing.T) {
				fileBytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				require.NoError(t, err)
				parser := Parser{
					lexer: NewLexer(string(fileBytes)),
				}
				stmts, err := parser.ParseStmts()
				require.NoError(t, err)
				outputBytes, _ := json.MarshalIndent(stmts, "", "  ")
				g := goldie.New(t,
					goldie.WithNameSuffix(".golden.json"),
					goldie.WithDiffEngine(goldie.ClassicDiff),
					goldie.WithFixtureDir(outputDir))
				g.Assert(t, entry.Name(), outputBytes)
			})
		}
	}
}

func TestParser_Format(t *testing.T) {
	for _, dir := range []string{"./testdata/dml", "./testdata/ddl", "./testdata/query", "./testdata/basic"} {
		outputDir := dir + "/format"

		entries, err := os.ReadDir(dir)
		if err != nil {
			require.NoError(t, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			t.Run(entry.Name(), func(t *testing.T) {
				fileBytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				require.NoError(t, err)
				parser := Parser{
					lexer: NewLexer(string(fileBytes)),
				}
				stmts, err := parser.ParseStmts()
				require.NoError(t, err)
				var builder strings.Builder
				builder.WriteString("-- Origin SQL:\n")
				builder.Write(fileBytes)
				builder.WriteString("\n\n-- Format SQL:\n")
				var formatSQLBuilder strings.Builder
				for _, stmt := range stmts {
					formatSQLBuilder.WriteString(stmt.String())
					formatSQLBuilder.WriteByte(';')
					formatSQLBuilder.WriteByte('\n')
				}
				formatSQL := formatSQLBuilder.String()
				builder.WriteString(formatSQL)
				validFormatSQL(t, formatSQL)
				g := goldie.New(t,
					goldie.WithNameSuffix(""),
					goldie.WithDiffEngine(goldie.ColoredDiff),
					goldie.WithFixtureDir(outputDir))
				g.Assert(t, entry.Name(), []byte(builder.String()))
			})
		}
	}
}

// validFormatSQL Verify that the format sql can be re-parsed with consistent results
func validFormatSQL(t *testing.T, sql string) {
	parser := NewParser(sql)
	stmts, err := parser.ParseStmts()
	require.NoError(t, err)
	var builder strings.Builder
	for _, stmt := range stmts {
		builder.WriteString(stmt.String())
		builder.WriteByte(';')
		builder.WriteByte('\n')
	}
	require.Equal(t, sql, builder.String())
}

func TestParser_InvalidSyntax(t *testing.T) {
	invalidSQLs := []string{
		"SELECT * FROM",
	}
	for _, sql := range invalidSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.Error(t, err)
	}
}

func TestParser_ConditionALL_With_Variables(t *testing.T) {
	validSQLs := []string{
		//"SELECT 1 FROM table WHERE statusCode ${a} (1,2)",
		//"SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(${timefilter}) as time, ${count} as http FROM ${table} WHERE $__timeFilter(${timefilter}) AND $__conditionalAll( statusCode IN (${statusCode:sqlstring}), $statusCode)",
		"SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(${timefilter}) as time, ${count} as http FROM ${table} WHERE $__timeFilter(${timefilter}) AND $__conditionalAll( statusCode ${AND_statusCode} (${statusCode:sqlstring}), $statusCode)",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		expr, err := parser.ParseStmts()
		marshal, err := json.Marshal(expr)
		fmt.Printf("%s", marshal)
		require.NoError(t, err)
	}
}

type selectQueryVisitor struct {
	DefaultASTVisitor
	Start int
	End   int
}

func (v *selectQueryVisitor) VisitTableExpr(expr *TableExpr) error {
	if strings.HasPrefix(expr.String(), "(") {
		v.Start = int(expr.Pos())
		v.End = int(expr.End())
	}

	return nil
}

func TestParser_With_SubSelect(t *testing.T) {
	validSQLs := map[string]string{
		"SELECT\n  bucket,\n  count()\nFROM\n  (\n    SELECT\n      toStartOfInterval(${timestamp}, INTERVAL 1 hour) AS bucket\n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})\n      AND $__adHocFilter()\n  )\nWHERE\n  $__adHocFilter()\nGROUP BY\n  bucket":               ")",
		"SELECT\n  bucket,\n  count()\nFROM\n  (\n    SELECT\n      toStartOfInterval(${timestamp}, INTERVAL 1 hour) AS bucket\n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})\n      AND $__adHocFilter()\n  ) as `alias1` \nWHERE\n  $__adHocFilter()\nGROUP BY\n  bucket":  "alias1",
		"SELECT\n  bucket,\n  count()\nFROM\n  (\n    SELECT\n      toStartOfInterval(${timestamp}, INTERVAL 1 hour) AS bucket\n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})\n      AND $__adHocFilter()\n  ) as `alias 2` \nWHERE\n  $__adHocFilter()\nGROUP BY\n  bucket": "alias 2",
	}
	for sql, suffix := range validSQLs {
		println(sql)
		parser := NewParser(sql)
		expr, err := parser.ParseStmts()
		visitor := selectQueryVisitor{}
		err = expr[0].Accept(&visitor)
		require.NoError(t, err)
		require.NotNil(t, visitor.Start)
		require.NotNil(t, visitor.End)
		println(sql[visitor.Start:visitor.End])
		require.True(t, strings.HasSuffix(strings.TrimSpace(sql[visitor.Start:visitor.End]), suffix))

	}
}

func TestParser_Dashboard_Queries(t *testing.T) {
	t.Skip() //skip test
	fail := 0
	success := 0
	err := filepath.WalkDir("path to folder with sql files", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			t.Fail()
		}

		if strings.HasSuffix(path, ".sql") {
			t.Run(path, func(t *testing.T) {
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				parser := NewParser(string(content))
				println(string(content))
				_, err = parser.ParseStmts()
				if err != nil {
					fail++
				} else {
					success++
				}
				require.NoError(t, err)

			})
		}
		return nil
	})
	require.NoError(t, err)
	println("success", success)
	println("fail", fail)
}

func TestParser_Temp(t *testing.T) {
	t.Skip() //skip test
	//sql := "SELECT uniq(sessionId) as sessions\nFROM ${mpulse}\nwhere $__timeFilter(timestamp)\nAND if (empty ('${paramsU}'),true,paramsU like '${paramsU}')\nAND $__conditionalAll(pageDomainName in (${pageDomainName:sqlstring}),${pageDomainName})\n${filter}\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment='db=${__dashboard}; dp=Sessions; du=${__user.login}'"
	//sql := "SELECT if('${column_type}' LIKE 'Map%', 'mapContains(${column_name}, \\'${values}\\')', '${column_name} ${evaluation} \\'${values}\\'')"
	//sql := "SELECT statusCode::String, * EXCEPT statusCode\nFROM ${table}\nWHERE $__timeFilter(reqTimeSec)"
	//sql := "SELECT ${aggregation}(cumulativeLayoutShift) as timer,\n$__timeInterval(timestamp) as time\nFROM ${mpulse}\nwhere $__timeFilter(timestamp)\nAND if (empty ('${paramsU}'),true,paramsU like '${paramsU}')\nAND $__conditionalAll(beaconTypeName in (${beaconTypeName:sqlstring}),${beaconTypeName})\nAND $__conditionalAll(pageDomainName in (${pageDomainName:sqlstring}),${pageDomainName})\n${filter}\ngroup by time\norder by time asc\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment='db=${__dashboard}; dp=CLS; du=${__user.login}'"
	sql := "SELECT toString(${group_by}) as grouping, count() as \"Total API Requests\", toStartOfFifteenMinutes(timestamp) as min\nFROM integrations.cloudflare\nWHERE $__timeFilter(timestamp) \n  AND response_content_type = 'application/json'\n  AND response_status_code = 200\nGROUP BY min, grouping\nORDER BY grouping, min ASC WITH FILL FROM fromUnixTimestamp(${__from:date:seconds}) TO fromUnixTimestamp(${__to:date:seconds}) STEP toIntervalMinute(15)\nSETTINGS hdx_query_max_execution_time=60"
	//sql := "DESCRIBE ${table}"
	//sql := "WITH (\n    WITH 1000 AS LATEST\n    SELECT [min(min_timestamp), max(max_timestamp)] AS min_max\n    FROM (\n        SELECT\n            *,\n            any(running_row_count) OVER (ORDER BY min_timestamp DESC ROWS BETWEEN 1 PRECEDING AND 1 PRECEDING) AS prev_running_row_count,\n            (running_row_count > LATEST) AND (prev_running_row_count < LATEST) AS is_boundary_row\n        FROM (\n            SELECT\n                min_timestamp,\n                max_timestamp,\n                sum(rows) OVER (ORDER BY min_timestamp DESC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running_row_count\n            FROM ${catalog}\n            ORDER BY max_timestamp DESC, min_timestamp ASC\n        )\n        ORDER BY max_timestamp DESC, min_timestamp ASC\n    )\n    WHERE (running_row_count < LATEST) OR is_boundary_row\n) AS min_max\n\nSELECT statusCode::String, * EXCEPT (statusCode, unknown)\nFROM ${table}\n--WHERE reqTimeSec BETWEEN min_max[1] AND min_max[2]\nWHERE $__timeFilter(reqTimeSec)\nAND $__conditionalAll( statusCode ${AND_statusCode} (${statusCode:sqlstring}), $statusCode)\nAND $__conditionalAll( reqHost ${AND_reqHost} (${reqHost:sqlstring}), $reqHost)\nAND $__conditionalAll( cacheStatus ${AND_cacheStatus} (${cacheStatus:sqlstring}), $cacheStatus)\nAND $__conditionalAll( reqMethod ${AND_reqMethod} (${reqMethod:sqlstring}), $reqMethod)\nAND $__conditionalAll( rspContentType ${AND_rspContentType} (${rspContentType:sqlstring}), $rspContentType)\nAND $__conditionalAll( errorCode ${AND_errorCode} (${errorCode:sqlstring}), $errorCode)\nAND $__conditionalAll( transferTimeMSec >= ${transferTimeMSec_percentile}, $transferTimeMSec_percentile)\nAND $__conditionalAll(${metric_name} ${metric_filter} ${metric_value}, $metric_filter)\nAND $__conditionalAll(reqPath ${AND_reqPath} (${reqPath:sqlstring}), $reqPath)\nAND $__conditionalAll(cliIP ${AND_cliIP} (${cliIP:sqlstring}), $cliIP)\nAND $__conditionalAll(policy ${AND_policy} (${policy:sqlstring}), $policy)\nAND $__conditionalAll(denyRule ${AND_denyRule} (${denyRule:sqlstring}), $denyRule)\nAND $__conditionalAll(UA ${AND_UA} (${UA:sqlstring}), $UA)\nAND $__conditionalAll(denyGroup ${AND_denyGroup} (${denyGroup:sqlstring}), $denyGroup)\nAND $__conditionalAll(cust_tlsFingerprint ${AND_custom_tlsFingerprint} (${custom_tlsFingerprint:sqlstring}), $custom_tlsFingerprint)\nORDER BY reqTimeSec DESC\nLIMIT 1000\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment='db=${__dashboard}; dp=Raw Logs; du=${__user.login}'"
	println(sql)
	parser := NewParser(sql)
	expr, err := parser.ParseStmts()
	marshal, err := json.Marshal(expr)
	fmt.Printf("%s", marshal)
	require.NoError(t, err)

}
