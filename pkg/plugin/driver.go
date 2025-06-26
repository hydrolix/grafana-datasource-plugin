package plugin

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
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
type Hydrolix struct{}

var (
	_ sqlds.Driver       = (*Hydrolix)(nil)
	_ sqlds.QueryMutator = (*Hydrolix)(nil)
)

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
func (h *Hydrolix) Connect(ctx context.Context, config backend.DataSourceInstanceSettings, message json.RawMessage) (*sql.DB, error) {
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

		BlockBufferSize: 2,
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

	// TODO: add config UI for connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(time.Duration(2) * time.Minute)
	db.SetConnMaxLifetime(time.Duration(2) * time.Minute)

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
	log.DefaultLogger.Info("connect datasource", "name", config.Name)
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
		ForwardHeaders: false,
	}
}

// MutateQuery adds user location timezone metadata if it is available. Also, it rounds the Query Time Range to
// specified time interval.
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

	if dataQuery.Meta.TimeZone != "" {
		loc, _ := time.LoadLocation(dataQuery.Meta.TimeZone)
		log.DefaultLogger.Info("Update query context with location info", "location", loc.String())
		ctx = clickhouse.Context(ctx, clickhouse.WithUserLocation(loc))
	}

	return ctx, req
}

// roundTimeRange rounds the time range to provided time interval
func roundTimeRange(timeRange backend.TimeRange, interval string) backend.TimeRange {
	if dInterval, err := time.ParseDuration(interval); err == nil && dInterval.Seconds() >= 1 {
		To := timeRange.To.Round(dInterval)
		From := timeRange.From.Round(dInterval)

		log.DefaultLogger.Debug("Time range rounded", "original", timeRange, "from", From, "to", To, "interval", interval)
		return backend.TimeRange{To: To, From: From}
	}

	log.DefaultLogger.Warn("Using default time range, provided round interval is invalid", "interval", interval)
	return timeRange
}

// MutateResponse converts fields of type FieldTypeNullableJSON to string, except for specific visualizations - traces,
// tables, and logs.
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
