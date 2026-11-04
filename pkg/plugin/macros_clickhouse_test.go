package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stdRange constructs the standard 2014-11-12 → 2015-11-12 panel range used
// by the fork's macros_test.go fixture cases; keeps expected unix values
// (1415792726 / 1447328726) intact when porting.
func stdRange(t *testing.T) backend.TimeRange {
	t.Helper()
	from, err := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	require.NoError(t, err)
	to, err := time.Parse("2006-01-02T15:04:05.000Z", "2015-11-12T11:45:26.371Z")
	require.NoError(t, err)
	return backend.TimeRange{From: from, To: to}
}

// emptyProvider returns a *MetadataProvider whose schema-query path panics
// — use it in macro tests that supply the column explicitly (no PK
// lookup) or that expect a typed error before PK lookup is reached.
func emptyProvider(t *testing.T) *MetadataProvider {
	t.Helper()
	return NewMetadataProvider(nopMetadataDS{})
}

func TestTimeToDate(t *testing.T) {
	d, err := time.Parse("2006-01-02T15:04:05.000Z", "2014-11-12T11:45:26.371Z")
	require.NoError(t, err)
	assert.Equal(t, "toDate('2014-11-12')", timeToDate(d))
}

func TestTimeToDateTime(t *testing.T) {
	assert.Equal(t, "toDateTime(1708430068)", timeToDateTime(time.Unix(1708430068, 0)))
}

func TestTimeToDateTime64(t *testing.T) {
	assert.Equal(t, "fromUnixTimestamp64Milli(1708430068123)", timeToDateTime64(time.UnixMilli(1708430068123)))
}

func TestMacroFromTimeFilter(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	got, err := FromTimeFilter(context.Background(), q, nil, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "toDateTime(1415792726)", got)
}

func TestMacroToTimeFilter(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	got, err := ToTimeFilter(context.Background(), q, nil, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "toDateTime(1447328726)", got)
}

func TestMacroFromTimeFilterMs(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	got, err := FromTimeFilterMs(context.Background(), q, nil, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "fromUnixTimestamp64Milli(1415792726371)", got)
}

func TestMacroToTimeFilterMs(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	got, err := ToTimeFilterMs(context.Background(), q, nil, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "fromUnixTimestamp64Milli(1447328726371)", got)
}

func TestMacroDateFilter(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	got, err := DateFilter(context.Background(), q, []string{"dateCol"}, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "dateCol >= toDate('2014-11-12') AND dateCol <= toDate('2015-11-12')", got)
}

func TestMacroDateFilter_WrongArgCount(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	_, err := DateFilter(context.Background(), q, nil, 0, emptyProvider(t))
	assert.ErrorIs(t, err, sqlutil.ErrorBadArgumentCount)
	_, err = DateFilter(context.Background(), q, []string{"a", "b"}, 0, emptyProvider(t))
	assert.ErrorIs(t, err, sqlutil.ErrorBadArgumentCount)
}

func TestMacroDateTimeFilter(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	got, err := DateTimeFilter(context.Background(), q, []string{"dateCol", "timeCol"}, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t,
		"(dateCol >= toDate('2014-11-12') AND dateCol <= toDate('2015-11-12')) AND (timeCol >= toDateTime(1415792726) AND timeCol <= toDateTime(1447328726))",
		got,
	)
}

func TestMacroDateTimeFilter_WrongArgCount(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	_, err := DateTimeFilter(context.Background(), q, []string{"only"}, 0, emptyProvider(t))
	assert.ErrorIs(t, err, sqlutil.ErrorBadArgumentCount)
}

func TestMacroTimeFilter_ExplicitColumn(t *testing.T) {
	q := &models.HdxQuery{
		RawSQL:    "SELECT $__timeFilter(ts) FROM mydb.events",
		TimeRange: stdRange(t),
	}
	got, err := TimeFilter(context.Background(), q, []string{"ts"}, parser.Pos(7), emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "ts >= toDateTime(1415792726) AND ts <= toDateTime(1447328726)", got)
}

func TestMacroTimeFilter_TooManyArgs(t *testing.T) {
	q := &models.HdxQuery{TimeRange: stdRange(t)}
	_, err := TimeFilter(context.Background(), q, []string{"a", "b"}, 0, emptyProvider(t))
	assert.ErrorIs(t, err, sqlutil.ErrorBadArgumentCount)

	// Same check for the other PK-lookup macros to keep the surface uniform.
	_, err = TimeFilterMs(context.Background(), q, []string{"a", "b"}, 0, emptyProvider(t))
	assert.ErrorIs(t, err, sqlutil.ErrorBadArgumentCount)
	_, err = TimeInterval(context.Background(), q, []string{"a", "b"}, 0, emptyProvider(t))
	assert.ErrorIs(t, err, sqlutil.ErrorBadArgumentCount)
	_, err = TimeIntervalMs(context.Background(), q, []string{"a", "b"}, 0, emptyProvider(t))
	assert.ErrorIs(t, err, sqlutil.ErrorBadArgumentCount)
}

func TestMacroTimeFilterMs_ExplicitColumn(t *testing.T) {
	q := &models.HdxQuery{
		RawSQL:    "SELECT $__timeFilter_ms(ts) FROM mydb.events",
		TimeRange: stdRange(t),
	}
	got, err := TimeFilterMs(context.Background(), q, []string{"ts"}, parser.Pos(7), emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t,
		"ts >= fromUnixTimestamp64Milli(1415792726371) AND ts <= fromUnixTimestamp64Milli(1447328726371)",
		got,
	)
}

func TestMacroTimeFilter_PKLookupFromCache(t *testing.T) {
	// SQL where macro position aligns with the FROM mydb.events reference;
	// `cte.GetMacroCTEs` resolves database/table -> ("mydb","events").
	// Pre-seed the PK cache so executeQuery is never called.
	sql := "SELECT $__timeFilter FROM mydb.events"
	p := NewMetadataProvider(nopMetadataDS{})
	p.pkCache.Set("mydb_events", "primary_ts", ttlcache.DefaultTTL)
	pos := macroPosFor(t, sql)

	q := &models.HdxQuery{
		RawSQL:    sql,
		TimeRange: stdRange(t),
	}
	got, err := TimeFilter(context.Background(), q, nil, pos, p)
	require.NoError(t, err)
	assert.Contains(t, got, "primary_ts >= toDateTime(1415792726)")
	assert.Contains(t, got, "primary_ts <= toDateTime(1447328726)")
}

func TestMacroPKLookup_QuotedViewLooksUpBareName(t *testing.T) {
	sql := "SELECT $__timeFilter FROM akamai.`logs#redacted`"
	pos := macroPosFor(t, sql)
	q := &models.HdxQuery{
		RawSQL:    sql,
		TimeRange: stdRange(t),
		Interval:  20 * time.Second,
	}

	for name, macro := range pkLookupMacros {
		t.Run(name, func(t *testing.T) {
			ds := &fakeMetadataDS{
				queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
					return respondWith(frameOf([]string{"_timestamp"}), "pk_query"), nil
				},
			}
			p := NewMetadataProvider(ds)

			got, err := macro(context.Background(), q, nil, pos, p)
			require.NoError(t, err)
			assert.Equal(t,
				"SELECT primary_key FROM system.tables WHERE database='akamai' AND table ='logs#redacted'",
				rawSQLOf(t, ds.lastRequest),
			)
			assert.Contains(t, got, "_timestamp")
		})
	}
}

func TestMacroTimeInterval(t *testing.T) {
	q := &models.HdxQuery{
		RawSQL:   "select $__timeInterval(col) from foo",
		Interval: 20 * time.Second,
	}
	got, err := TimeInterval(context.Background(), q, []string{"col"}, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "toStartOfInterval(toDateTime(col), INTERVAL 20 second)", got)
}

func TestMacroTimeIntervalMs(t *testing.T) {
	q := &models.HdxQuery{
		RawSQL:   "select $__timeInterval_ms(col) from foo",
		Interval: 20 * time.Second,
	}
	got, err := TimeIntervalMs(context.Background(), q, []string{"col"}, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "toStartOfInterval(toDateTime64(col, 3), INTERVAL 20000 millisecond)", got)
}

func TestMacroIntervalSeconds(t *testing.T) {
	q := &models.HdxQuery{
		RawSQL:   "select toStartOfInterval(col, INTERVAL $__interval_s second) AS time from foo",
		Interval: 20 * time.Second,
	}
	got, err := IntervalSeconds(context.Background(), q, nil, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "20", got)
}

func TestMacroIntervalSeconds_SubSecondFloorsToOne(t *testing.T) {
	q := &models.HdxQuery{Interval: 500 * time.Millisecond}
	got, err := IntervalSeconds(context.Background(), q, nil, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "1", got)
}

func TestMacroTimeInterval_SubSecondFloorsToOne(t *testing.T) {
	q := &models.HdxQuery{Interval: 500 * time.Millisecond}
	got, err := TimeInterval(context.Background(), q, []string{"col"}, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "toStartOfInterval(toDateTime(col), INTERVAL 1 second)", got)
}

func TestMacroTimeIntervalMs_SubMsFloorsToOne(t *testing.T) {
	// 500µs == 0ms after Milliseconds() — must floor to 1ms.
	q := &models.HdxQuery{Interval: 500 * time.Microsecond}
	got, err := TimeIntervalMs(context.Background(), q, []string{"col"}, 0, emptyProvider(t))
	require.NoError(t, err)
	assert.Equal(t, "toStartOfInterval(toDateTime64(col, 3), INTERVAL 1 millisecond)", got)
}

func TestMacrosRegistry_HasAllForkNames(t *testing.T) {
	// Dashboard contract: every name below must remain present in the
	// registry. Renames break production dashboards.
	want := map[string]string{
		"fromTime":        "FromTimeFilter",
		"toTime":          "ToTimeFilter",
		"fromTime_ms":     "FromTimeFilterMs",
		"toTime_ms":       "ToTimeFilterMs",
		"timeFilter":      "TimeFilter",
		"timeFilter_ms":   "TimeFilterMs",
		"dateFilter":      "DateFilter",
		"dateTimeFilter":  "DateTimeFilter",
		"dt":              "DateTimeFilter (alias)",
		"timeInterval":    "TimeInterval",
		"timeInterval_ms": "TimeIntervalMs",
		"interval_s":      "IntervalSeconds",
		// C5 / C7 entries; included for clarity.
		"conditionalAll": "Stub",
		"adHocFilter":    "AdHocFilterMacro",
	}
	for name, label := range want {
		assert.NotNil(t, Macros[name], "Macros[%q] (%s) missing", name, label)
	}
}

func TestStub_ReturnsTruthIdentity(t *testing.T) {
	got, err := Stub(context.Background(), &models.HdxQuery{}, nil, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, "1=1", got)
}

func TestMacroPKLookupErrorPropagates(t *testing.T) {
	// SQL with no resolvable table at pos. getPK returns
	// "no CTE found for macro at pos %d" → propagated.
	q := &models.HdxQuery{
		RawSQL:    "SELECT 1",
		TimeRange: stdRange(t),
	}
	_, err := TimeFilter(context.Background(), q, nil, parser.Pos(0), emptyProvider(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CTE found for macro at pos")
}

// macroPosFor parses sql and returns the position of its first macro CTE, so
// the PK-lookup macros resolve against the table in that statement's FROM.
func macroPosFor(t *testing.T, sql string) parser.Pos {
	t.Helper()
	exprs, err := parser.NewParser(sql).ParseStmts()
	require.NoError(t, err)
	macroCTEs, err := cte.GetMacroCTEs(exprs)
	require.NoError(t, err)
	require.NotEmpty(t, macroCTEs)
	for id := range macroCTEs {
		return id.Index
	}
	return parser.Pos(0)
}

// pkLookupMacros are the four macros that resolve their column through getPK
// when the argument is omitted.
var pkLookupMacros = map[string]func(context.Context, *models.HdxQuery, []string, parser.Pos, *MetadataProvider) (string, error){
	"timeFilter":      TimeFilter,
	"timeFilter_ms":   TimeFilterMs,
	"timeInterval":    TimeInterval,
	"timeInterval_ms": TimeIntervalMs,
}

func TestMacroPKLookup_KeylessTableIsTypedErrorNotSQL(t *testing.T) {
	// The stored empty string is how a table that exists but declares no
	// primary key is represented; getPK is what refuses to expand against it.
	sql := "SELECT $__timeFilter FROM mydb.keyless"
	pos := macroPosFor(t, sql)
	q := &models.HdxQuery{
		RawSQL:    sql,
		TimeRange: stdRange(t),
		Interval:  20 * time.Second,
	}

	for name, macro := range pkLookupMacros {
		t.Run(name, func(t *testing.T) {
			p := NewMetadataProvider(nopMetadataDS{})
			p.pkCache.Set("mydb_keyless", "", ttlcache.DefaultTTL)

			got, err := macro(context.Background(), q, nil, pos, p)
			assert.ErrorIs(t, err, ErrPrimaryKeyEmpty)
			assert.True(t, backend.IsDownstreamError(err))
			assert.Empty(t, got, "no SQL fragment may be emitted for an unresolved column")
			assert.Contains(t, err.Error(), "mydb.keyless")
			assert.Contains(t, err.Error(), "pass the time column as the macro argument")
		})
	}
}

func TestMacroPKLookup_GenericDownstreamFailureIsNotKeyless(t *testing.T) {
	sql := "SELECT $__timeFilter FROM mydb.events"
	pos := macroPosFor(t, sql)
	q := &models.HdxQuery{
		RawSQL:    sql,
		TimeRange: stdRange(t),
	}
	upstream := backend.DownstreamError(errors.New("cluster unavailable"))
	p := NewMetadataProvider(&fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return nil, upstream
		},
	})

	got, err := TimeFilter(context.Background(), q, nil, pos, p)
	assert.ErrorIs(t, err, upstream)
	assert.NotErrorIs(t, err, ErrPrimaryKeyEmpty)
	assert.Empty(t, got)
	assert.NotContains(t, err.Error(), "pass the time column")
}

// pkProvider answers every primary-key lookup with pk and records the request.
func pkProvider(pk string) (*MetadataProvider, *fakeMetadataDS) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frameOf([]string{pk}), "pk_query"), nil
		},
	}
	return NewMetadataProvider(ds), ds
}

func TestMacroPKLookup_FollowsWithAliasToTable(t *testing.T) {
	sql := "WITH errors AS (SELECT * FROM sample.logs WHERE status >= 500)\n" +
		"SELECT $__timeInterval() AS t, count() FROM errors\n" +
		"WHERE $__timeFilter()\nGROUP BY t"
	q := &models.HdxQuery{RawSQL: sql, TimeRange: stdRange(t), Interval: 20 * time.Second}

	for name, macro := range pkLookupMacros {
		t.Run(name, func(t *testing.T) {
			p, ds := pkProvider("timestamp")
			got, err := macro(context.Background(), q, nil, macroPosFor(t, sql), p)
			require.NoError(t, err)
			assert.Equal(t,
				"SELECT primary_key FROM system.tables WHERE database='sample' AND table ='logs'",
				rawSQLOf(t, ds.lastRequest),
			)
			assert.Contains(t, got, "timestamp")
		})
	}
}

func TestMacroPKLookup_ColumnNotSelectedByAliasIsClearError(t *testing.T) {
	sql := "WITH e AS (SELECT timestamp AS ts, v FROM sample.logs) SELECT $__timeFilter() FROM e"
	q := &models.HdxQuery{RawSQL: sql, TimeRange: stdRange(t)}
	p, _ := pkProvider("timestamp")

	got, err := TimeFilter(context.Background(), q, nil, macroPosFor(t, sql), p)
	assert.ErrorIs(t, err, ErrPKNotExposed)
	assert.True(t, backend.IsDownstreamError(err))
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "`timestamp`")
	assert.Contains(t, err.Error(), "`e`")
	assert.Contains(t, err.Error(), "pass the time column as the macro argument")
}

func TestMacroPKLookup_ColumnDroppedByInnerLayerIsClearError(t *testing.T) {
	// The outer alias selects *, but the alias it reads already dropped the
	// column, so the error must name the inner one.
	sql := "WITH b AS (SELECT v FROM sample.logs), a AS (SELECT * FROM b) SELECT $__timeFilter() FROM a"
	q := &models.HdxQuery{RawSQL: sql, TimeRange: stdRange(t)}
	p, _ := pkProvider("timestamp")

	_, err := TimeFilter(context.Background(), q, nil, macroPosFor(t, sql), p)
	assert.ErrorIs(t, err, ErrPKNotExposed)
	assert.Contains(t, err.Error(), "`b`")
}

func TestMacroPKLookup_ExpressionPrimaryKeySkipsColumnCheck(t *testing.T) {
	// An expression key cannot be matched by name; expanding it is today's
	// behavior, and ClickHouse reports whatever is wrong with it.
	sql := "WITH e AS (SELECT v FROM sample.logs) SELECT $__timeFilter() FROM e"
	q := &models.HdxQuery{RawSQL: sql, TimeRange: stdRange(t)}
	p, _ := pkProvider("toStartOfHour(timestamp)")

	got, err := TimeFilter(context.Background(), q, nil, macroPosFor(t, sql), p)
	require.NoError(t, err)
	assert.Contains(t, got, "toStartOfHour(timestamp) >= ")
}

func TestMacroPKLookup_NoSingleSourceNamesAliasWithoutLookup(t *testing.T) {
	sql := "WITH e AS (SELECT * FROM a JOIN b ON a.k = b.k) SELECT $__timeFilter() FROM e"
	q := &models.HdxQuery{RawSQL: sql, TimeRange: stdRange(t)}

	// nopMetadataDS panics on QueryData: the error must come before any lookup.
	got, err := TimeFilter(context.Background(), q, nil, macroPosFor(t, sql), NewMetadataProvider(nopMetadataDS{}))
	assert.ErrorIs(t, err, cte.ErrNoSingleSource)
	assert.True(t, backend.IsDownstreamError(err))
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "`e`")
	assert.Contains(t, err.Error(), "pass the time column as the macro argument")
	assert.NotContains(t, err.Error(), "primary key not found")
}

func TestMacroPKLookup_KeylessTableBehindAliasNamesTheTable(t *testing.T) {
	sql := "WITH e AS (SELECT * FROM mydb.keyless) SELECT $__timeFilter() FROM e"
	q := &models.HdxQuery{RawSQL: sql, TimeRange: stdRange(t)}
	p, _ := pkProvider("")

	_, err := TimeFilter(context.Background(), q, nil, macroPosFor(t, sql), p)
	assert.ErrorIs(t, err, ErrPrimaryKeyEmpty)
	assert.Contains(t, err.Error(), "mydb.keyless")
}

func TestMacroPKLookup_ExplicitColumnBehindAliasSendsNoLookup(t *testing.T) {
	sql := "WITH e AS (SELECT * FROM sample.logs) SELECT $__timeFilter(ts) FROM e"
	q := &models.HdxQuery{RawSQL: sql, TimeRange: stdRange(t)}

	got, err := TimeFilter(context.Background(), q, []string{"ts"}, macroPosFor(t, sql), NewMetadataProvider(nopMetadataDS{}))
	require.NoError(t, err)
	assert.Contains(t, got, "ts >= ")
}
