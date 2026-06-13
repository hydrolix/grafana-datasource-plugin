package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

func AST(rw http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			wrapError(rw, errors.New("Unknown Error"))
		}
	}()
	var astRequest Request[ASTData]
	if err := json.NewDecoder(req.Body).Decode(&astRequest); err != nil {
		wrapError(rw, err)
		return
	}

	body, err := parser.NewParser(astRequest.Data.Query).ParseStmts()
	if err != nil {
		wrapError(rw, err)
		return

	}

	writeJSON(rw, Response[[]parser.Expr]{
		false,
		"",
		body,
	})
}
func Interpolate(ds *sqlds.SQLDatasource, rw http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			rawMessage, _ := json.Marshal(r)
			wrapError(rw, errors.New((string(rawMessage))))
		}
	}()
	var request Request[QueryData]
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		wrapError(rw, err)
		return
	}
	timeRange := request.Data.Range.ToTimeRange()
	interval, err := time.ParseDuration(request.Data.Interval)

	if err != nil {
		wrapError(rw, err)
		return
	}

	// The new sqlds.Interpolator interface takes (*sqlds.SQLDatasource,
	// *sqlutil.Query, json.RawMessage). Hydrolix-specific fields (filters,
	// round, etc.) travel via the rawJSON payload — shape preserved from
	// the fork's HDXQuery so the plugin-local interpolator (C5) can decode
	// it the same way. Until C5 lands, ds.Interpolator is nil and the call
	// falls back to sqlds.DefaultInterpolator (no Hydrolix macro expansion).
	hdxQuery := models.HdxQuery{
		RawSQL:    request.Data.RawSql,
		Filters:   request.Data.Filters,
		Round:     request.Data.Round,
		Interval:  interval,
		TimeRange: timeRange,
		Headers:   req.Header,
	}
	rawJSON, err := json.Marshal(hdxQuery)
	if err != nil {
		wrapError(rw, err)
		return
	}

	interpolator := ds.Interpolator
	if interpolator == nil {
		interpolator = sqlds.DefaultInterpolator{}
	}
	body, err := interpolator.Interpolate(req.Context(), ds,
		&sqlutil.Query{
			RawSQL:    request.Data.RawSql,
			TimeRange: timeRange,
			Interval:  interval,
		},
		rawJSON,
	)

	if err != nil {
		wrapError(rw, err)
		return

	}

	writeJSON(rw, Response[string]{
		false,
		"",
		body,
	})

}

// MacroCTEs is temporarily stubbed during the sqlds-extraction migration.
// At the pinned sqlds revision (ef925e1), GetMacroCTEs and CTE no longer
// live in sqlds. The plugin-local replacements ship in C5
// (plugin-hdx-interpolator). Until then, this handler returns an empty
// CTE list — the dashboard's macro-CTE preview surface is unavailable.
func MacroCTEs(rw http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			wrapError(rw, errors.New("Unknown Error"))
		}
	}()
	var astRequest Request[ASTData]
	if err := json.NewDecoder(req.Body).Decode(&astRequest); err != nil {
		wrapError(rw, err)
		return
	}
	if _, err := parser.NewParser(astRequest.Data.Query).ParseStmts(); err != nil {
		wrapError(rw, err)
		return
	}
	writeJSON(rw, Response[[]any]{false, "", []any{}})
}

func wrapError(rw http.ResponseWriter, err error) {
	marshal, marshalErr := json.Marshal(Response[any]{
		true,
		err.Error(),
		nil,
	})
	if marshalErr != nil {
		http.Error(rw, marshalErr.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", strconv.Itoa(len(marshal)))
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(marshal)
}

func writeJSON(rw http.ResponseWriter, v any) {
	marshal, err := json.Marshal(v)
	if err != nil {
		wrapError(rw, err)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", strconv.Itoa(len(marshal)))
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(marshal)
}

func Routes(ds *sqlds.SQLDatasource) map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"/ast": AST,
		"/interpolate": func(writer http.ResponseWriter, request *http.Request) {
			Interpolate(ds, writer, request)
		},
		"/macroCTE": MacroCTEs,
	}
}

type Request[T any] struct {
	Data T
}
type QueryData struct {
	RawSql   string               `json:"rawSql"`
	Round    string               `json:"round"`
	Filters  []models.AdHocFilter `json:"filters"`
	Range    Range                `json:"range"`
	Interval string               `json:"interval"`
}

type Range struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func (r *Range) ToTimeRange() backend.TimeRange {
	return backend.TimeRange{
		From: r.From,
		To:   r.To,
	}
}

type ASTData struct {
	Query string `json:"query"`
}

type Response[T any] struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"errorMessage"`
	Data         T      `json:"data"`
}
