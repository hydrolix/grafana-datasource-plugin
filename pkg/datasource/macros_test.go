package datasource

import (
	"context"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeToDate(t *testing.T) {
	d, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")

	expected := "toDate('2014-11-12')"
	result := timeToDate(d)

	if expected != result {
		t.Errorf("unexpected output. expected: %s got: %s", expected, result)
	}
}

func TestTimeToDateTime(t *testing.T) {
	dt := time.Unix(1708430068, 0)

	expected := "toDateTime(1708430068)"
	result := timeToDateTime(dt)

	if expected != result {
		t.Errorf("unexpected output. expected: %s got: %s", expected, result)
	}
}

func TestTimeToDateTime64(t *testing.T) {
	dt := time.UnixMilli(1708430068123)

	expected := "fromUnixTimestamp64Milli(1708430068123)"
	result := timeToDateTime64(dt)

	if expected != result {
		t.Errorf("unexpected output. expected: %s got: %s", expected, result)
	}
}

func TestMacroFromTimeFilter(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.371Z")
	query := HDXQuery{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
		RawSQL: "select foo from foo where bar > $__fromTime",
	}
	tests := []struct {
		want    string
		wantErr bool
		name    string
	}{
		{
			name: "should return timeFilter",
			want: "toDateTime(1415792726)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromTimeFilter(&query, []string{}, 0, &MetaDataProvider{}, context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("macroFromTimeFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMacroToTimeFilter(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.371Z")
	query := HDXQuery{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
		RawSQL: "select foo from foo where bar > $__toTime",
	}
	tests := []struct {
		want    string
		wantErr bool
		name    string
	}{
		{
			name: "should return timeFilter",
			want: "toDateTime(1447328726)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToTimeFilter(&query, []string{}, 0, &MetaDataProvider{}, context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("macroToTimeFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMacroFromTimeFilterMs(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.371Z")
	query := HDXQuery{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
		RawSQL: "select foo from foo where bar > $__fromTime",
	}
	tests := []struct {
		want    string
		wantErr bool
		name    string
	}{
		{
			name: "should return timeFilter_ms",
			want: "fromUnixTimestamp64Milli(1415792726371)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromTimeFilterMs(&query, []string{}, 0, &MetaDataProvider{}, context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("macroFromTimeFilterMs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMacroToTimeFilterMs(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.371Z")
	query := HDXQuery{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
		RawSQL: "select foo from foo where bar > $__toTime",
	}
	tests := []struct {
		want    string
		wantErr bool
		name    string
	}{
		{
			name: "should return timeFilter_ms",
			want: "fromUnixTimestamp64Milli(1447328726371)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToTimeFilterMs(&query, []string{}, 0, &MetaDataProvider{}, context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("macroToTimeFilterMs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMacroDateFilter(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.371Z")
	query := HDXQuery{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	got, err := DateFilter(&query, []string{"dateCol"}, 0, &MetaDataProvider{}, context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "dateCol >= toDate('2014-11-12') AND dateCol <= toDate('2015-11-12')", got)
}

func TestMacroDateTimeFilter(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.371Z")
	query := HDXQuery{
		TimeRange: backend.TimeRange{
			From: from,
			To:   to,
		},
	}
	got, err := DateTimeFilter(&query, []string{"dateCol", "timeCol"}, 0, &MetaDataProvider{}, context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "(dateCol >= toDate('2014-11-12') AND dateCol <= toDate('2015-11-12')) AND (timeCol >= toDateTime(1415792726) AND timeCol <= toDateTime(1447328726))", got)
}

func TestMacroTimeInterval(t *testing.T) {
	query := HDXQuery{
		RawSQL:   "select $__timeInterval(col) from foo",
		Interval: time.Duration(20000000000),
	}
	got, err := TimeInterval(&query, []string{"col"}, 0, &MetaDataProvider{}, context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "toStartOfInterval(toDateTime(col), INTERVAL 20 second)", got)
}

func TestMacroTimeIntervalMs(t *testing.T) {
	query := HDXQuery{
		RawSQL:   "select $__timeInterval_ms(col) from foo",
		Interval: time.Duration(20000000000),
	}
	got, err := TimeIntervalMs(&query, []string{"col"}, 0, &MetaDataProvider{}, context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "toStartOfInterval(toDateTime64(col, 3), INTERVAL 20000 millisecond)", got)
}

func TestMacroIntervalSeconds(t *testing.T) {
	query := HDXQuery{
		RawSQL:   "select toStartOfInterval(col, INTERVAL $__interval_s second) AS time from foo",
		Interval: time.Duration(20000000000),
	}
	got, err := IntervalSeconds(&query, []string{}, 0, &MetaDataProvider{}, context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "20", got)
}

// test sqlds query interpolation with clickhouse filters used
func TestInterpolate(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.123Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.456Z")

	type test struct {
		name   string
		input  string
		output string
	}

	tests := []test{
		{input: "select * from foo where $__timeFilter(cast(sth as timestamp))", output: "select * from foo where cast(sth as timestamp) >= toDateTime(1415792726) AND cast(sth as timestamp) <= toDateTime(1447328726)", name: "clickhouse timeFilter"},
		{input: "select * from foo where $__timeFilter(cast(sth as timestamp) )", output: "select * from foo where cast(sth as timestamp) >= toDateTime(1415792726) AND cast(sth as timestamp) <= toDateTime(1447328726)", name: "clickhouse timeFilter with empty spaces"},
		{input: "select * from foo where $__timeFilter_ms(cast(sth as timestamp))", output: "select * from foo where cast(sth as timestamp) >= fromUnixTimestamp64Milli(1415792726123) AND cast(sth as timestamp) <= fromUnixTimestamp64Milli(1447328726456)", name: "clickhouse timeFilter_ms"},
		{input: "select * from foo where $__timeFilter_ms(cast(sth as timestamp) )", output: "select * from foo where cast(sth as timestamp) >= fromUnixTimestamp64Milli(1415792726123) AND cast(sth as timestamp) <= fromUnixTimestamp64Milli(1447328726456)", name: "clickhouse timeFilter_ms with empty spaces"},
		{input: "select * from foo where ( date >= $__fromTime and date <= $__toTime ) limit 100", output: "select * from foo where ( date >= toDateTime(1415792726) and date <= toDateTime(1447328726) ) limit 100", name: "clickhouse fromTime and toTime"},
		{input: "select * from foo where ( date >= $__fromTime ) and ( date <= $__toTime ) limit 100", output: "select * from foo where ( date >= toDateTime(1415792726) ) and ( date <= toDateTime(1447328726) ) limit 100", name: "clickhouse fromTime and toTime inside a complex clauses"},
		{input: "select * from foo where ( date >= $__fromTime_ms and date <= $__toTime_ms ) limit 100", output: "select * from foo where ( date >= fromUnixTimestamp64Milli(1415792726123) and date <= fromUnixTimestamp64Milli(1447328726456) ) limit 100", name: "clickhouse fromTime_ms and toTime_ms"},
		{input: "select * from foo where ( date >= $__fromTime_ms ) and ( date <= $__toTime_ms ) limit 100", output: "select * from foo where ( date >= fromUnixTimestamp64Milli(1415792726123) ) and ( date <= fromUnixTimestamp64Milli(1447328726456) ) limit 100", name: "clickhouse fromTime_ms and toTime_ms inside a complex clauses"},
	}

	for i, tc := range tests {
		db, _, _ := sqlmock.New()
		interpolator := NewInterpolator(&HydrolixDatasource{

			Connector: &MockConnector{
				db:  db,
				uid: "uid-123",
			},
		})
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			query := &HDXQuery{
				RawSQL: tc.input,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
			}
			interpolatedQuery, err := interpolator.Interpolate(query, context.Background())
			require.Nil(t, err)
			assert.Equal(t, tc.output, interpolatedQuery)
		})
	}
}

// test sqlds query interpolation with clickhouse filters used
func TestInterpolateWithAutomaticParams(t *testing.T) {
	from, _ := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.123Z")
	to, _ := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.456Z")

	type test struct {
		name   string
		input  string
		output string
	}

	tests := []test{
		{input: "select * from foo.bar where $__timeFilter()", output: "select * from foo.bar where timestamp >= toDateTime(1415792726) AND timestamp <= toDateTime(1447328726)", name: "timeFilter auto timestamp empty param"},
		{input: "select * from bar where $__timeFilter()", output: "select * from bar where timestamp >= toDateTime(1415792726) AND timestamp <= toDateTime(1447328726)", name: "timeFilter auto timestamp empty param default db"},
		{input: "select * from foo.bar where $__timeFilter_ms()", output: "select * from foo.bar where timestamp >= fromUnixTimestamp64Milli(1415792726123) AND timestamp <= fromUnixTimestamp64Milli(1447328726456)", name: "timeFilter_ms auto timestamp empty param"},
		{input: "select * from bar where $__timeFilter_ms()", output: "select * from bar where timestamp >= fromUnixTimestamp64Milli(1415792726123) AND timestamp <= fromUnixTimestamp64Milli(1447328726456)", name: "timeFilter_ms auto timestamp empty param default db"},
		{input: "select $__timeInterval() from foo.bar", output: "select toStartOfInterval(toDateTime(timestamp), INTERVAL 1 second) from foo.bar", name: "timeInterval auto timestamp empty param"},
		{input: "select $__timeInterval() from bar", output: "select toStartOfInterval(toDateTime(timestamp), INTERVAL 1 second) from bar", name: "timeInterval auto timestamp empty param default db"},
		{input: "select $__timeInterval_ms() from foo.bar", output: "select toStartOfInterval(toDateTime64(timestamp, 3), INTERVAL 1 millisecond) from foo.bar", name: "timeInterval_ms auto timestamp empty param"},
		{input: "select $__timeInterval_ms() from bar", output: "select toStartOfInterval(toDateTime64(timestamp, 3), INTERVAL 1 millisecond) from bar", name: "timeInterval_ms auto timestamp empty param default db"},
	}

	for i, tc := range tests {
		db, mock, _ := sqlmock.New()

		rows := sqlmock.NewRows([]string{"primary_key"}).AddRow("timestamp")
		mock.ExpectQuery(fmt.Sprintf(PRIMARY_KEY_QUERY_STRING, "foo", "bar")).
			WillReturnRows(rows)
		interpolator := NewInterpolator(&HydrolixDatasource{

			Connector: &MockConnector{
				db:  db,
				uid: "uid-123",
			},
			rowLimit: defaultRowLimit,
		})
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			query := &HDXQuery{
				RawSQL: tc.input,
				TimeRange: backend.TimeRange{
					From: from,
					To:   to,
				},
			}
			interpolatedQuery, err := interpolator.Interpolate(query, context.Background())
			require.Nil(t, err)
			assert.Equal(t, tc.output, interpolatedQuery)
		})
	}
}

func TestNegativeCases(t *testing.T) {

	type test struct {
		name  string
		input string
		error string
	}

	tests := []test{
		{input: "select * from foo.bar where $__timeFilter(arg1, arg2)", error: "unexpected number of arguments: expected 0 or 1 argument, received 2", name: "timeFilter auto timestamp empty param"},
		{input: "select * from foo.bar where $__timeFilter(arg1, arg2", error: "failed to parse macro arguments (missing close bracket?)", name: "timeFilter auto timestamp empty param"},
	}

	for i, tc := range tests {

		interpolator := NewInterpolator(&HydrolixDatasource{
			Connector: &MockConnector{
				uid: "uid-123",
			},
		})
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			query := &HDXQuery{
				RawSQL: tc.input,
			}
			_, err := interpolator.Interpolate(query, context.Background())
			require.Error(t, err, tc.error)
			require.Equal(t, err.Error(), tc.error)
		})
	}
}

func TestAdHocFilterMacro(t *testing.T) {
	type test struct {
		name    string
		input   string
		output  string
		filters []AdHocFilter
	}

	tests := []test{
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where 1=1",
			filters: []AdHocFilter{},
			name:    "no filters test",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column = $$test$$",
			filters: []AdHocFilter{{Key: "column", Operator: "=", Value: "test"}},
			name:    "single equals filter",
		},
		{
			input:  "select * from foo where $__adHocFilter()",
			output: "select * from foo where column = $$test$$ AND column2 != $$value2$$",
			filters: []AdHocFilter{
				{Key: "column", Operator: "=", Value: "test"},
				{Key: "column2", Operator: "!=", Value: "value2"},
			},
			name: "multiple filters",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column IS NULL",
			filters: []AdHocFilter{{Key: "column", Operator: "=", Value: "null"}},
			name:    "null value filter",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where (column = '' OR column = '__empty__')",
			filters: []AdHocFilter{{Key: "column", Operator: "=", Value: ""}},
			name:    "empty value filter",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where toString(column) LIKE $$%pattern%$$",
			filters: []AdHocFilter{{Key: "column", Operator: "=~", Value: "*pattern*"}},
			name:    "regex wildcard filter",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where toString(column) NOT LIKE $$pattern$$",
			filters: []AdHocFilter{{Key: "column", Operator: "!~", Value: "pattern"}},
			name:    "regex not match filter",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column IN ($$a$$, $$b$$, $$c$$)",
			filters: []AdHocFilter{{Key: "column", Operator: "=|", Values: []string{"a", "b", "c"}}},
			name:    "multi-value IN filter",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column NOT IN ($$a$$, $$b$$, $$c$$)",
			filters: []AdHocFilter{{Key: "column", Operator: "!=|", Values: []string{"a", "b", "c"}}},
			name:    "multi-value NOT IN filter",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column IS NULL OR column IN ($$a$$, $$c$$)",
			filters: []AdHocFilter{{Key: "column", Operator: "=|", Values: []string{"a", "null", "c"}}},
			name:    "multi-value IN with null",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column IS NOT NULL AND column NOT IN ($$a$$, $$c$$)",
			filters: []AdHocFilter{{Key: "column", Operator: "!=|", Values: []string{"a", "null", "c"}}},
			name:    "multi-value NOT IN with null",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where 1=1",
			filters: []AdHocFilter{{Key: "nonexistent", Operator: "=", Value: "test"}},
			name:    "filter on non-existent column",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column = $$val'ue$$",
			filters: []AdHocFilter{{Key: "column", Operator: "=", Value: "val'ue"}},
			name:    "value with single quotes",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where column IN ($$$$, $$b$$)",
			filters: []AdHocFilter{{Key: "column", Operator: "=|", Values: []string{"", "b"}}},
			name:    "multi-value IN with empty string",
		},
	}
	for i, tc := range tests {
		db, mock, _ := sqlmock.New()

		rows := sqlmock.NewRows([]string{"name", "type"}).
			AddRow("column", "Nullable(String)").
			AddRow("column2", "UInt64")
		mock.ExpectQuery(fmt.Sprintf(AD_HOC_KEY_QUERY, "foo")).
			WillReturnRows(rows)
		interpolator := NewInterpolator(&HydrolixDatasource{

			Connector: &MockConnector{
				db:  db,
				uid: "uid-123",
			},
			rowLimit: defaultRowLimit,
		})
		t.Run(fmt.Sprintf("[%d/%d] %s", i+1, len(tests), tc.name), func(t *testing.T) {
			query := &HDXQuery{
				RawSQL:  tc.input,
				Filters: tc.filters,
			}
			interpolatedQuery, err := interpolator.Interpolate(query, context.Background())
			require.Nil(t, err)
			assert.Equal(t, tc.output, interpolatedQuery)
		})
	}
}

func TestBuildFilterCondition(t *testing.T) {
	tests := []struct {
		name     string
		filter   AdHocFilter
		expected string
		wantErr  bool
	}{
		{
			name:     "equals operator",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "value"},
			expected: "column = $$value$$",
		},
		{
			name:     "equals with empty string",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: ""},
			expected: "(column = '' OR column = '__empty__')",
		},
		{
			name:     "equals with null string",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "null"},
			expected: "column IS NULL OR column = __null__",
		},
		{
			name:     "not equals operator",
			filter:   AdHocFilter{Key: "column", Operator: "!=", Value: "value"},
			expected: "column != $$value$$",
		},
		{
			name:     "not equals with empty string",
			filter:   AdHocFilter{Key: "column", Operator: "!=", Value: ""},
			expected: "(column != '' AND column != '__empty__')",
		},
		{
			name:     "not equals with null",
			filter:   AdHocFilter{Key: "column", Operator: "!=", Value: "null"},
			expected: "column IS NOT NULL OR column != __null__",
		},
		{
			name:     "regex match",
			filter:   AdHocFilter{Key: "column", Operator: "=~", Value: "pattern"},
			expected: "toString(column) LIKE $$pattern$$",
		},
		{
			name:     "regex match with wildcards",
			filter:   AdHocFilter{Key: "column", Operator: "=~", Value: "*test*"},
			expected: "toString(column) LIKE $$%test%$$",
		},
		{
			name:     "regex not match",
			filter:   AdHocFilter{Key: "column", Operator: "!~", Value: "pattern"},
			expected: "toString(column) NOT LIKE $$pattern$$",
		},
		{
			name:     "multi-value IN",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"a", "b", "c"}},
			expected: "column IN ($$a$$, $$b$$, $$c$$)",
		},
		{
			name:     "multi-value IN with null",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"a", "null", "c"}},
			expected: "column IS NULL OR column IN ($$a$$, $$c$$)",
		},
		{
			name:     "multi-value IN with empty",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"a", "", "c"}},
			expected: "column IN ($$a$$, $$$$, $$c$$)",
		},
		{
			name:     "multi-value NOT IN",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{"a", "b", "c"}},
			expected: "column NOT IN ($$a$$, $$b$$, $$c$$)",
		},
		{
			name:     "multi-value NOT IN with null",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{"a", "null", "c"}},
			expected: "column IS NOT NULL AND column NOT IN ($$a$$, $$c$$)",
		},
		{
			name:     "single quote escaping",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "val'ue"},
			expected: "column = $$val'ue$$",
		},
		{
			name:     "less than operator",
			filter:   AdHocFilter{Key: "column", Operator: "<", Value: "100"},
			expected: "column < $$100$$",
		},
		{
			name:     "greater than operator",
			filter:   AdHocFilter{Key: "column", Operator: ">", Value: "50"},
			expected: "column > $$50$$",
		},
		{
			name:     "multi-value IN empty values",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{}},
			expected: "",
		},
		{
			name:     "multi-value NOT IN empty values",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{}},
			expected: "",
		},
		{
			name:     "multi-value IN only null",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"null"}},
			expected: "column IS NULL",
		},
		{
			name:     "multi-value NOT IN only null",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{"null"}},
			expected: "column IS NOT NULL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildFilterCondition(tt.filter, true)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestEscapeWildcard(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no wildcards",
			input:    "pattern",
			expected: "pattern",
		},
		{
			name:     "single wildcard",
			input:    "patt*ern",
			expected: "patt%ern",
		},
		{
			name:     "multiple wildcards",
			input:    "*pattern*",
			expected: "%pattern%",
		},
		{
			name:     "only wildcards",
			input:    "***",
			expected: "%%%",
		},
		{
			name:     "escaped wildcard",
			input:    "foo\\*bar",
			expected: "foo*bar",
		},
		{
			name:     "escaped and unescaped wildcard",
			input:    "a*b\\*c*d",
			expected: "a%b*c%d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeWildcard(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
