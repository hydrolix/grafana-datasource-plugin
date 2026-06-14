package plugin

import (
	"context"
	"fmt"
	"math"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

// Build-time sentinel: fails to compile if the upstream sentinel for
// argument-count errors is renamed/removed. The macros depend on it for
// error classification by sqlds.handleQuery.
var _ = sqlutil.ErrorBadArgumentCount

// FromTimeFilter expands `$__fromTime` to the panel's lower time bound in
// seconds precision: `toDateTime(<unix>)`.
func FromTimeFilter(_ context.Context, query *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	return timeToDateTime(query.TimeRange.From), nil
}

// ToTimeFilter expands `$__toTime` to the panel's upper time bound in
// seconds precision: `toDateTime(<unix>)`.
func ToTimeFilter(_ context.Context, query *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	return timeToDateTime(query.TimeRange.To), nil
}

// FromTimeFilterMs expands `$__fromTime_ms` to the panel's lower time bound
// in millisecond precision: `fromUnixTimestamp64Milli(<unixMilli>)`.
func FromTimeFilterMs(_ context.Context, query *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	return timeToDateTime64(query.TimeRange.From), nil
}

// ToTimeFilterMs expands `$__toTime_ms` to the panel's upper time bound in
// millisecond precision.
func ToTimeFilterMs(_ context.Context, query *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	return timeToDateTime64(query.TimeRange.To), nil
}

// TimeFilter expands `$__timeFilter(<col>)` (or `$__timeFilter` resolving
// the column via PK lookup) into a seconds-precision range comparison
// against the panel's time bounds.
func TimeFilter(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, mdProvider *MetadataProvider) (string, error) {
	if len(args) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	column, err := resolveColumnArg(ctx, query, args, pos, mdProvider)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s >= %s AND %s <= %s",
		column, timeToDateTime(query.TimeRange.From),
		column, timeToDateTime(query.TimeRange.To)), nil
}

// TimeFilterMs is TimeFilter at millisecond precision.
func TimeFilterMs(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, mdProvider *MetadataProvider) (string, error) {
	if len(args) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	column, err := resolveColumnArg(ctx, query, args, pos, mdProvider)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s >= %s AND %s <= %s",
		column, timeToDateTime64(query.TimeRange.From),
		column, timeToDateTime64(query.TimeRange.To)), nil
}

// DateFilter expands `$__dateFilter(<col>)` to a Date-only range comparison.
func DateFilter(_ context.Context, query *models.HdxQuery, args []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	if len(args) != 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	column := args[0]
	return fmt.Sprintf("%s >= %s AND %s <= %s",
		column, timeToDate(query.TimeRange.From),
		column, timeToDate(query.TimeRange.To)), nil
}

// DateTimeFilter expands `$__dateTimeFilter(<dateCol>, <timeCol>)` (and its
// `$__dt(...)` alias) to AND-joined Date + DateTime comparisons.
func DateTimeFilter(_ context.Context, query *models.HdxQuery, args []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	if len(args) != 2 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 2 arguments, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	dateColumn := args[0]
	timeColumn := args[1]
	from := query.TimeRange.From
	to := query.TimeRange.To

	dateFilter := fmt.Sprintf("(%s >= %s AND %s <= %s)",
		dateColumn, timeToDate(from), dateColumn, timeToDate(to))
	timeFilter := fmt.Sprintf("(%s >= %s AND %s <= %s)",
		timeColumn, timeToDateTime(from), timeColumn, timeToDateTime(to))
	return dateFilter + " AND " + timeFilter, nil
}

// TimeInterval expands `$__timeInterval(<col>)` (or `$__timeInterval`
// resolving the column via PK lookup) to a `toStartOfInterval` bucketing
// expression at seconds precision. Sub-second intervals floor to 1 second.
func TimeInterval(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, mdProvider *MetadataProvider) (string, error) {
	if len(args) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	column, err := resolveColumnArg(ctx, query, args, pos, mdProvider)
	if err != nil {
		return "", err
	}
	seconds := int(math.Max(query.Interval.Seconds(), 1))
	return fmt.Sprintf("toStartOfInterval(toDateTime(%s), INTERVAL %d second)", column, seconds), nil
}

// TimeIntervalMs is TimeInterval at millisecond precision.
func TimeIntervalMs(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, mdProvider *MetadataProvider) (string, error) {
	if len(args) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	column, err := resolveColumnArg(ctx, query, args, pos, mdProvider)
	if err != nil {
		return "", err
	}
	ms := int(math.Max(float64(query.Interval.Milliseconds()), 1))
	return fmt.Sprintf("toStartOfInterval(toDateTime64(%s, 3), INTERVAL %d millisecond)", column, ms), nil
}

// IntervalSeconds expands `$__interval_s` to the panel's effective interval
// in seconds, floored to 1.
func IntervalSeconds(_ context.Context, query *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	seconds := int(math.Max(query.Interval.Seconds(), 1))
	return fmt.Sprintf("%d", seconds), nil
}

// resolveColumnArg returns the column the macro should bucket on: either
// the explicit args[0], or the primary key of the table at pos when args
// is empty / args[0] is empty. getPK is defined in metadata.go (C7).
func resolveColumnArg(ctx context.Context, query *models.HdxQuery, args []string, pos parser.Pos, mdProvider *MetadataProvider) (string, error) {
	if len(args) == 1 && args[0] != "" {
		return args[0], nil
	}
	return getPK(ctx, query.RawSQL, pos, mdProvider, query.Headers)
}

func init() {
	Macros["fromTime"] = FromTimeFilter
	Macros["toTime"] = ToTimeFilter
	Macros["fromTime_ms"] = FromTimeFilterMs
	Macros["toTime_ms"] = ToTimeFilterMs
	Macros["timeFilter"] = TimeFilter
	Macros["timeFilter_ms"] = TimeFilterMs
	Macros["dateFilter"] = DateFilter
	Macros["dateTimeFilter"] = DateTimeFilter
	Macros["dt"] = DateTimeFilter
	Macros["timeInterval"] = TimeInterval
	Macros["timeInterval_ms"] = TimeIntervalMs
	Macros["interval_s"] = IntervalSeconds
}
