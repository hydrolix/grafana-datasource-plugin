package plugin

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/hydrolix/plugin/pkg/macros"
	"github.com/hydrolix/plugin/pkg/models"
	"github.com/pkg/errors"
)

// Hydrolix defines how to connect to a Hydrolix datasource
type Hydrolix struct{}

func getClientInfoProducts(ctx context.Context) (products []struct{ Name, Version string }) {
	version := backend.UserAgentFromContext(ctx).GrafanaVersion()

	if version != "" {
		products = append(products, struct{ Name, Version string }{
			Name:    "grafana",
			Version: version,
		})
	}

	if info, err := build.GetBuildInfo(); err == nil {
		products = append(products, struct{ Name, Version string }{
			Name:    models.HydrolixPluginName,
			Version: info.Version,
		})
	}

	return products
}

func CheckMinServerVersion(conn *sql.DB, major, minor, patch uint64) (bool, error) {
	var version struct {
		Major uint64
		Minor uint64
		Patch uint64
	}
	var res string
	if err := conn.QueryRow("SELECT version()").Scan(&res); err != nil {
		return false, err
	}
	for i, v := range strings.Split(res, ".") {
		switch i {
		case 0:
			version.Major, _ = strconv.ParseUint(v, 10, 64)
		case 1:
			version.Minor, _ = strconv.ParseUint(v, 10, 64)
		case 2:
			version.Patch, _ = strconv.ParseUint(v, 10, 64)
		}
	}
	if version.Major < major || (version.Major == major && version.Minor < minor) || (version.Major == major && version.Minor == minor && version.Patch < patch) {
		return false, nil
	}
	return true, nil
}

// Connect opens a sql.DB connection using datasource settings
func (h *Hydrolix) Connect(ctx context.Context, config backend.DataSourceInstanceSettings, message json.RawMessage) (*sql.DB, error) {
	settings, err := models.LoadPluginSettings(ctx, config)
	if err != nil {
		return nil, err
	}

	dt, err := strconv.Atoi(settings.DialTimeout)
	if err != nil {
		return nil, backend.DownstreamError(errors.New(fmt.Sprintf("invalid timeout: %s", settings.DialTimeout)))
	}
	qt, err := strconv.Atoi(settings.QueryTimeout)
	if err != nil {
		return nil, backend.DownstreamError(errors.New(fmt.Sprintf("invalid query timeout: %s", settings.QueryTimeout)))
	}

	protocol := clickhouse.Native
	if settings.Protocol == "http" {
		protocol = clickhouse.HTTP
	}

	compression := clickhouse.CompressionLZ4
	if protocol == clickhouse.HTTP {
		compression = clickhouse.CompressionGZIP
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
		Auth: clickhouse.Auth{
			Database: settings.DefaultDatabase,
			Username: settings.UserName,
			Password: settings.Password,
		},
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
	}

	if protocol == clickhouse.HTTP {
		// https & basic auth
		if settings.Secure {
			opts.HttpHeaders = map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", settings.UserName, settings.Password)))}
		}

		// native format
		opts.Settings = map[string]any{"hdx_query_output_format": "Native"}
	}

	db := clickhouse.OpenDB(opts)

	select {
	case <-ctx.Done():
		return db, fmt.Errorf("connect to database was cancelled: %w", ctx.Err())
	default:
		err := db.PingContext(ctx)
		if err != nil {
			var ex *clickhouse.Exception
			if errors.As(err, &ex) {
				log.DefaultLogger.Error("[%d] %s \n%s\n", ex.Code, ex.Message, ex.StackTrace)
			}
			return db, err
		}
	}

	return db, nil
}

// Converters defines list of data type converters
func (h *Hydrolix) Converters() []sqlutil.Converter {
	return converters.Converters
}

// Macros returns list of macro functions convert the macros of raw query
func (h *Hydrolix) Macros() sqlds.Macros {
	return macros.Macros
}

func (h *Hydrolix) Settings(ctx context.Context, config backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	settings, err := models.LoadPluginSettings(ctx, config)
	timeout := 60
	if err == nil {
		t, err := strconv.Atoi(settings.QueryTimeout)
		if err == nil {
			timeout = t
		}
	}

	return sqlds.DriverSettings{
		Timeout: time.Second * time.Duration(timeout),
		FillMode: &data.FillMissing{
			Mode: data.FillModeNull,
		},
		ForwardHeaders: false,
	}
}

func (h *Hydrolix) MutateQuery(ctx context.Context, req backend.DataQuery) (context.Context, backend.DataQuery) {
	var dataQuery struct {
		Meta struct {
			TimeZone string `json:"timezone"`
		} `json:"meta"`
		Format int    `json:"format"`
		Round  string `json:"round"`
	}

	if err := json.Unmarshal(req.JSON, &dataQuery); err != nil {
		return ctx, req
	}

	if dataQuery.Round != "" && dataQuery.Round != "0" {
		req.TimeRange = roundTimeRange(req.TimeRange, dataQuery.Round)
	}

	if dataQuery.Meta.TimeZone == "" {
		return ctx, req
	}

	loc, _ := time.LoadLocation(dataQuery.Meta.TimeZone)
	return clickhouse.Context(ctx, clickhouse.WithUserLocation(loc)), req
}

func roundTimeRange(timeRange backend.TimeRange, round string) backend.TimeRange {
	if d, err := time.ParseDuration(round); err != nil {
		log.DefaultLogger.Warn("invalid round time range, using default: %s", round)
		return timeRange
	} else {
		To := timeRange.To.Round(d)
		From := timeRange.From.Round(d)
		return backend.TimeRange{To: To, From: From}
	}

}

// MutateResponse converts fields of type FieldTypeNullableJSON to string,
// except for specific visualizations (traces, tables, and logs).
func (h *Hydrolix) MutateResponse(ctx context.Context, res data.Frames) (data.Frames, error) {
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
