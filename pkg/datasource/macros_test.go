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
			output:  "select * from foo where column2 IS NULL",
			filters: []AdHocFilter{{Key: "column2", Operator: "=", Value: "null"}},
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
			output:  "select * from foo where (column IS NULL OR column IN ($$a$$, $$c$$))",
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
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where has(arrayColumn, $$value$$)",
			filters: []AdHocFilter{{Key: "arrayColumn", Operator: "=", Value: "value"}},
			name:    "array column with equals",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where not has(arrayColumn, $$test$$)",
			filters: []AdHocFilter{{Key: "arrayColumn", Operator: "!=", Value: "test"}},
			name:    "array column with not equals",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where (has(arrayColumn, $$a$$) OR has(arrayColumn, $$b$$) OR has(arrayColumn, $$c$$))",
			filters: []AdHocFilter{{Key: "arrayColumn", Operator: "=|", Values: []string{"a", "b", "c"}}},
			name:    "array column with multi-value IN",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where (not has(arrayColumn, $$x$$) OR not has(arrayColumn, $$y$$))",
			filters: []AdHocFilter{{Key: "arrayColumn", Operator: "!=|", Values: []string{"x", "y"}}},
			name:    "array column with multi-value NOT IN",
		},
		{
			input:  "select * from foo where $__adHocFilter()",
			output: "select * from foo where column = $$test$$ AND has(arrayColumn, $$prod$$)",
			filters: []AdHocFilter{
				{Key: "column", Operator: "=", Value: "test"},
				{Key: "arrayColumn", Operator: "=", Value: "prod"},
			},
			name: "mixed string and array columns",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where mapColumn['key1'] = $$value1$$",
			filters: []AdHocFilter{{Key: "mapColumn['key1']", Operator: "=", Value: "value1"}},
			name:    "map column with key syntax",
		},
		{
			input:   "select * from foo where $__adHocFilter()",
			output:  "select * from foo where mapColumn['status'] IN ($$active$$, $$pending$$)",
			filters: []AdHocFilter{{Key: "mapColumn['status']", Operator: "=|", Values: []string{"active", "pending"}}},
			name:    "map column with multi-value IN",
		},
		{
			input:  "select * from foo where $__adHocFilter()",
			output: "select * from foo where column = $$test$$ AND mapColumn['env'] = $$prod$$",
			filters: []AdHocFilter{
				{Key: "column", Operator: "=", Value: "test"},
				{Key: "mapColumn['env']", Operator: "=", Value: "prod"},
			},
			name: "mixed string and map columns",
		},
	}
	for i, tc := range tests {
		db, mock, _ := sqlmock.New()

		rows := sqlmock.NewRows([]string{"name", "type"}).
			AddRow("column", "Nullable(String)").
			AddRow("column2", "UInt64").
			AddRow("arrayColumn", "Array(String)").
			AddRow("mapColumn", "Map(String, String)")
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
		keyType  string
		expected string
		wantErr  bool
	}{
		{
			name:     "equals operator",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "value"},
			keyType:  "String",
			expected: "column = $$value$$",
		},
		{
			name:     "equals with empty string",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: ""},
			keyType:  "String",
			expected: "(column = '' OR column = '__empty__')",
		},
		{
			name:     "equals with null string",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "null"},
			keyType:  "String",
			expected: "(column IS NULL OR column = '__null__')",
		},
		{
			name:     "not equals operator",
			filter:   AdHocFilter{Key: "column", Operator: "!=", Value: "value"},
			keyType:  "String",
			expected: "column != $$value$$",
		},
		{
			name:     "not equals with empty string",
			filter:   AdHocFilter{Key: "column", Operator: "!=", Value: ""},
			keyType:  "String",
			expected: "(column != '' AND column != '__empty__')",
		},
		{
			name:     "not equals with null",
			filter:   AdHocFilter{Key: "column", Operator: "!=", Value: "null"},
			keyType:  "String",
			expected: "(column IS NOT NULL OR column != '__null__')",
		},
		{
			name:     "regex match",
			filter:   AdHocFilter{Key: "column", Operator: "=~", Value: "pattern"},
			keyType:  "String",
			expected: "toString(column) LIKE $$pattern$$",
		},
		{
			name:     "regex match with wildcards",
			filter:   AdHocFilter{Key: "column", Operator: "=~", Value: "*test*"},
			keyType:  "String",
			expected: "toString(column) LIKE $$%test%$$",
		},
		{
			name:     "regex not match",
			filter:   AdHocFilter{Key: "column", Operator: "!~", Value: "pattern"},
			keyType:  "String",
			expected: "toString(column) NOT LIKE $$pattern$$",
		},
		{
			name:     "multi-value IN",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"a", "b", "c"}},
			keyType:  "String",
			expected: "column IN ($$a$$, $$b$$, $$c$$)",
		},
		{
			name:     "multi-value IN with null",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"a", "null", "c"}},
			keyType:  "String",
			expected: "(column IS NULL OR column IN ($$a$$, $$c$$))",
		},
		{
			name:     "multi-value IN with empty",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"a", "", "c"}},
			keyType:  "String",
			expected: "column IN ($$a$$, $$$$, $$c$$)",
		},
		{
			name:     "multi-value NOT IN",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{"a", "b", "c"}},
			keyType:  "String",
			expected: "column NOT IN ($$a$$, $$b$$, $$c$$)",
		},
		{
			name:     "multi-value NOT IN with null",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{"a", "null", "c"}},
			keyType:  "String",
			expected: "column IS NOT NULL AND column NOT IN ($$a$$, $$c$$)",
		},
		{
			name:     "single quote escaping",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "val'ue"},
			keyType:  "String",
			expected: "column = $$val'ue$$",
		},
		{
			name:     "less than operator",
			filter:   AdHocFilter{Key: "column", Operator: "<", Value: "100"},
			keyType:  "UInt32",
			expected: "column < $$100$$",
		},
		{
			name:     "greater than operator",
			filter:   AdHocFilter{Key: "column", Operator: ">", Value: "50"},
			keyType:  "UInt32",
			expected: "column > $$50$$",
		},
		{
			name:     "multi-value IN only null",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"null"}},
			keyType:  "String",
			expected: "column IS NULL",
		},
		{
			name:     "multi-value NOT IN only null",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{"null"}},
			keyType:  "String",
			expected: "column IS NOT NULL",
		},
		{
			name:     "array type with has operator",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "value"},
			keyType:  "Array(String)",
			expected: "has(column, $$value$$)",
		},
		{
			name:     "array type with not has operator",
			filter:   AdHocFilter{Key: "column", Operator: "!=", Value: "value"},
			keyType:  "Array(String)",
			expected: "not has(column, $$value$$)",
		},
		{
			name:     "array type with multi-value IN",
			filter:   AdHocFilter{Key: "column", Operator: "=|", Values: []string{"a", "b", "c"}},
			keyType:  "Array(String)",
			expected: "(has(column, $$a$$) OR has(column, $$b$$) OR has(column, $$c$$))",
		},
		{
			name:     "array type with multi-value NOT IN",
			filter:   AdHocFilter{Key: "column", Operator: "!=|", Values: []string{"a", "b", "c"}},
			keyType:  "Array(String)",
			expected: "(not has(column, $$a$$) OR not has(column, $$b$$) OR not has(column, $$c$$))",
		},
		{
			name:     "nullable array type",
			filter:   AdHocFilter{Key: "column", Operator: "=", Value: "value"},
			keyType:  "Array(Nullable(String))",
			expected: "has(column, $$value$$)",
		},
		{
			name:    "array type with unsupported operator",
			filter:  AdHocFilter{Key: "column", Operator: "=~", Value: "pattern"},
			keyType: "Array(String)",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildFilterCondition(tt.filter, tt.keyType)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBuildArrayCondition(t *testing.T) {
	tests := []struct {
		name     string
		filter   AdHocFilter
		expected string
		wantErr  bool
	}{
		{
			name:     "array equals operator",
			filter:   AdHocFilter{Key: "tags", Operator: "=", Value: "production"},
			expected: "has(tags, $$production$$)",
		},
		{
			name:     "array not equals operator",
			filter:   AdHocFilter{Key: "tags", Operator: "!=", Value: "test"},
			expected: "not has(tags, $$test$$)",
		},
		{
			name:     "array multi-value IN",
			filter:   AdHocFilter{Key: "tags", Operator: "=|", Values: []string{"prod", "staging", "dev"}},
			expected: "(has(tags, $$prod$$) OR has(tags, $$staging$$) OR has(tags, $$dev$$))",
		},
		{
			name:     "array multi-value NOT IN",
			filter:   AdHocFilter{Key: "tags", Operator: "!=|", Values: []string{"prod", "staging"}},
			expected: "(not has(tags, $$prod$$) OR not has(tags, $$staging$$))",
		},
		{
			name:     "array with single value in multi-value IN",
			filter:   AdHocFilter{Key: "tags", Operator: "=|", Values: []string{"production"}},
			expected: "(has(tags, $$production$$))",
		},
		{
			name:    "array with less than operator (unsupported)",
			filter:  AdHocFilter{Key: "tags", Operator: "<", Value: "100"},
			wantErr: true,
		},
		{
			name:    "array with greater than operator (unsupported)",
			filter:  AdHocFilter{Key: "tags", Operator: ">", Value: "50"},
			wantErr: true,
		},
		{
			name:    "array with regex operator (unsupported)",
			filter:  AdHocFilter{Key: "tags", Operator: "=~", Value: "pattern"},
			wantErr: true,
		},
		{
			name:    "array with regex not match operator (unsupported)",
			filter:  AdHocFilter{Key: "tags", Operator: "!~", Value: "pattern"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildArrayCondition(tt.filter)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBuildFilterConditionWithMaps(t *testing.T) {
	tests := []struct {
		name     string
		filter   AdHocFilter
		keyType  string
		expected string
		wantErr  bool
	}{
		{
			name:     "map string type with equals",
			filter:   AdHocFilter{Key: "labels['env']", Operator: "=", Value: "prod"},
			keyType:  "Map(String, String)",
			expected: "labels['env'] = $$prod$$",
		},
		{
			name:     "map string type with not equals",
			filter:   AdHocFilter{Key: "labels['status']", Operator: "!=", Value: "inactive"},
			keyType:  "Map(String, String)",
			expected: "labels['status'] != $$inactive$$",
		},
		{
			name:     "map string type with multi-value IN",
			filter:   AdHocFilter{Key: "labels['region']", Operator: "=|", Values: []string{"us-east", "us-west"}},
			keyType:  "Map(String, String)",
			expected: "labels['region'] IN ($$us-east$$, $$us-west$$)",
		},
		{
			name:     "map string type with multi-value NOT IN",
			filter:   AdHocFilter{Key: "labels['env']", Operator: "!=|", Values: []string{"dev", "test"}},
			keyType:  "Map(String, String)",
			expected: "labels['env'] NOT IN ($$dev$$, $$test$$)",
		},
		{
			name:     "map nullable string type",
			filter:   AdHocFilter{Key: "metadata['key']", Operator: "=", Value: "value"},
			keyType:  "Map(String, Nullable(String))",
			expected: "metadata['key'] = $$value$$",
		},
		{
			name:    "map uint type with multi-value IN (error)",
			filter:  AdHocFilter{Key: "counts['total']", Operator: "=|", Values: []string{"100", "200"}},
			keyType: "Map(String, UInt32)",
			wantErr: true,
		},
		{
			name:    "map uint type with multi-value NOT IN (error)",
			filter:  AdHocFilter{Key: "counts['total']", Operator: "!=|", Values: []string{"100", "200"}},
			keyType: "Map(String, UInt32)",
			wantErr: true,
		},
		{
			name:     "map uint type with equals (allowed)",
			filter:   AdHocFilter{Key: "counts['total']", Operator: "=", Value: "100"},
			keyType:  "Map(String, UInt32)",
			expected: "counts['total'] = $$100$$",
		},
		{
			name:     "map with regex match",
			filter:   AdHocFilter{Key: "labels['name']", Operator: "=~", Value: "*prod*"},
			keyType:  "Map(String, String)",
			expected: "toString(labels['name']) LIKE $$%prod%$$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildFilterCondition(tt.filter, tt.keyType)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "cannot apply")
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

// TestAdHocFilterMacroWithExplicitTable tests the AdHocFilterMacro with an explicit table parameter
func TestAdHocFilterMacroWithExplicitTable(t *testing.T) {
	type test struct {
		name    string
		input   string
		output  string
		filters []AdHocFilter
		table   string
	}

	tests := []test{
		{
			input:   "select * from bar where $__adHocFilter(foo)",
			output:  "select * from bar where column = $$test$$",
			filters: []AdHocFilter{{Key: "column", Operator: "=", Value: "test"}},
			table:   "foo",
			name:    "explicit table parameter",
		},
		{
			input:  "select * from bar where $__adHocFilter(baz)",
			output: "select * from bar where column = $$test$$ AND column2 != $$value2$$",
			filters: []AdHocFilter{
				{Key: "column", Operator: "=", Value: "test"},
				{Key: "column2", Operator: "!=", Value: "value2"},
			},
			table: "baz",
			name:  "explicit table with multiple filters",
		},
		{
			input:   "select * from bar where $__adHocFilter(myTable)",
			output:  "select * from bar where 1=1",
			filters: []AdHocFilter{},
			table:   "myTable",
			name:    "explicit table with no filters",
		},
		{
			input:   "select * from bar where $__adHocFilter(foo)",
			output:  "select * from bar where 1=1",
			filters: []AdHocFilter{{Key: "nonexistent", Operator: "=", Value: "test"}},
			table:   "foo",
			name:    "explicit table with filter on non-existent column",
		},
	}

	for i, tc := range tests {
		db, mock, _ := sqlmock.New()

		rows := sqlmock.NewRows([]string{"name", "type"}).
			AddRow("column", "Nullable(String)").
			AddRow("column2", "UInt64")
		mock.ExpectQuery(fmt.Sprintf(AD_HOC_KEY_QUERY, tc.table)).
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

// TestAdHocFilterMacroWithTooManyParams tests that AdHocFilterMacro returns an error with too many parameters
func TestAdHocFilterMacroWithTooManyParams(t *testing.T) {
	db, _, _ := sqlmock.New()

	interpolator := NewInterpolator(&HydrolixDatasource{
		Connector: &MockConnector{
			db:  db,
			uid: "uid-123",
		},
		rowLimit: defaultRowLimit,
	})

	query := &HDXQuery{
		RawSQL:  "select * from foo where $__adHocFilter(table1, table2)",
		Filters: []AdHocFilter{{Key: "column", Operator: "=", Value: "test"}},
	}

	_, err := interpolator.Interpolate(query, context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 0 or 1 argument, received 2")
}
