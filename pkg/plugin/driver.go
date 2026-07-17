package plugin

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	hdxbuild "github.com/hydrolix/plugin/pkg/build"
	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/pkg/errors"
)

// Hydrolix defines how to connect to a Hydrolix datasource
type Hydrolix struct {
	querySettingsContextHandler func(context.Context, map[string]any) context.Context
}

var (
	_ sqlds.Driver            = (*Hydrolix)(nil)
	_ sqlds.QueryMutator      = (*Hydrolix)(nil)
	_ sqlds.QueryDataMutator  = (*Hydrolix)(nil)
	_ sqlds.QueryErrorMutator = (*Hydrolix)(nil)

	OrgIdHeaderKey = "X-Grafana-Org-Id"
)

// NewHydrolix creates plugin instance with default parameters
func NewHydrolix() *Hydrolix {
	return &Hydrolix{querySettingsContextHandler: clickhouseContextHandler}
}

// getClientInfoProducts reads build information of grafana and plugin
func getClientInfoProducts(ctx context.Context) (products []struct{ Name, Version string }) {
	version := backend.UserAgentFromContext(ctx).GrafanaVersion()

	if version != "" {
		products = append(products, struct{ Name, Version string }{
			Name:    "grafana",
			Version: version,
		})
	}

	info := hdxbuild.BuildInfo{}.GetBuildInfo()
	products = append(products, struct{ Name, Version string }{
		Name:    info.PluginID,
		Version: info.Version,
	})

	return products
}

// Connect opens a sql.DB connection using datasource settings
func (h *Hydrolix) Connect(ctx context.Context, config backend.DataSourceInstanceSettings, args json.RawMessage) (*sql.DB, error) {
	settings, err := models.NewPluginSettings(ctx, config)
	if err != nil {
		return nil, err
	}

	dt, _ := strconv.Atoi(settings.DialTimeout)
	qt, _ := strconv.Atoi(settings.QueryTimeout)

	protocol := clickhouse.Native
	if settings.Protocol == "http" {
		protocol = clickhouse.HTTP
	}

	compression := clickhouse.CompressionLZ4
	if protocol == clickhouse.HTTP {
		compression = clickhouse.CompressionNone
	}

	var tlsConfig *tls.Config
	if settings.Secure {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: settings.SkipTlsVerify,
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(dt)*time.Second)
	defer cancel()

	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", settings.Host, settings.Port)},

		ClientInfo: clickhouse.ClientInfo{
			Products: getClientInfoProducts(ctx),
		},
		Compression: &clickhouse.Compression{
			Method: compression,
		},
		Protocol:    protocol,
		HttpUrlPath: settings.Path,
		DialTimeout: time.Duration(dt) * time.Second,
		ReadTimeout: time.Duration(qt) * time.Second,
		TLS:         tlsConfig,

		BlockBufferSize: 2,
	}

	opts.TransportFunc = func(t *http.Transport) (http.RoundTripper, error) {
		t.DisableCompression = false
		return t, nil
	}

	if settings.CredentialsType == "userAccount" || settings.CredentialsType == "" {
		opts.Auth = clickhouse.Auth{
			Database: settings.DefaultDatabase,
			Username: settings.UserName,
			Password: settings.Password,
		}

		if protocol == clickhouse.HTTP {
			// basic auth
			if settings.UserName != "" && settings.Password != "" {
				opts.HttpHeaders = map[string]string{
					"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", settings.UserName, settings.Password))),
				}
			}
			// native format
			opts.Settings = map[string]any{
				"hdx_query_output_format": "Native",
				//"hdx_query_streaming_result": "true",
			}
		}
	} else {
		token := ""
		if settings.CredentialsType == "forwardOAuth" {
			// Two callers reach here:
			//   1) sqlds.NewConnector's bootstrap call with args == nil. No
			//      token yet; build a no-auth *sql.DB. sql.OpenDB does not
			//      contact the server, and the forwardOAuth branch below
			//      skips PingContext, so this entry is safe to sit idle in
			//      the cache. Per-user DBs land on first per-query call once
			//      MutateQueryData has populated connectionArgs.oauthToken.
			//   2) Per-query call with args != nil. The token must be
			//      present; missing is a real error.
			if args == nil {
				token = ""
			} else if oAuthToken, ok := getOAuthToken(args); ok {
				token = oAuthToken
			} else {
				return nil, backend.DownstreamError(fmt.Errorf("forwardOAuth: missing OAuth token in connection args"))
			}
		} else {
			token = settings.Token
		}

		if protocol == clickhouse.HTTP {
			opts.Auth = clickhouse.Auth{
				Database: settings.DefaultDatabase,
			}
			httpHeaders := make(map[string]string, 2)

			orgId, ok := getOrgId(args)
			if ok {
				httpHeaders[OrgIdHeaderKey] = orgId
			}
			if token != "" {
				httpHeaders[backend.OAuthIdentityTokenHeaderName] = "Bearer " + token
			}

			if len(httpHeaders) > 0 {
				opts.HttpHeaders = httpHeaders
			}
			// native format
			opts.Settings = map[string]any{
				"hdx_query_output_format": "Native",
				//"hdx_query_streaming_result": "true",
			}
		} else {
			opts.Auth = clickhouse.Auth{
				Database: settings.DefaultDatabase,
				Username: "__api_token__",
				Password: token,
			}
		}
	}

	db := clickhouse.OpenDB(opts)

	// TODO: add config UI for connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(time.Duration(2) * time.Minute)
	db.SetConnMaxLifetime(time.Duration(2) * time.Minute)

	select {
	case <-ctx.Done():
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("connect to database was cancelled: %w", ctx.Err())
	default:
		if settings.CredentialsType != "forwardOAuth" {
			err := db.PingContext(ctx)
			if err != nil {
				var ex *clickhouse.Exception
				if errors.As(err, &ex) {
					log.DefaultLogger.Error(
						"clickhouse exception",
						"code", ex.Code,
						"message", ex.Message,
						"stack", ex.StackTrace,
					)
				}
				if db != nil {
					_ = db.Close()
				}
				return nil, err
			}
		}
	}
	log.DefaultLogger.Debug("connect datasource", "name", config.Name)
	return db, nil
}

// Converters defines list of data type converters
func (h *Hydrolix) Converters() []sqlutil.Converter {
	return converters.Converters
}

// Macros returns list of macro functions convert the macros of raw query
func (h *Hydrolix) Macros() sqlutil.Macros {
	return sqlutil.Macros{}
}

// Settings reads Json Datasource Plugin's configuration. ForwardHeaders is
// pinned false here: the OAuth-keying flow (C4) injects connectionArgs via
// Driver.MutateQueryData, and ForwardHeaders=true would otherwise pollute
// the cache key by writing the full HTTP header map into ConnectionArgs.
func (h *Hydrolix) Settings(ctx context.Context, config backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	settings, err := models.NewPluginSettings(ctx, config)
	if err != nil {
		return sqlds.DriverSettings{}
	}

	timeoutSec, _ := strconv.Atoi(settings.QueryTimeout)

	return sqlds.DriverSettings{
		Timeout: time.Second * time.Duration(timeoutSec),
		FillMode: &data.FillMissing{
			Mode: data.FillModeNull,
		},
		ForwardHeaders: false,
	}
}

// MutateQueryData merges datasource's query options with the target query's
// query options, and injects per-request connectionArgs (oauthToken, orgId)
// derived from inbound HTTP headers so sqlds keys the connection cache per
// user / per org. See openspec/changes/plugin-oauth-keyed-pooling.
func (h *Hydrolix) MutateQueryData(ctx context.Context, req *backend.QueryDataRequest) (context.Context, *backend.QueryDataRequest) {
	pluginSettings, err := models.NewPluginSettings(ctx, *req.PluginContext.DataSourceInstanceSettings)

	if err != nil {
		log.DefaultLogger.Error("failed to parse plugin settings", "err", err)
		return ctx, req
	}
	if pluginSettings.QuerySettings == nil {
		pluginSettings.QuerySettings = []models.QuerySetting{}
	}

	headers := req.GetHTTPHeaders()
	connArgs := map[string]string{}
	if pluginSettings.CredentialsType == "forwardOAuth" {
		if token := strings.TrimPrefix(headers.Get(backend.OAuthIdentityTokenHeaderName), "Bearer "); token != "" {
			connArgs["oauthToken"] = token
		}
	}
	if org := headers.Get(OrgIdHeaderKey); org != "" {
		connArgs["orgId"] = org
	}

	for i, q := range req.Queries {
		var dataQuery struct {
			RawSql        string                `json:"rawSql"`
			QuerySettings []models.QuerySetting `json:"querySettings,omitempty"`
		}
		_ = json.Unmarshal(q.JSON, &dataQuery)
		mergedSettings := make(map[string]string)
		for _, setting := range pluginSettings.QuerySettings {
			mergedSettings[setting.Setting] = setting.Value
		}
		if dataQuery.QuerySettings != nil {

			for _, setting := range dataQuery.QuerySettings {
				mergedSettings[setting.Setting] = setting.Value
			}
		}

		mergedSettingsArray := make([]models.QuerySetting, len(mergedSettings))
		n := 0
		for k, v := range mergedSettings {
			mergedSettingsArray[n] = models.QuerySetting{
				Setting: k,
				Value:   v,
			}
			n++
		}

		patches := map[string]any{"querySettings": mergedSettingsArray}
		if len(connArgs) > 0 {
			patches["connectionArgs"] = connArgs
		}
		if jmsg, err := jsonSet(q.JSON, patches); err == nil {
			req.Queries[i].JSON = jmsg
		} else {
			log.DefaultLogger.Error("failed to serialize query JSON", "err", err, "refId", q.RefID)
			continue
		}
	}

	return ctx, req
}

// MutateQuery adds user location timezone metadata if it is available. Also, it rounds the Query Time Range to
// specified time interval.
func (h *Hydrolix) MutateQuery(ctx context.Context, req backend.DataQuery) (context.Context, backend.DataQuery) {
	var dataQuery struct {
		Meta struct {
			TimeZone string `json:"timezone"`
		} `json:"meta"`
		Format        int                   `json:"format"`
		Round         string                `json:"round"`
		QuerySettings []models.QuerySetting `json:"querySettings"`
	}

	if err := json.Unmarshal(req.JSON, &dataQuery); err != nil {
		return ctx, req
	}

	if dataQuery.Meta.TimeZone != "" {
		loc, err := time.LoadLocation(dataQuery.Meta.TimeZone)
		if err != nil || loc == nil {
			log.DefaultLogger.Warn("invalid timezone", "tz", dataQuery.Meta.TimeZone)
		} else {
			log.DefaultLogger.Debug("Update query context with location info", "location", loc.String())
			ctx = clickhouse.Context(ctx, clickhouse.WithUserLocation(loc))
		}
	}

	if dataQuery.QuerySettings != nil {
		log.DefaultLogger.Debug("Update query context with settings info", "settings", dataQuery.QuerySettings)
		customSettings := make(map[string]any, len(dataQuery.QuerySettings))
		for _, v := range dataQuery.QuerySettings {
			customSettings[v.Setting] = clickhouse.CustomSetting{Value: fmt.Sprintf("%v", v.Value)}
		}
		ctx = h.querySettingsContextHandler(ctx, customSettings)
	}

	return ctx, req
}

// getOAuthToken returns the OAuth bearer token written into connectionArgs
// by MutateQueryData. The value is the bare token (Bearer-prefix stripped at
// write time); callers that need the wire form prepend "Bearer " themselves.
func getOAuthToken(args json.RawMessage) (string, bool) {
	return readConnArg(args, "oauthToken")
}

// getOrgId returns the X-Grafana-Org-Id value written into connectionArgs by
// MutateQueryData.
func getOrgId(args json.RawMessage) (string, bool) {
	return readConnArg(args, "orgId")
}

// readConnArg decodes connectionArgs as a flat map[string]string and returns
// the requested key. Nil input, malformed JSON, missing keys, and empty
// values all yield ("", false).
func readConnArg(args json.RawMessage, key string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	var m map[string]string
	if err := json.Unmarshal(args, &m); err != nil {
		return "", false
	}
	v, ok := m[key]
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// jsonSet update raw message's root object by applying a value to a key property
func jsonSet(jmsg json.RawMessage, val map[string]any) (json.RawMessage, error) {
	var objmap map[string]interface{}
	err := json.Unmarshal(jmsg, &objmap)
	if err != nil {
		return nil, err
	}
	for k, v := range val {
		objmap[k] = v
	}
	return json.Marshal(objmap)
}

// clickhouseContextHandler applies query options to context
func clickhouseContextHandler(ctx context.Context, settings map[string]any) context.Context {
	return clickhouse.Context(ctx, clickhouse.WithSettings(settings))
}

// MutateResponse converts fields of type FieldTypeNullableJSON to string, except for specific visualizations - traces,
// tables, and logs.
func (h *Hydrolix) MutateResponse(_ context.Context, res data.Frames) (data.Frames, error) {
	for _, frame := range res {
		if shouldConvertFields(frame.Meta.PreferredVisualization) {
			if err := convertNullableJSONFields(frame); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// shouldConvertFields determines whether field conversion is needed based on visualization type.
func shouldConvertFields(visType data.VisType) bool {
	return visType != data.VisTypeTrace && visType != data.VisTypeTable && visType != data.VisTypeLogs
}

// convertNullableJSONFields converts all FieldTypeNullableJSON fields in the given frame to string.
func convertNullableJSONFields(frame *data.Frame) error {
	var convertedFields []*data.Field

	for _, field := range frame.Fields {
		if field.Type() == data.FieldTypeNullableJSON {
			newField, err := convertFieldToString(field)
			if err != nil {
				return err
			}
			convertedFields = append(convertedFields, newField)
		} else {
			convertedFields = append(convertedFields, field)
		}
	}

	frame.Fields = convertedFields
	return nil
}

// convertFieldToString creates a new field where JSON values are marshaled into string representations.
func convertFieldToString(field *data.Field) (*data.Field, error) {
	values := make([]*string, field.Len())
	newField := data.NewField(field.Name, field.Labels, values)
	newField.SetConfig(field.Config)

	for i := 0; i < field.Len(); i++ {
		val, _ := field.At(i).(*json.RawMessage)
		if val == nil {
			newField.Set(i, nil)
		} else {
			bytes, err := val.MarshalJSON()
			if err != nil {
				return nil, err
			}
			sVal := string(bytes)
			newField.Set(i, &sVal)
		}
	}

	return newField, nil
}

func (h *Hydrolix) MutateQueryError(err error) backend.ErrorWithSource {
	if uw, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range uw.Unwrap() {
			if ex, ok := e.(*proto.Exception); ok {
				return backend.NewErrorWithSource(
					backend.DownstreamError(fmt.Errorf("Code: %d. %s: %s", ex.Code, ex.Name, ex.Message)),
					backend.ErrorSourceDownstream,
				)
			}
		}
	}

	var ex *proto.Exception
	if errors.As(err, &ex) {
		return backend.NewErrorWithSource(
			backend.DownstreamError(fmt.Errorf("Code: %d. %s: %s", ex.Code, ex.Name, ex.Message)),
			backend.ErrorSourceDownstream,
		)
	}

	return backend.NewErrorWithSource(
		backend.DownstreamError(err),
		backend.ErrorSourceDownstream,
	)
}
