package macros

import (
	"fmt"
	"time"

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

// Macros for sqlds datasource
var Macros = sqlutil.Macros{
	"fromTime":    FromTimeFilter,
	"toTime":      ToTimeFilter,
	"fromTime_ms": FromTimeFilterMs,
	"toTime_ms":   ToTimeFilterMs,
}
