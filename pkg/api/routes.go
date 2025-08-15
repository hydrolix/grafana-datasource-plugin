package api

import (
	"encoding/json"
	"errors"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/datasource"
	"maps"
	"net/http"
	"slices"
)

func AST(rw http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			wrapError(rw, errors.New("Unknown Error"))
		}
	}()
	var astRequest QueryRequest
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
func Interpolate(ds *datasource.HydrolixDatasource, rw http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			rawMessage, _ := json.Marshal(r)
			wrapError(rw, errors.New((string(rawMessage))))
		}
	}()
	var query sqlutil.Query
	if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
		wrapError(rw, err)
		return
	}
	body, err := ds.Interpolator.Interpolate(&query, req.Context())

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
	var astRequest QueryRequest
	if err := json.NewDecoder(req.Body).Decode(&astRequest); err != nil {
		wrapError(rw, err)
		return
	}

	expr, err := parser.NewParser(astRequest.Data.Query).ParseStmts()
	if err != nil {
		wrapError(rw, err)
		return

	}

	body, err := datasource.GetMacroCTEs(expr)
	if err != nil {
		wrapError(rw, err)
		return

	}

	rw.WriteHeader(http.StatusOK)
	marshal, err := json.Marshal(Response[[]datasource.CTE]{
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

func Routes(ds *datasource.HydrolixDatasource) map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"/ast": AST,
		"/interpolate": func(writer http.ResponseWriter, request *http.Request) {
			Interpolate(ds, writer, request)
		},
		"/macroCTE": MacroCTEs,
	}
}

type QueryRequest struct {
	Data struct {
		Query string `json:"query"`
	} `json:"data"`
}
type Response[T any] struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"error_message"`
	Data         T      `json:"data"`
}
