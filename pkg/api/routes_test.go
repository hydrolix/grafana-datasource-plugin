package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	rangeFrom = time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	rangeTo   = time.Date(2026, 6, 13, 13, 0, 0, 0, time.UTC)
)

// stubInterpolator records the rawJSON it receives and returns a canned
// rewrite. Lets the Interpolate handler test assert on what the API layer
// hands to the interpolator without spinning up the real macro pipeline.
type stubInterpolator struct {
	calls    int
	gotJSON  json.RawMessage
	gotQuery *sqlutil.Query
	out      string
	err      error
}

func (s *stubInterpolator) Interpolate(_ context.Context, _ *sqlds.SQLDatasource, q *sqlutil.Query, raw json.RawMessage) (string, error) {
	s.calls++
	s.gotJSON = raw
	s.gotQuery = q
	return s.out, s.err
}

func postJSON(t *testing.T, handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func decodeResponse[T any](t *testing.T, rr *httptest.ResponseRecorder) Response[T] {
	t.Helper()
	body, err := io.ReadAll(rr.Body)
	require.NoError(t, err)
	var resp Response[T]
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp
}

func TestAST_ValidQueryReturnsParsedExprs(t *testing.T) {
	rr := postJSON(t, AST, Request[ASTData]{Data: ASTData{Query: "SELECT 1 FROM t"}})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	// AST returns []parser.Expr — round-trip via map[string]any so the
	// test stays decoupled from concrete AST node types.
	var resp Response[[]map[string]any]
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Error)
	assert.NotEmpty(t, resp.Data)
}

func TestAST_InvalidJSONIsReportedAsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	AST(rr, req)

	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
	assert.NotEmpty(t, resp.ErrorMessage)
}

func TestAST_InvalidSQLIsReportedAsError(t *testing.T) {
	rr := postJSON(t, AST, Request[ASTData]{Data: ASTData{Query: "SELEC FROM :: bad"}})

	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
	assert.NotEmpty(t, resp.ErrorMessage)
}

func TestInterpolate_HappyPath(t *testing.T) {
	stub := &stubInterpolator{out: "SELECT 1 AND col >= toDateTime(0) AND col <= toDateTime(60)"}
	ds := &sqlds.SQLDatasource{Interpolator: stub}

	body := Request[QueryData]{
		Data: QueryData{
			RawSql:   "SELECT 1 AND $__timeFilter(col)",
			Round:    "1m",
			Filters:  []models.AdHocFilter{{Key: "host", Operator: "=", Value: "prod-1"}},
			Range:    Range{From: rangeFrom, To: rangeTo},
			Interval: "30s",
		},
	}
	rr := postJSON(t, func(w http.ResponseWriter, r *http.Request) { Interpolate(ds, w, r) }, body)

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeResponse[string](t, rr)
	assert.False(t, resp.Error)
	assert.Equal(t, stub.out, resp.Data)

	require.Equal(t, 1, stub.calls)
	assert.Equal(t, body.Data.RawSql, stub.gotQuery.RawSQL)
	assert.Equal(t, body.Data.Range.From, stub.gotQuery.TimeRange.From)
	assert.Equal(t, body.Data.Range.To, stub.gotQuery.TimeRange.To)

	// The handler should have round-tripped the request through HdxQuery so
	// the rawJSON the interpolator sees carries the Hydrolix-only fields
	// (round, filters) plus the time range.
	var forwarded models.HdxQuery
	require.NoError(t, json.Unmarshal(stub.gotJSON, &forwarded))
	assert.Equal(t, body.Data.RawSql, forwarded.RawSQL)
	assert.Equal(t, body.Data.Round, forwarded.Round)
	assert.Equal(t, body.Data.Filters, forwarded.Filters)
}

func TestInterpolate_NilInterpolatorFallsBackToDefault(t *testing.T) {
	// ds.Interpolator == nil → the handler should resolve the field to
	// sqlds.DefaultInterpolator{} rather than nil-derefing on the call.
	// A bare &sqlds.SQLDatasource{} can't actually interpolate (the default
	// reaches for ds.driver().Macros() which isn't wired without
	// NewDatasource), so the call returns an error via the handler's
	// recover. The point of this test is the branch — not the rewrite —
	// so we assert the handler returns a well-formed JSON response
	// instead of escaping the panic.
	ds := &sqlds.SQLDatasource{}

	body := Request[QueryData]{
		Data: QueryData{
			RawSql:   "SELECT 1",
			Range:    Range{From: rangeFrom, To: rangeTo},
			Interval: "30s",
		},
	}
	rr := postJSON(t, func(w http.ResponseWriter, r *http.Request) { Interpolate(ds, w, r) }, body)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	// Response decodes — recover() caught the nil-driver panic and wrapped
	// it via wrapError, which produces Response[any]{Error: true, ...}.
	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
}

func TestInterpolate_InvalidJSONIsReportedAsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	Interpolate(&sqlds.SQLDatasource{}, rr, req)

	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
	assert.NotEmpty(t, resp.ErrorMessage)
}

func TestInterpolate_BadIntervalIsReportedAsError(t *testing.T) {
	rr := postJSON(t, func(w http.ResponseWriter, r *http.Request) { Interpolate(&sqlds.SQLDatasource{}, w, r) },
		Request[QueryData]{Data: QueryData{
			RawSql:   "SELECT 1",
			Range:    Range{From: rangeFrom, To: rangeTo},
			Interval: "not-a-duration",
		}})

	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
	assert.Contains(t, resp.ErrorMessage, "not-a-duration")
}

func TestInterpolate_InterpolatorErrorIsReported(t *testing.T) {
	stub := &stubInterpolator{err: errors.New("boom")}
	ds := &sqlds.SQLDatasource{Interpolator: stub}

	rr := postJSON(t, func(w http.ResponseWriter, r *http.Request) { Interpolate(ds, w, r) },
		Request[QueryData]{Data: QueryData{
			RawSql:   "SELECT 1",
			Range:    Range{From: rangeFrom, To: rangeTo},
			Interval: "30s",
		}})

	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
	assert.Equal(t, "boom", resp.ErrorMessage)
}

func TestMacroCTEs_ReturnsCTEsForKnownMacros(t *testing.T) {
	rr := postJSON(t, MacroCTEs, Request[ASTData]{Data: ASTData{Query: "SELECT $__timeFilter FROM mydb.events"}})

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeResponse[[]cte.CTE](t, rr)
	assert.False(t, resp.Error)
	require.Len(t, resp.Data, 1)
	got := resp.Data[0]
	assert.Equal(t, "$__timeFilter", got.Macro)
	assert.Equal(t, "events", got.Table)
	assert.Equal(t, "mydb", got.Database)
}

func TestMacroCTEs_NoMacrosYieldsEmptySet(t *testing.T) {
	rr := postJSON(t, MacroCTEs, Request[ASTData]{Data: ASTData{Query: "SELECT 1 FROM t"}})

	resp := decodeResponse[[]cte.CTE](t, rr)
	assert.False(t, resp.Error)
	assert.Empty(t, resp.Data)
}

func TestMacroCTEs_InvalidJSONIsReportedAsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	MacroCTEs(rr, req)

	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
	assert.NotEmpty(t, resp.ErrorMessage)
}

func TestMacroCTEs_InvalidSQLIsReportedAsError(t *testing.T) {
	rr := postJSON(t, MacroCTEs, Request[ASTData]{Data: ASTData{Query: "SELEC FROM :: bad"}})

	resp := decodeResponse[any](t, rr)
	assert.True(t, resp.Error)
	assert.NotEmpty(t, resp.ErrorMessage)
}

func TestRoutes_ExposesExpectedKeysAndWiresInterpolate(t *testing.T) {
	stub := &stubInterpolator{out: "rewritten"}
	ds := &sqlds.SQLDatasource{Interpolator: stub}

	routes := Routes(ds)
	assert.ElementsMatch(t, []string{"/ast", "/interpolate", "/macroCTE"}, keysOf(routes))

	// Hit /interpolate through the factory-returned closure and confirm the
	// ds-bound interpolator is the one that runs.
	body, _ := json.Marshal(Request[QueryData]{Data: QueryData{
		RawSql:   "SELECT 1",
		Range:    Range{From: rangeFrom, To: rangeTo},
		Interval: "30s",
	}})
	req := httptest.NewRequest(http.MethodPost, "/interpolate", strings.NewReader(string(body)))
	rr := httptest.NewRecorder()
	routes["/interpolate"](rr, req)

	assert.Equal(t, 1, stub.calls, "interpolate closure should dispatch to ds.Interpolator")
	resp := decodeResponse[string](t, rr)
	assert.Equal(t, "rewritten", resp.Data)
}

func TestRoutes_ASTAndMacroCTEAreDirectHandlers(t *testing.T) {
	// /ast and /macroCTE are package-level functions in the map, not closures.
	// Driving them via the map exercises the wire-up the plugin's Routes call
	// installs into sqlds.CustomRoutes.
	routes := Routes(&sqlds.SQLDatasource{})

	{
		body, _ := json.Marshal(Request[ASTData]{Data: ASTData{Query: "SELECT 1 FROM t"}})
		rr := httptest.NewRecorder()
		routes["/ast"](rr, httptest.NewRequest(http.MethodPost, "/ast", strings.NewReader(string(body))))
		assert.Equal(t, http.StatusOK, rr.Code)
	}
	{
		body, _ := json.Marshal(Request[ASTData]{Data: ASTData{Query: "SELECT 1 FROM t"}})
		rr := httptest.NewRecorder()
		routes["/macroCTE"](rr, httptest.NewRequest(http.MethodPost, "/macroCTE", strings.NewReader(string(body))))
		assert.Equal(t, http.StatusOK, rr.Code)
	}
}

func keysOf(m map[string]func(http.ResponseWriter, *http.Request)) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
