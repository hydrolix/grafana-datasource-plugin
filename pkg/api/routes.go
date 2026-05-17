package api

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/sqlds/v5"
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

	rw.WriteHeader(http.StatusOK)
	marshal, err := json.Marshal(Response[[]parser.Expr]{
		false,
		"",
		body,
	})
	_, err = rw.Write(marshal)
}
func Interpolate(ds *sqlds.HydrolixDatasource, rw http.ResponseWriter, req *http.Request) {
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

	body, err := ds.Interpolator.Interpolate(req.Context(),
		&sqlds.HDXQuery{
			RawSQL:    request.Data.RawSql,
			Filters:   request.Data.Filters,
			Round:     request.Data.Round,
			Interval:  interval,
			TimeRange: timeRange,
			Headers:   req.Header,
		})

	if err != nil {
		wrapError(rw, err)
		return

	}

	rw.WriteHeader(http.StatusOK)
	marshal, err := json.Marshal(Response[string]{
		false,
		"",
		body,
	})
	_, err = rw.Write(marshal)

}

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

	body, err := sqlds.GetMacroCTEs(expr)
	if err != nil {
		wrapError(rw, err)
		return

	}

	rw.WriteHeader(http.StatusOK)
	marshal, err := json.Marshal(Response[[]sqlds.CTE]{
		false,
		"",
		slices.Collect(maps.Values(body)),
	})
	_, err = rw.Write(marshal)

}

func wrapError(rw http.ResponseWriter, err error) {
	rw.WriteHeader(http.StatusOK)
	marshal, _ := json.Marshal(Response[any]{
		true,
		err.Error(),
		nil,
	})
	_, err = rw.Write(marshal)
	return
}

func Routes(ds *sqlds.HydrolixDatasource) map[string]func(http.ResponseWriter, *http.Request) {
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
	RawSql   string              `json:"rawSql"`
	Round    string              `json:"round"`
	Filters  []sqlds.AdHocFilter `json:"filters"`
	Range    Range               `json:"range"`
	Interval string              `json:"interval"`
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
