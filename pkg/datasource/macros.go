package datasource

import (
	"context"
	"fmt"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"math"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

type MacroFunc func(*sqlutil.Query, []string, parser.Pos, *MetaDataProvider, context.Context) (string, error)

// Converts a time.Time to a Date
func timeToDate(t time.Time) string {
	return fmt.Sprintf("toDate('%s')", t.Format("2006-01-02"))
}

// Converts a time.Time to a UTC DateTime with seconds precision
func timeToDateTime(t time.Time) string {
	return fmt.Sprintf("toDateTime(%d)", t.Unix())
}

// Converts a time.Time to a UTC DateTime64 with milliseconds precision
func timeToDateTime64(t time.Time) string {
	return fmt.Sprintf("fromUnixTimestamp64Milli(%d)", t.UnixMilli())
}

// FromTimeFilter returns a time filter expression based on grafana's timepicker's "from" time in seconds
func FromTimeFilter(query *sqlutil.Query, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime(query.TimeRange.From), nil
}

// ToTimeFilter returns a time filter expression based on grafana's timepicker's "to" time in seconds
func ToTimeFilter(query *sqlutil.Query, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime(query.TimeRange.To), nil
}

// FromTimeFilterMs returns a time filter expression based on grafana's timepicker's "from" time in milliseconds
func FromTimeFilterMs(query *sqlutil.Query, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime64(query.TimeRange.From), nil
}

// ToTimeFilterMs returns a time filter expression based on grafana's timepicker's "to" time in milliseconds
func ToTimeFilterMs(query *sqlutil.Query, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime64(query.TimeRange.To), nil
}

func TimeFilter(query *sqlutil.Query, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
	if len(args) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}

	var (
		column string
		from   = query.TimeRange.From
		to     = query.TimeRange.To
	)

	if len(args) == 1 && args[0] != "" {
		column = args[0]
	} else {
		pk, err := getPK(query.RawSQL, pos, mdProvider, context)
		if err != nil {
			return "", err
		}
		column = pk
	}

	return fmt.Sprintf("%s >= %s AND %s <= %s", column, timeToDateTime(from), column, timeToDateTime(to)), nil
}

func TimeFilterMs(query *sqlutil.Query, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
	if len(args) != 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}

	var (
		column string
		from   = query.TimeRange.From
		to     = query.TimeRange.To
	)

	if len(args) == 1 && args[0] != "" {
		column = args[0]
	} else {
		pk, err := getPK(query.RawSQL, pos, mdProvider, context)
		if err != nil {
			return "", err
		}
		column = pk
	}

	return fmt.Sprintf("%s >= %s AND %s <= %s", column, timeToDateTime64(from), column, timeToDateTime64(to)), nil
}

func DateFilter(query *sqlutil.Query, args []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	if len(args) != 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	var (
		column = args[0]
		from   = query.TimeRange.From
		to     = query.TimeRange.To
	)

	return fmt.Sprintf("%s >= %s AND %s <= %s", column, timeToDate(from), column, timeToDate(to)), nil
}

func DateTimeFilter(query *sqlutil.Query, args []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	if len(args) != 2 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 2 arguments, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	var (
		dateColumn = args[0]
		timeColumn = args[1]
		from       = query.TimeRange.From
		to         = query.TimeRange.To
	)

	dateFilter := fmt.Sprintf("(%s >= %s AND %s <= %s)", dateColumn, timeToDate(from), dateColumn, timeToDate(to))
	timeFilter := fmt.Sprintf("(%s >= %s AND %s <= %s)", timeColumn, timeToDateTime(from), timeColumn, timeToDateTime(to))
	return fmt.Sprintf("%s AND %s", dateFilter, timeFilter), nil
}

func TimeInterval(query *sqlutil.Query, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
	if len(args) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	var (
		column string
	)

	if len(args) == 1 && args[0] != "" {
		column = args[0]
	} else {
		pk, err := getPK(query.RawSQL, pos, mdProvider, context)
		if err != nil {
			return "", err
		}
		column = pk
	}

	seconds := math.Max(query.Interval.Seconds(), 1)
	return fmt.Sprintf("toStartOfInterval(toDateTime(%s), INTERVAL %d second)", column, int(seconds)), nil
}

func TimeIntervalMs(query *sqlutil.Query, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
	if len(args) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	var (
		column string
	)

	if len(args) == 1 && args[0] != "" {
		column = args[0]
	} else {
		pk, err := getPK(query.RawSQL, pos, mdProvider, context)
		if err != nil {
			return "", err
		}
		column = pk
	}
	milliseconds := math.Max(float64(query.Interval.Milliseconds()), 1)
	return fmt.Sprintf("toStartOfInterval(toDateTime64(%s, 3), INTERVAL %d millisecond)", column, int(milliseconds)), nil
}

func IntervalSeconds(query *sqlutil.Query, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	seconds := math.Max(query.Interval.Seconds(), 1)
	return fmt.Sprintf("%d", int(seconds)), nil
}

func Stub(_ *sqlutil.Query, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return "1=1", nil
}

func getPK(rawSQL string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
	expr, err := parser.NewParser(rawSQL).ParseStmts()
	if err != nil {
		return rawSQL, err
	}
	macroIds, err := GetMacroCTEs(expr)
	if err != nil {
		return rawSQL, err
	}
	var cte *CTE
	for _, macroCTE := range macroIds {
		if macroCTE.MacroPos == pos {
			cte = &macroCTE
			break
		}
	}
	if cte == nil {
		return rawSQL, fmt.Errorf("no CTE found for macro at pos %d", pos)
	}
	return mdProvider.GetPK(context, cte.Database, cte.Table)
}

// Macros is a map of all macro functions
var Macros = map[string]MacroFunc{
	"adHocFilter":     Stub,
	"conditionalAll":  Stub,
	"fromTime":        FromTimeFilter,
	"toTime":          ToTimeFilter,
	"fromTime_ms":     FromTimeFilterMs,
	"toTime_ms":       ToTimeFilterMs,
	"timeFilter":      TimeFilter,
	"timeFilter_ms":   TimeFilterMs,
	"dateFilter":      DateFilter,
	"dateTimeFilter":  DateTimeFilter,
	"dt":              DateTimeFilter,
	"timeInterval":    TimeInterval,
	"timeInterval_ms": TimeIntervalMs,
	"interval_s":      IntervalSeconds,
}
