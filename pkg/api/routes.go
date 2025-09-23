package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/datasource"
	"github.com/hydrolix/plugin/pkg/models"
	"maps"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"time"
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
func Interpolate(ds *datasource.HydrolixDatasource, rw http.ResponseWriter, req *http.Request) {
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

	body, err := ds.Interpolator.Interpolate(
		&datasource.HDXQuery{
			RawSQL:    request.Data.RawSql,
			Filters:   request.Data.Filters,
			Round:     request.Data.Round,
			Interval:  interval,
			TimeRange: timeRange,
		}, req.Context())

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

func NewAssistantProxyHandler(routePrefix string, settings models.PluginSettings) (http.HandlerFunc, error) {
	errorHandler := func(rw http.ResponseWriter, req *http.Request, err error) {
		status := http.StatusBadGateway
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusRequestTimeout
		}

		log.DefaultLogger.Error(
			"Assistant proxy error",
			"method", req.Method,
			"url", req.URL.String(),
			"status", status,
			"error", err,
		)

		response, _ := json.Marshal(map[string]string{"detail": err.Error()})

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(status)
		_, _ = rw.Write(response)
	}
	aiBaseUrl := settings.AiBaseUrl
	if aiBaseUrl == nil {
		defaultUrl, err := url.Parse(fmt.Sprintf("https://%s:4040/assistant", settings.Host))
		if err != nil {
			return nil, err
		}
		aiBaseUrl = defaultUrl
	}

	proxy := httputil.NewSingleHostReverseProxy(aiBaseUrl)
	proxy.ErrorHandler = errorHandler

	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, routePrefix)
		if settings.CredentialsType == "serviceAccount" {
			req.Header.Set("Authorization", "Bearer "+settings.Token)
		} else {
			auth := settings.UserName + ":" + settings.Password
			auth = base64.StdEncoding.EncodeToString([]byte(auth))
			req.Header.Set("Authorization", "Basic "+auth)
		}
	}

	return proxy.ServeHTTP, nil
}

func Routes(ds *datasource.HydrolixDatasource, ctx context.Context, settings backend.DataSourceInstanceSettings) (map[string]func(http.ResponseWriter, *http.Request), error) {
	pluginSettings, err := models.NewPluginSettings(ctx, settings)
	if err != nil {
		return nil, err
	}

	assistantPrefix := "/assistant"
	assistantHandler, err := NewAssistantProxyHandler(assistantPrefix, pluginSettings)
	if err != nil {
		return nil, err
	}

	return map[string]func(http.ResponseWriter, *http.Request){
		"/ast": AST,
		"/interpolate": func(writer http.ResponseWriter, request *http.Request) {
			Interpolate(ds, writer, request)
		},
		"/macroCTE":           MacroCTEs,
		assistantPrefix + "/": assistantHandler,
	}, nil
}

type Request[T any] struct {
	Data T
}
type QueryData struct {
	RawSql   string                   `json:"rawSql"`
	Round    string                   `json:"round"`
	Filters  []datasource.AdHocFilter `json:"filters"`
	Range    Range                    `json:"range"`
	Interval string                   `json:"interval"`
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
