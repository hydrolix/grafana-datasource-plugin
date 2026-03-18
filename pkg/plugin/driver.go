package plugin

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hydrolix/plugin/pkg/datasource"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
	hdxbuild "github.com/hydrolix/plugin/pkg/build"
	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/hydrolix/plugin/pkg/models"
	"github.com/pkg/errors"
)

// Hydrolix defines how to connect to a Hydrolix datasource
type Hydrolix struct {
	querySettingsContextHandler func(context.Context, map[string]any) context.Context
}

var (
	_ sqlds.Driver           = (*Hydrolix)(nil)
	_ sqlds.QueryMutator     = (*Hydrolix)(nil)
	_ sqlds.QueryDataMutator = (*Hydrolix)(nil)

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
		compression = clickhouse.CompressionDeflate
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

	if settings.CredentialsType == "userAccount" || settings.CredentialsType == "" {
		opts.Auth = clickhouse.Auth{
			Database: settings.DefaultDatabase,
			Username: settings.UserName,
			Password: settings.Password,
		}

		if protocol == clickhouse.HTTP {
			// basic auth
			if settings.UserName != "" && settings.Password != "" {
				opts.HttpHeaders = map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", settings.UserName, settings.Password)))}
			}
			// native format
			opts.Settings = map[string]any{"hdx_query_output_format": "Native"}
		}
	} else {
		token := ""
		if settings.CredentialsType == "forwardOAuth" {
			oAuthToken, ok := getOAuthToken(args)
			if ok {
				token = oAuthToken
			} else {
				return nil, fmt.Errorf("cannot get auth header")
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
			opts.Settings = map[string]any{"hdx_query_output_format": "Native"}
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

// Settings reads Json Datasource Plugin's configuration
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
		ForwardHeaders: settings.CredentialsType == "forwardOAuth",
	}
}

// MutateQueryData merges datasource's query options with the target query's query options.
func (h *Hydrolix) MutateQueryData(ctx context.Context, req *backend.QueryDataRequest) (context.Context, *backend.QueryDataRequest) {
	pluginSettings, err := models.NewPluginSettings(ctx, *req.PluginContext.DataSourceInstanceSettings)

	if err != nil {
		log.DefaultLogger.Error("failed to parse plugin settings", "err", err)
		return ctx, req
	}
	if pluginSettings.QuerySettings == nil {
		pluginSettings.QuerySettings = []models.QuerySetting{}
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

		if jmsg, err := jsonSet(q.JSON, map[string]any{"querySettings": mergedSettingsArray}); err == nil {
			req.Queries[i].JSON = jmsg
		} else {
			log.DefaultLogger.Error("failed to serialize querySettings", "err", err)
			return ctx, req
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

func getOAuthToken(jmsg json.RawMessage) (string, bool) {
	header, ok := getHeader(backend.OAuthIdentityTokenHeaderName, jmsg)
	if ok && header != "" && strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer "), true
	} else {
		return "", false
	}
}

func getOrgId(jmsg json.RawMessage) (string, bool) {
	return getHeader(OrgIdHeaderKey, jmsg)
}

func getHeader(headerName string, jmsg json.RawMessage) (string, bool) {

	var m map[string]map[string][]string
	if jmsg != nil {
		err := json.Unmarshal(jmsg, &m)
		if err == nil && m != nil && m[datasource.HeaderKey] != nil && m[datasource.HeaderKey][headerName] != nil && len(m[datasource.HeaderKey][headerName]) > 0 {
			header := m[datasource.HeaderKey][headerName][0]
			return header, true
		}
	}
	return "", false
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
