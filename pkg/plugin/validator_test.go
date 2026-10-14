package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeadingPKColumn(t *testing.T) {
	tests := []struct {
		name   string
		pk     string
		want   string
		wantOK bool
	}{
		{"single column", "datetime", "datetime", true},
		{"backtick-quoted", "`event time`", "event time", true},
		{"composite takes the leading column", "ts, id", "ts", true},
		{"expression key has no single column", "toStartOfHour(ts)", "", false},
		{"empty means the table has no key", "", "", false},
		{"blank", "  ", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := leadingPKColumn(tt.pk)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func parseSelect(t *testing.T, sql string) *parser.SelectQuery {
	t.Helper()
	stmts, err := parser.NewParser(sql).ParseStmts()
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	sel, ok := stmts[0].(*parser.SelectQuery)
	require.True(t, ok, "expected a SELECT, got %T", stmts[0])
	return sel
}

func TestUnconstrainedPKTables(t *testing.T) {
	keys := map[string]string{
		"t":         "ts",
		"u":         "ts",
		"db.t":      "ts",
		"db.x":      "ts",
		"composite": "ts, id",
		"expr":      "toStartOfHour(ts)",
		"nokey":     "",
	}
	resolve := func(_ context.Context, _ http.Header, database, table string) (string, error) {
		name := table
		if database != "" {
			name = database + "." + table
		}
		pk, ok := keys[name]
		if !ok {
			return "", ErrPrimaryKeyNotFound
		}
		return pk, nil
	}

	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{"no WHERE", "SELECT * FROM t", []string{"Primary key `ts` of `t`"}},
		{"filtered in WHERE", "SELECT * FROM t WHERE ts > now()", nil},
		{"alias-qualified column", "SELECT * FROM t AS a WHERE a.ts > now()", nil},
		{"filtered in PREWHERE", "SELECT * FROM t PREWHERE ts > now() WHERE x = 1", nil},
		{"expanded $__timeFilter", "SELECT * FROM t WHERE ts >= toDateTime(1) AND ts <= toDateTime(2)", nil},
		{"other column only", "SELECT * FROM t WHERE other = 1", []string{"Primary key `ts` of `t`"}},
		{"match is case-sensitive", "SELECT * FROM t WHERE TS > 1", []string{"Primary key `ts` of `t`"}},
		{"database-qualified table", "SELECT * FROM db.t", []string{"Primary key `ts` of `db.t`"}},
		{"CTE name is not a table", "WITH x AS (SELECT * FROM t WHERE ts > 1) SELECT * FROM x", nil},
		{"CTE body filtered by its reader", "WITH x AS (SELECT * FROM t) SELECT * FROM x WHERE ts > 1", nil},
		{"CTE body read without a key filter", "WITH x AS (SELECT * FROM t) SELECT * FROM x WHERE other = 1", []string{"Primary key `ts` of `t`"}},
		{"CTE read twice, once unfiltered", "WITH x AS (SELECT * FROM t) SELECT * FROM x WHERE ts > 1 UNION ALL SELECT * FROM x", []string{"Primary key `ts` of `t`"}},
		{"database-qualified name is a table, not the CTE", "WITH x AS (SELECT * FROM t WHERE ts > 1) SELECT * FROM db.x", []string{"Primary key `ts` of `db.x`"}},
		{"CTEs reading each other terminate", "WITH x AS (SELECT * FROM y), y AS (SELECT * FROM x) SELECT * FROM x", nil},
		{"FROM subquery filtered by its reader", "SELECT * FROM (SELECT * FROM t) WHERE ts > 1", nil},
		{"nested subqueries filtered at the top", "SELECT * FROM (SELECT * FROM (SELECT * FROM t)) WHERE ts > 1", nil},
		{"UNION inside a subquery filtered by its reader", "SELECT * FROM (SELECT * FROM t UNION ALL SELECT * FROM u) WHERE ts > 1", nil},
		{"FROM subquery read without a key filter", "SELECT * FROM (SELECT * FROM t) WHERE other = 1", []string{"Primary key `ts` of `t`"}},
		{"same table in two branches is reported once", "SELECT * FROM t UNION ALL SELECT * FROM t", []string{"Primary key `ts` of `t`"}},
		{"string literal naming the column does not count", "SELECT * FROM t WHERE other = 'ts'", []string{"Primary key `ts` of `t`"}},
		{"each UNION branch is checked", "SELECT * FROM t WHERE ts > 1 UNION ALL SELECT * FROM u", []string{"Primary key `ts` of `u`"}},
		{"subquery in FROM", "SELECT * FROM (SELECT * FROM t WHERE ts > 1)", nil},
		{"JOIN is skipped", "SELECT * FROM t JOIN u ON t.id = u.id", nil},
		{"table function is skipped", "SELECT * FROM numbers(10)", nil},
		{"composite key filtered on trailing column", "SELECT * FROM composite WHERE id = 1", []string{"Primary key `ts` of `composite`"}},
		{"expression key is skipped", "SELECT * FROM expr", nil},
		{"table without a key", "SELECT * FROM nokey", nil},
		{"unknown table is skipped", "SELECT * FROM missing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unconstrainedPKTables(context.Background(), parseSelect(t, tt.sql), nil, resolve)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUnconstrainedPKTables_ResolverErrorIsSkipped(t *testing.T) {
	resolve := func(context.Context, http.Header, string, string) (string, error) {
		return "", errors.New("cluster unreachable")
	}
	got := unconstrainedPKTables(context.Background(), parseSelect(t, "SELECT * FROM t"), nil, resolve)
	assert.Empty(t, got)
}

// warnCapture records Warn calls; every other method is the wrapped logger's.
type warnCapture struct {
	log.Logger
	warnings []string
}

func (w *warnCapture) Warn(msg string, _ ...any) { w.warnings = append(w.warnings, msg) }

func captureWarnings(t *testing.T) *warnCapture {
	t.Helper()
	capture := &warnCapture{Logger: log.DefaultLogger}
	previous := log.DefaultLogger
	log.DefaultLogger = capture
	t.Cleanup(func() { log.DefaultLogger = previous })
	return capture
}

// The skip log is for lookup failures only: a table without a primary key is
// a value, and a table the cluster does not know is the user's query problem.
func TestUnconstrainedPKTables_SkipLogOnlyForLookupFailures(t *testing.T) {
	tests := []struct {
		name    string
		pk      string
		err     error
		wantLog bool
	}{
		{"table without a primary key", "", nil, false},
		{"table not found", "", backend.PluginError(ErrPrimaryKeyNotFound), false},
		{"other plugin-sourced failure", "", backend.PluginError(errors.New("default database not configured")), true},
		{"downstream failure", "", backend.DownstreamError(errors.New("cluster unreachable")), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := captureWarnings(t)
			resolve := func(context.Context, http.Header, string, string) (string, error) {
				return tt.pk, tt.err
			}
			got := unconstrainedPKTables(context.Background(), parseSelect(t, "SELECT * FROM t"), nil, resolve)
			assert.Empty(t, got)
			if tt.wantLog {
				assert.Equal(t, []string{"query validation: skipping PK check"}, capture.warnings)
			} else {
				assert.Empty(t, capture.warnings)
			}
		})
	}
}

func TestUnconstrainedPKTables_StopsWhenContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	resolve := func(context.Context, http.Header, string, string) (string, error) {
		calls++
		cancel()
		return "", context.Canceled
	}
	got := unconstrainedPKTables(ctx, parseSelect(t, "SELECT * FROM t UNION ALL SELECT * FROM u"), nil, resolve)
	assert.Empty(t, got)
	assert.Equal(t, 1, calls)
}

func TestLooksLikeQuery(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"SELECT FROM WHERE (", true},
		{"  select 1", true},
		{"WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"(SELECT 1) UNION ALL (SELECT 2)", true},
		{"-- note\nSELECT 1", true},
		{"# note\n/* block */ SELECT 1", true},
		{"EXISTS TABLE t", false},
		{"SELECTED", false},
		{"WITHOUT", false},
		{"-- only a comment", false},
		{"/* unterminated SELECT", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			assert.Equal(t, tt.want, looksLikeQuery(tt.sql))
		})
	}
}

func TestPositionsInUserSQL(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{"single-line position", "Syntax error: failed at position 42 (ORDER): ORDER BY t.", "Syntax error: failed at position 34 (ORDER): ORDER BY t."},
		{"first-line column", "failed at position 20 (FROM) (line 1, col 20): FROM", "failed at position 12 (FROM) (line 1, col 12): FROM"},
		{"later lines keep their column", "failed at position 42 (ORDER) (line 2, col 25): ORDER", "failed at position 34 (ORDER) (line 2, col 25): ORDER"},
		{"offset inside the prefix is left alone", "failed at position 3 (EXPLAIN)", "failed at position 3 (EXPLAIN)"},
		{"other errors are untouched", "Code: 47. Unknown identifier `position`", "Code: 47. Unknown identifier `position`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, positionsInUserSQL(tt.msg))
		})
	}
}

// validatorFixture fakes the PK lookup and the EXPLAIN, recording requests.
type validatorFixture struct {
	mu             sync.Mutex
	requests       []*backend.QueryDataRequest
	pk             string
	pkFn           func(ctx context.Context) error
	explainErr     error
	explainFn      func(ctx context.Context) error
	explainSkipped bool
}

func (f *validatorFixture) queryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()

	q := req.Queries[0]
	switch q.RefID {
	case "pk_query":
		if f.pkFn != nil {
			if err := f.pkFn(ctx); err != nil {
				return &backend.QueryDataResponse{Responses: map[string]backend.DataResponse{q.RefID: {Error: err}}}, nil
			}
		}
		return respondWith(frameOf([]string{f.pk}), q.RefID), nil
	case "validate_query":
		f.mu.Lock()
		f.explainSkipped = interpolationSkipped(ctx)
		f.mu.Unlock()
		err := f.explainErr
		if f.explainFn != nil {
			err = f.explainFn(ctx)
		}
		if err != nil {
			return &backend.QueryDataResponse{Responses: map[string]backend.DataResponse{q.RefID: {Error: err}}}, nil
		}
		return respondWith(frameOf([]string{"Expression"}), q.RefID), nil
	}
	return nil, errors.New("unexpected query " + q.RefID)
}

func (f *validatorFixture) explainRequests() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, r := range f.requests {
		if r.Queries[0].RefID != "validate_query" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(r.Queries[0].JSON, &payload); err == nil {
			out = append(out, payload)
		}
	}
	return out
}

func newTestValidator(f *validatorFixture) *QueryValidator {
	md := NewMetadataProvider(&fakeMetadataDS{queryDataFn: f.queryData, defaultDB: "db"})
	return NewQueryValidator(NewHdxInterpolator(md, Macros), md)
}

func validationQuery(sql string) models.HdxQuery {
	return models.HdxQuery{
		RawSQL:    sql,
		TimeRange: backend.TimeRange{From: time.Unix(1000, 0).UTC(), To: time.Unix(2000, 0).UTC()},
		Interval:  30 * time.Second,
	}
}

func TestValidate_FilteredSelectIsValid(t *testing.T) {
	f := &validatorFixture{pk: "ts"}
	q := validationQuery("SELECT count() FROM t WHERE ts > now() - 60")
	q.QuerySettings = []models.QuerySetting{{Setting: "max_threads", Value: "4"}}
	q.Headers = http.Header{"Authorization": []string{"Bearer user-token"}}

	res, err := newTestValidator(f).Validate(context.Background(), q)

	require.NoError(t, err)
	assert.Equal(t, models.ValidationResult{}, res)

	explains := f.explainRequests()
	require.Len(t, explains, 1)
	assert.Equal(t, "EXPLAIN SELECT count() FROM t WHERE ts > now() - 60", explains[0]["rawSql"])
	assert.Equal(t, []any{map[string]any{"setting": "max_threads", "value": "4"}}, explains[0]["querySettings"])
	for _, r := range f.requests {
		assert.Equal(t, "Bearer user-token", r.GetHTTPHeader("Authorization"), "every schema query carries the user's headers")
	}
}

func TestValidate_UnfilteredPrimaryKeyWarns(t *testing.T) {
	f := &validatorFixture{pk: "ts"}

	res, err := newTestValidator(f).Validate(context.Background(), validationQuery("SELECT * FROM t"))

	require.NoError(t, err)
	assert.Empty(t, res.Error)
	assert.Equal(t, "Primary key `ts` of `t` not filtered in WHERE; add $__timeFilter() to limit the scan.", res.Warning)
}

func TestValidate_TimeFilterMacroSatisfiesPKCheck(t *testing.T) {
	f := &validatorFixture{pk: "ts"}
	q := validationQuery("SELECT * FROM t WHERE $__timeFilter()")
	q.Headers = http.Header{"Authorization": []string{"Bearer user-token"}}

	res, err := newTestValidator(f).Validate(context.Background(), q)

	require.NoError(t, err)
	assert.Equal(t, models.ValidationResult{}, res)
	explains := f.explainRequests()
	require.Len(t, explains, 1)
	assert.Equal(t, "EXPLAIN SELECT * FROM t WHERE ts >= toDateTime(1000) AND ts <= toDateTime(2000)", explains[0]["rawSql"])
	require.Equal(t, "pk_query", f.requests[0].Queries[0].RefID, "the macro's own primary-key lookup runs first")
	assert.Equal(t, "Bearer user-token", f.requests[0].GetHTTPHeader("Authorization"), "the macro lookup carries the user's headers")
}

func TestValidate_ExplainErrorIsReportedWithoutPKCheck(t *testing.T) {
	f := &validatorFixture{pk: "ts", explainErr: errors.New("Code: 47. UNKNOWN_IDENTIFIER: Missing columns: 'nope'")}

	res, err := newTestValidator(f).Validate(context.Background(), validationQuery("SELECT nope FROM t"))

	require.NoError(t, err)
	assert.Equal(t, models.ValidationResult{Error: "Code: 47. UNKNOWN_IDENTIFIER: Missing columns: 'nope'"}, res)
	for _, r := range f.requests {
		assert.NotEqual(t, "pk_query", r.Queries[0].RefID, "PK check must not run after a failed dry-run")
	}
}

// sqlds interpolates every query, so the EXPLAIN's context must say to skip.
func TestValidate_ExplainSkipsSecondInterpolation(t *testing.T) {
	f := &validatorFixture{pk: "ts"}
	sql := "SELECT '$__fromTime' AS a, '$$__timeFilter' AS b FROM t WHERE $__timeFilter(ts)"
	firstPass := "SELECT '$__fromTime' AS a, '$$__timeFilter' AS b FROM t WHERE ts >= toDateTime(1000) AND ts <= toDateTime(2000)"

	res, err := newTestValidator(f).Validate(context.Background(), validationQuery(sql))
	require.NoError(t, err)
	assert.Equal(t, models.ValidationResult{}, res)

	explains := f.explainRequests()
	require.Len(t, explains, 1)
	assert.Equal(t, "EXPLAIN "+firstPass, explains[0]["rawSql"])
	assert.NotContains(t, explains[0], "skipInterpolation")
	assert.True(t, f.explainSkipped, "the EXPLAIN context must tell the interpolator to skip")
}

func TestValidate_NonSelectStatementsAreSkipped(t *testing.T) {
	for _, sql := range []string{
		"DESCRIBE TABLE t",
		"SHOW TABLES",
		"SELECT 1; SELECT 2",
		"INSERT INTO t SELECT * FROM u",
		"DROP TABLE t",
		"ALTER TABLE t DELETE WHERE 1",
		"EXPLAIN SYNTAX SELECT 1",
		"EXISTS TABLE t",
	} {
		t.Run(sql, func(t *testing.T) {
			f := &validatorFixture{}

			res, err := newTestValidator(f).Validate(context.Background(), validationQuery(sql))

			require.NoError(t, err)
			assert.Equal(t, models.ValidationResult{Skipped: true}, res)
			assert.Empty(t, f.requests)
		})
	}
}

func TestValidate_UnparseableQueryStillGoesToExplain(t *testing.T) {
	for _, sql := range []string{"SELECT FROM WHERE (", "-- note\nSELECT FROM WHERE ("} {
		t.Run(sql, func(t *testing.T) {
			f := &validatorFixture{explainErr: errors.New("Code: 62. SYNTAX_ERROR")}

			res, err := newTestValidator(f).Validate(context.Background(), validationQuery(sql))

			require.NoError(t, err)
			assert.Equal(t, models.ValidationResult{Error: "Code: 62. SYNTAX_ERROR"}, res)
			assert.Len(t, f.explainRequests(), 1)
		})
	}
}

func TestValidate_SyntaxErrorPositionsPointIntoTheQuery(t *testing.T) {
	f := &validatorFixture{explainErr: errors.New("Code: 62. DB::Exception: Syntax error: failed at position 42 (ORDER): ORDER BY ts.")}

	res, err := newTestValidator(f).Validate(context.Background(), validationQuery("SELECT * FROM t LIMIT 1 ORDER BY ts"))

	require.NoError(t, err)
	assert.Equal(t, "Code: 62. DB::Exception: Syntax error: failed at position 34 (ORDER): ORDER BY ts.", res.Error)
}

func TestValidate_EmptyErrorMessageIsNotValid(t *testing.T) {
	f := &validatorFixture{explainErr: errors.New("")}

	res, err := newTestValidator(f).Validate(context.Background(), validationQuery("SELECT * FROM t WHERE ts > 1"))

	require.NoError(t, err)
	assert.NotEmpty(t, res.Error)
}

func TestValidate_InterpolationErrorIsReportedWithoutExplain(t *testing.T) {
	f := &validatorFixture{pk: "ts"}

	res, err := newTestValidator(f).Validate(context.Background(), validationQuery("SELECT * FROM t WHERE $__timeFilter(ts"))

	require.NoError(t, err)
	assert.Equal(t, ErrParseMacroArgs.Error(), res.Error)
	assert.Empty(t, f.explainRequests())
}

func TestValidate_ExplainTimeoutWarns(t *testing.T) {
	f := &validatorFixture{pk: "ts", explainFn: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	v := newTestValidator(f)
	v.timeout = 10 * time.Millisecond

	res, err := v.Validate(context.Background(), validationQuery("SELECT * FROM t WHERE ts > 1"))

	require.NoError(t, err)
	assert.Equal(t, models.ValidationResult{Warning: msgValidationTimedOut}, res)
}

func TestValidate_DatasourceQueryTimeoutWarns(t *testing.T) {
	// sqlds's own query timeout can expire before ours.
	f := &validatorFixture{pk: "ts", explainErr: fmt.Errorf("error querying the database: %w", context.DeadlineExceeded)}

	res, err := newTestValidator(f).Validate(context.Background(), validationQuery("SELECT * FROM t WHERE ts > 1"))

	require.NoError(t, err)
	assert.Equal(t, models.ValidationResult{Warning: msgValidationTimedOut}, res)
}

func TestValidate_MacroLookupTimeoutWarns(t *testing.T) {
	f := &validatorFixture{pkFn: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	v := newTestValidator(f)
	v.timeout = 10 * time.Millisecond

	res, err := v.Validate(context.Background(), validationQuery("SELECT * FROM t WHERE $__timeFilter()"))

	require.NoError(t, err)
	assert.Equal(t, models.ValidationResult{Warning: msgValidationTimedOut}, res)
	assert.Empty(t, f.explainRequests())
}

func TestValidate_CancelledCallerReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &validatorFixture{pk: "ts", explainFn: func(context.Context) error {
		cancel()
		return context.Canceled
	}}

	_, err := newTestValidator(f).Validate(ctx, validationQuery("SELECT * FROM t WHERE ts > 1"))

	assert.ErrorIs(t, err, context.Canceled)
}
