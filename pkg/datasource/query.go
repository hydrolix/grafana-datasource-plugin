package datasource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"net/http"
	"time"

	"github.com/grafana/dataplane/sdata/timeseries"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
)

// GetQuery wraps sqlutil's GetQuery to add headers if needed
func GetQuery(query backend.DataQuery, headers http.Header, setHeaders bool) (*sqlutil.Query, error) {
	model, err := sqlutil.GetQuery(query)
	if err != nil {
		return nil, backend.PluginError(err)
	}

	if setHeaders {
		applyHeaders(model, headers)
	}

	return model, nil
}

type DBQuery struct {
	DB         sqlds.Connection
	fillMode   *data.FillMissing
	Settings   backend.DataSourceInstanceSettings
	metrics    sqlds.Metrics
	DSName     string
	converters []sqlutil.Converter
	rowLimit   int64
}

func NewQuery(db sqlds.Connection, settings backend.DataSourceInstanceSettings, converters []sqlutil.Converter, fillMode *data.FillMissing, rowLimit int64) *DBQuery {
	return &DBQuery{
		DB:         db,
		DSName:     settings.Name,
		converters: converters,
		fillMode:   fillMode,
		metrics:    sqlds.NewMetrics(settings.Name, settings.Type, sqlds.EndpointQuery),
		rowLimit:   rowLimit,
	}
}

// Run sends the query to the connection and converts the rows to a dataframe.
func (q *DBQuery) Run(ctx context.Context, query *sqlutil.Query, args ...interface{}) (data.Frames, error) {
	start := time.Now()
	rows, err := q.DB.QueryContext(ctx, query.RawSQL, args...)
	if err != nil {
		errType := sqlds.ErrorQuery
		if errors.Is(err, context.Canceled) {
			errType = context.Canceled
		}
		var errWithSource error
		switch err.(type) {
		default:
			errWithSource = backend.DownstreamError(fmt.Errorf("%w: %s", errType, err.Error()))
		case *proto.Exception:
			errWithSource = backend.DownstreamError(fmt.Errorf("Code: %d. %s: %s", err.(*proto.Exception).Code, err.(*proto.Exception).Name, err.(*proto.Exception).Message))
		}
		//errWithSource := backend.DownstreamError(fmt.Errorf("%w: %s", errType, err.Error()))
		q.metrics.CollectDuration(sqlds.SourceDownstream, sqlds.StatusError, time.Since(start).Seconds())
		return sqlutil.ErrorFrameFromQuery(query), errWithSource
	}
	q.metrics.CollectDuration(sqlds.SourceDownstream, sqlds.StatusOK, time.Since(start).Seconds())

	// Check for an error response
	if err := rows.Err(); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Should we even response with an error here?
			// The panel will simply show "no data"
			errWithSource := backend.DownstreamError(fmt.Errorf("%s: %w", "No results from query", err))
			return sqlutil.ErrorFrameFromQuery(query), errWithSource
		}
		errWithSource := backend.DownstreamError(fmt.Errorf("%s: %w", "Error response from database", err))
		q.metrics.CollectDuration(sqlds.SourceDownstream, sqlds.StatusError, time.Since(start).Seconds())
		return sqlutil.ErrorFrameFromQuery(query), errWithSource
	}

	defer func() {
		if err := rows.Close(); err != nil {
			backend.Logger.Error(err.Error())
		}
	}()

	start = time.Now()
	// Convert the response to frames
	res, err := getFrames(rows, q.rowLimit, q.converters, q.fillMode, query)
	if err != nil {
		// We default to plugin error source
		errSource := backend.ErrorSourcePlugin
		if backend.IsDownstreamHTTPError(err) || isProcessingDownstreamError(err) {
			errSource = backend.ErrorSourceDownstream
		}
		errWithSource := backend.NewErrorWithSource(fmt.Errorf("%w: %s", err, "Could not process SQL results"), errSource)
		q.metrics.CollectDuration(sqlds.Source(errSource), sqlds.StatusError, time.Since(start).Seconds())
		return sqlutil.ErrorFrameFromQuery(query), errWithSource
	}

	q.metrics.CollectDuration(sqlds.SourcePlugin, sqlds.StatusOK, time.Since(start).Seconds())
	return res, nil
}

func getFrames(rows *sql.Rows, limit int64, converters []sqlutil.Converter, fillMode *data.FillMissing, query *sqlutil.Query) (data.Frames, error) {
	frame, err := sqlutil.FrameFromRows(rows, limit, converters...)
	if err != nil {
		return nil, err
	}
	frame.Name = query.RefID
	if frame.Meta == nil {
		frame.Meta = &data.FrameMeta{}
	}

	count, err := frame.RowLen()
	if err != nil {
		return nil, err
	}

	// the handling of zero-rows differs between various "format"s.
	zeroRows := count == 0

	frame.Meta.ExecutedQueryString = query.RawSQL
	frame.Meta.PreferredVisualization = data.VisTypeGraph

	switch query.Format {
	case sqlutil.FormatOptionMulti:
		if zeroRows {
			return nil, sqlds.ErrorNoResults
		}

		if frame.TimeSeriesSchema().Type == data.TimeSeriesTypeLong {

			err = fixFrameForLongToMulti(frame)
			if err != nil {
				return nil, err
			}

			frames, err := timeseries.LongToMulti(&timeseries.LongFrame{frame})
			if err != nil {
				return nil, err
			}
			return frames.Frames(), nil
		}
	case sqlutil.FormatOptionTable:
		frame.Meta.PreferredVisualization = data.VisTypeTable
	case sqlutil.FormatOptionLogs:
		frame.Meta.PreferredVisualization = data.VisTypeLogs
	case sqlutil.FormatOptionTrace:
		frame.Meta.PreferredVisualization = data.VisTypeTrace
	// Format as timeSeries
	default:
		if zeroRows {
			return nil, sqlds.ErrorNoResults
		}

		if frame.TimeSeriesSchema().Type == data.TimeSeriesTypeLong {
			frame, err = data.LongToWide(frame, fillMode)
			if err != nil {
				return nil, err
			}
		}
	}
	return data.Frames{frame}, nil
}

// fixFrameForLongToMulti edits the passed in frame so that it's first time field isn't nullable and has the correct meta
func fixFrameForLongToMulti(frame *data.Frame) error {
	if frame == nil {
		return fmt.Errorf("can not convert to wide series, input is nil")
	}

	timeFields := frame.TypeIndices(data.FieldTypeTime, data.FieldTypeNullableTime)
	if len(timeFields) == 0 {
		return fmt.Errorf("can not convert to wide series, input is missing a time field")
	}

	// the timeseries package expects the first time field in the frame to be non-nullable and ignores the rest
	timeField := frame.Fields[timeFields[0]]
	if timeField.Type() == data.FieldTypeNullableTime {
		newValues := []time.Time{}
		for i := 0; i < timeField.Len(); i++ {
			val, ok := timeField.ConcreteAt(i)
			if !ok {
				return fmt.Errorf("can not convert to wide series, input has null time values")
			}
			newValues = append(newValues, val.(time.Time))
		}
		newField := data.NewField(timeField.Name, timeField.Labels, newValues)
		newField.Config = timeField.Config
		frame.Fields[timeFields[0]] = newField

		// LongToMulti requires the meta to be set for the frame
		if frame.Meta == nil {
			frame.Meta = &data.FrameMeta{}
		}
		frame.Meta.Type = data.FrameTypeTimeSeriesLong
		frame.Meta.TypeVersion = data.FrameTypeVersion{0, 1}
	}
	return nil
}

func isProcessingDownstreamError(err error) bool {
	downstreamErrors := []error{
		data.ErrorInputFieldsWithoutRows,
		data.ErrorSeriesUnsorted,
		data.ErrorNullTimeValues,
	}
	for _, e := range downstreamErrors {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
