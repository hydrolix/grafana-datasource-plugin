package macros_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMacrosInterpolation(t *testing.T) {
	from, _ := time.Parse(time.RFC3339, "2025-02-12T11:45:26.123Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-13T11:45:26.456Z")
	fromSec := from.Unix()
	toSec := to.Unix()
	fromMS := from.UnixMilli()
	toMS := to.UnixMilli()

	testCases := []struct {
		name         string
		origin       string
		interpolated string
	}{
		{origin: "SELECT * FROM foo WHERE $__timeFilter(cast(col as timestamp))",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE cast(col as timestamp) >= toDateTime(%d) AND cast(col as timestamp) <= toDateTime(%d)", fromSec, toSec),
			name:         "timeFilter"},
		{origin: "SELECT * FROM foo WHERE $__timeFilter( cast(col as timestamp) )",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE cast(col as timestamp) >= toDateTime(%d) AND cast(col as timestamp) <= toDateTime(%d)", fromSec, toSec),
			name:         "timeFilter with whitespaces"},
		{origin: "SELECT * FROM foo WHERE $__timeFilter_ms(cast(col as timestamp))",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE cast(col as timestamp) >= fromUnixTimestamp64Milli(%d) AND cast(col as timestamp) <= fromUnixTimestamp64Milli(%d)", fromMS, toMS),
			name:         "timeFilter_ms"},
		{origin: "SELECT * FROM foo WHERE $__timeFilter_ms( cast(col as timestamp) )",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE cast(col as timestamp) >= fromUnixTimestamp64Milli(%d) AND cast(col as timestamp) <= fromUnixTimestamp64Milli(%d)", fromMS, toMS),
			name:         "timeFilter_ms with whitespaces"},
		{origin: "SELECT * FROM foo WHERE ( col >= $__fromTime and col <= $__toTime ) limit 100",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE ( col >= toDateTime(%d) and col <= toDateTime(%d) ) limit 100", fromSec, toSec),
			name:         "fromTime and toTime"},
		{origin: "SELECT * FROM foo WHERE ( col >= $__fromTime ) and ( col <= $__toTime ) limit 100",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE ( col >= toDateTime(%d) ) and ( col <= toDateTime(%d) ) limit 100", fromSec, toSec),
			name:         "fromTime and toTime condition #2"},
		{origin: "SELECT * FROM foo WHERE ( col >= $__fromTime_ms and col <= $__toTime_ms ) limit 100",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE ( col >= fromUnixTimestamp64Milli(%d) and col <= fromUnixTimestamp64Milli(%d) ) limit 100", fromMS, toMS),
			name:         "fromTime_ms and toTime_ms"},
		{origin: "SELECT * FROM foo WHERE ( col >= $__fromTime_ms ) and ( col <= $__toTime_ms ) limit 100",
			interpolated: fmt.Sprintf("SELECT * FROM foo WHERE ( col >= fromUnixTimestamp64Milli(%d) ) and ( col <= fromUnixTimestamp64Milli(%d) ) limit 100", fromMS, toMS),
			name:         "fromTime_ms and toTime_ms condition #2"},
	}

	for _, tc := range testCases {
		driver := &plugin.Hydrolix{}
		t.Run(tc.name, func(t *testing.T) {
			query := &sqlutil.Query{
				RawSQL: tc.origin,
				Table:  "macros_table",
				Column: "macros_column",
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
			}
			interpolatedQuery, err := sqlds.Interpolate(driver, query)
			require.Nil(t, err)
			assert.Equal(t, tc.interpolated, interpolatedQuery)
		})
	}
}
