package plugin

import (
	"fmt"
	"time"
)

// timeToDate emits a ClickHouse `toDate('YYYY-MM-DD')` literal.
func timeToDate(t time.Time) string {
	return fmt.Sprintf("toDate('%s')", t.Format("2006-01-02"))
}

// timeToDateTime emits a ClickHouse `toDateTime(<unix>)` expression — UTC
// DateTime at seconds precision.
func timeToDateTime(t time.Time) string {
	return fmt.Sprintf("toDateTime(%d)", t.Unix())
}

// timeToDateTime64 emits a ClickHouse `fromUnixTimestamp64Milli(<unixMilli>)`
// expression — UTC DateTime64 at millisecond precision.
func timeToDateTime64(t time.Time) string {
	return fmt.Sprintf("fromUnixTimestamp64Milli(%d)", t.UnixMilli())
}
