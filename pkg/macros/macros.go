package macros

import (
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

// TimeToDate Time to a Date
func TimeToDate(t time.Time) string {
	return fmt.Sprintf("toDate('%s')", t.Format(time.DateOnly))
}

// TimeToDateTime Time to a Unix seconds datetime (DateTime UTC)
func TimeToDateTime(t time.Time) string {
	return fmt.Sprintf("toDateTime(%d)", t.Unix())
}

// TimeToDateTime64 Time to a Unix milliseconds datetime (DateTime64 UTC)
func TimeToDateTime64(t time.Time) string {
	return fmt.Sprintf("fromUnixTimestamp64Milli(%d)", t.UnixMilli())
}

// FromTimeFilter the TimeFilter's From to Unix seconds datetime
func FromTimeFilter(query *sqlutil.Query, args []string) (string, error) {
	return TimeToDateTime(query.TimeRange.From), nil
}

// ToTimeFilter the TimeFilter's To to Unix seconds datetime
func ToTimeFilter(query *sqlutil.Query, args []string) (string, error) {
	return TimeToDateTime(query.TimeRange.To), nil
}

// FromTimeFilterMs the TimeFilter's From to Unix milliseconds datetime
func FromTimeFilterMs(query *sqlutil.Query, args []string) (string, error) {
	return TimeToDateTime64(query.TimeRange.From), nil
}

// ToTimeFilterMs the TimeFilter's To to Unix milliseconds datetime
func ToTimeFilterMs(query *sqlutil.Query, args []string) (string, error) {
	return TimeToDateTime64(query.TimeRange.To), nil
}

// TimeFilter the TimeFilter's From <= args[0] AND args[0] <= To as Unix seconds datetime
// args should contain one string element with a column name
func TimeFilter(query *sqlutil.Query, args []string) (string, error) {
	if len(args) != 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}

	var (
		column = args[0]
		from   = query.TimeRange.From
		to     = query.TimeRange.To
	)

	return fmt.Sprintf("%s >= %s AND %s <= %s", column, TimeToDateTime(from), column, TimeToDateTime(to)), nil
}

// TimeFilterMs the TimeFilter's From <= args[0] AND args[0] <= To as Unix milliseconds datetime
// args should contain one string element with a column name
func TimeFilterMs(query *sqlutil.Query, args []string) (string, error) {
	if len(args) != 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}

	var (
		column = args[0]
		from   = query.TimeRange.From
		to     = query.TimeRange.To
	)

	return fmt.Sprintf("%s >= %s AND %s <= %s", column, TimeToDateTime64(from), column, TimeToDateTime64(to)), nil
}

// DateFilter the TimeFilter's From <= args[0] AND args[0] <= To as Date
// args should contain one string element with a column name
func DateFilter(query *sqlutil.Query, args []string) (string, error) {
	if len(args) != 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	var (
		column = args[0]
		from   = query.TimeRange.From
		to     = query.TimeRange.To
	)

	return fmt.Sprintf("%s >= %s AND %s <= %s", column, TimeToDate(from), column, TimeToDate(to)), nil
}

// DateTimeFilter the TimeFilter's From <= args[0] AND args[0] <= To as Date
// AND From <= args[1] AND args[1] <= To as a Unix seconds datetime
// args should contain two string elements. First one is for Date comparision. Second one for DateTime comparision.
func DateTimeFilter(query *sqlutil.Query, args []string) (string, error) {
	if len(args) != 2 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 2 arguments, received %d", sqlutil.ErrorBadArgumentCount, len(args)))
	}
	var (
		dateColumn = args[0]
		timeColumn = args[1]
	)

	dateCondition, err := DateFilter(query, []string{dateColumn})
	if err != nil {
		return "", err
	}
	dateTimeCondition, err := TimeFilter(query, []string{timeColumn})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(%s) AND (%s)", dateCondition, dateTimeCondition), nil
}

// Macros for sqlds datasource
var Macros = sqlutil.Macros{
	"fromTime":       FromTimeFilter,
	"toTime":         ToTimeFilter,
	"fromTime_ms":    FromTimeFilterMs,
	"toTime_ms":      ToTimeFilterMs,
	"timeFilter":     TimeFilter,
	"timeFilter_ms":  TimeFilterMs,
	"dateFilter":     DateFilter,
	"dateTimeFilter": DateTimeFilter,
	"dt":             DateTimeFilter,
}
