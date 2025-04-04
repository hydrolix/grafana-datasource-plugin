package macros_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
	"github.com/hydrolix/plugin/pkg/macros"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeToDate(t *testing.T) {
	dt, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	dtS := dt.Format(time.DateOnly)

	res := macros.TimeToDate(dt)
	assert.Equal(t, fmt.Sprintf("toDate('%s')", dtS), res)
}

func TestTimeToDateTime(t *testing.T) {
	dt, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	dtSec := dt.Unix()

	res := macros.TimeToDateTime(dt)
	assert.Equal(t, fmt.Sprintf("toDateTime(%d)", dtSec), res)
}

func TestTimeToDateTime64(t *testing.T) {
	dt, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	dtMS := dt.UnixMilli()

	res := macros.TimeToDateTime64(dt)
	assert.Equal(t, fmt.Sprintf("fromUnixTimestamp64Milli(%d)", dtMS), res)
}

func TestMacroFromTimeFilter(t *testing.T) {
	from, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-13T01:02:03.9999Z")
	fromSec := from.Unix()
	// toSec := to.Unix()
	// fromMS := from.UnixMilli()
	// toMS := to.UnixMilli()

	query := sqlutil.Query{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	t.Run("FromTimeFilter", func(t *testing.T) {
		res, err := macros.FromTimeFilter(&query, []string{})
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, fmt.Sprintf("toDateTime(%d)", fromSec), res)
	})
}

func TestMacroToTimeFilter(t *testing.T) {
	from, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-13T01:02:03.9999Z")
	// fromSec := from.Unix()
	toSec := to.Unix()
	// fromMS := from.UnixMilli()
	// toMS := to.UnixMilli()

	query := sqlutil.Query{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	t.Run("ToTimeFilter", func(t *testing.T) {
		res, err := macros.ToTimeFilter(&query, []string{})
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, fmt.Sprintf("toDateTime(%d)", toSec), res)
	})
}

func TestMacroFromTimeFilterMs(t *testing.T) {
	from, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-13T01:02:03.9999Z")
	// fromSec := from.Unix()
	// toSec := to.Unix()
	fromMS := from.UnixMilli()
	// toMS := to.UnixMilli()

	query := sqlutil.Query{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	t.Run("FromTimeFilterMs", func(t *testing.T) {
		res, err := macros.FromTimeFilterMs(&query, []string{})
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, fmt.Sprintf("fromUnixTimestamp64Milli(%d)", fromMS), res)
	})
}

func TestMacroToTimeFilterMs(t *testing.T) {
	from, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-13T01:02:03.9999Z")
	// fromSec := from.Unix()
	// toSec := to.Unix()
	// fromMS := from.UnixMilli()
	toMS := to.UnixMilli()

	query := sqlutil.Query{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	t.Run("ToTimeFilterMs", func(t *testing.T) {
		res, err := macros.ToTimeFilterMs(&query, []string{})
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, fmt.Sprintf("fromUnixTimestamp64Milli(%d)", toMS), res)
	})
}

func TestMacroDateFilter(t *testing.T) {
	from, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-13T01:02:03.9999Z")
	fromD := from.Format(time.DateOnly)
	toD := to.Format(time.DateOnly)

	query := sqlutil.Query{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	res, err := macros.DateFilter(&query, []string{"col"})
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("col >= toDate('%s') AND col <= toDate('%s')", fromD, toD), res)
}

func TestMacroDateTimeFilter(t *testing.T) {
	from, _ := time.Parse(time.RFC3339, "2025-02-12T01:02:03.9999Z")
	to, _ := time.Parse(time.RFC3339, "2025-02-13T01:02:03.9999Z")
	fromD := from.Format(time.DateOnly)
	toD := to.Format(time.DateOnly)
	fromSec := from.Unix()
	toSec := to.Unix()

	query := sqlutil.Query{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	res, err := macros.DateTimeFilter(&query, []string{"dateCol", "timeCol"})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("(dateCol >= toDate('%s') AND dateCol <= toDate('%s')) AND (timeCol >= toDateTime(%d) AND timeCol <= toDateTime(%d))", fromD, toD, fromSec, toSec), res)
}

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
