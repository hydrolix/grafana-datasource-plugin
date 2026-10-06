package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
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
	hdxQuery, err := toHdxQuery(req, request.Data)
	if err != nil {
		wrapError(rw, err)
		return
	}

	// sqlds.Interpolator is a func field taking (*sqlutil.Query,
	// json.RawMessage). Hydrolix-specific fields (filters, round, etc.)
	// travel via the rawJSON payload — shape preserved from the fork's
	// HDXQuery so the plugin-local interpolator (C5) decodes it the same
	// way. Headers are not serialised.
	rawJSON, err := json.Marshal(hdxQuery)
	if err != nil {
		wrapError(rw, err)
		return
	}

	// NewHdxSqlDatasource always installs the Hydrolix interpolator, so a nil
	// field here means the datasource was not constructed through that path —
	// surface it as an error rather than silently degrading.
	if ds.Interpolator == nil {
		wrapError(rw, errors.New("interpolator not configured"))
		return
	}
	body, err := ds.Interpolator(req.Context(),
		&sqlutil.Query{
			RawSQL:    hdxQuery.RawSQL,
			TimeRange: hdxQuery.TimeRange,
			Interval:  hdxQuery.Interval,
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

// Validator is declared here because pkg/plugin imports pkg/api.
type Validator interface {
	Validate(ctx context.Context, q models.HdxQuery) (models.ValidationResult, error)
}

// Validate puts query problems in the response data; the envelope error is
// only for failing to validate.
func Validate(v Validator, responseWriter http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Error("validate: recovered panic", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			wrapError(responseWriter, fmt.Errorf("internal error while validating query: %v", r))
		}
	}()
	if v == nil {
		wrapError(responseWriter, errors.New("query validation is not configured"))
		return
	}
	var request Request[QueryData]
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		wrapError(responseWriter, err)
		return
	}
	hdxQuery, err := toHdxQuery(req, request.Data)
	if err != nil {
		wrapError(responseWriter, err)
		return
	}
	result, err := v.Validate(req.Context(), hdxQuery)
	if err != nil {
		wrapError(responseWriter, err)
		return
	}
	writeJSON(responseWriter, Response[models.ValidationResult]{
		false,
		"",
		result,
	})
}

func toHdxQuery(req *http.Request, data QueryData) (models.HdxQuery, error) {
	interval, err := time.ParseDuration(data.Interval)
	if err != nil {
		return models.HdxQuery{}, err
	}
	return models.HdxQuery{
		RawSQL:        data.RawSql,
		Filters:       data.Filters,
		Round:         data.Round,
		QuerySettings: data.QuerySettings,
		Interval:      interval,
		TimeRange:     data.Range.ToTimeRange(),
		Headers:       req.Header,
	}, nil
}

// MacroCTEs returns the map of macro-to-CTE associations the dashboard's
// macro-expansion preview consumes. Restored in C5 with plugin-local
// GetMacroCTEs / CTE types.
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

	expr, err := parser.NewParser(astRequest.Data.Query).ParseStmts()
	if err != nil {
		wrapError(rw, err)
		return
	}

	body, err := cte.GetMacroCTEs(expr)
	if err != nil {
		wrapError(rw, err)
		return
	}

	writeJSON(rw, Response[[]cte.CTE]{
		false,
		"",
		slices.Collect(maps.Values(body)),
	})
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

func Routes(ds *sqlds.SQLDatasource, v Validator) map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"/ast": AST,
		"/interpolate": func(writer http.ResponseWriter, request *http.Request) {
			Interpolate(ds, writer, request)
		},
		"/macroCTE": MacroCTEs,
		"/validate": func(writer http.ResponseWriter, request *http.Request) {
			Validate(v, writer, request)
		},
	}
}

type Request[T any] struct {
	Data T
}
type QueryData struct {
	RawSql        string                `json:"rawSql"`
	Round         string                `json:"round"`
	Filters       []models.AdHocFilter  `json:"filters"`
	Range         Range                 `json:"range"`
	Interval      string                `json:"interval"`
	QuerySettings []models.QuerySetting `json:"querySettings"`
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
