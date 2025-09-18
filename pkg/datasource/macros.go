package datasource

import (
	"context"
	"fmt"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

const (
	SyntheticNull  = "__null__"
	SyntheticEmpty = "__empty__"
)

type MacroFunc func(*HDXQuery, []string, parser.Pos, *MetaDataProvider, context.Context) (string, error)

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
func FromTimeFilter(query *HDXQuery, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime(query.TimeRange.From), nil
}

// ToTimeFilter returns a time filter expression based on grafana's timepicker's "to" time in seconds
func ToTimeFilter(query *HDXQuery, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime(query.TimeRange.To), nil
}

// FromTimeFilterMs returns a time filter expression based on grafana's timepicker's "from" time in milliseconds
func FromTimeFilterMs(query *HDXQuery, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime64(query.TimeRange.From), nil
}

// ToTimeFilterMs returns a time filter expression based on grafana's timepicker's "to" time in milliseconds
func ToTimeFilterMs(query *HDXQuery, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	return timeToDateTime64(query.TimeRange.To), nil
}

func TimeFilter(query *HDXQuery, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
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

func TimeFilterMs(query *HDXQuery, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
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

func DateFilter(query *HDXQuery, args []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
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

func DateTimeFilter(query *HDXQuery, args []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
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

func TimeInterval(query *HDXQuery, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
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

func TimeIntervalMs(query *HDXQuery, args []string, pos parser.Pos, mdProvider *MetaDataProvider, context context.Context) (string, error) {
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

func IntervalSeconds(query *HDXQuery, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
	seconds := math.Max(query.Interval.Seconds(), 1)
	return fmt.Sprintf("%d", int(seconds)), nil
}

// AdHocFilterMacro implements the $__adHocFilter() macro
func AdHocFilterMacro(query *HDXQuery, _ []string, pos parser.Pos, mdProvider *MetaDataProvider, _ context.Context) (string, error) {
	if query.Filters == nil || len(query.Filters) == 0 {
		return "1=1", nil
	}
	expr, err := parser.NewParser(query.RawSQL).ParseStmts()
	if err != nil {
		return "", err
	}

	macroCTEs, err := GetMacroCTEs(expr)
	if err != nil {
		return "", err
	}
	var cte = ""
	for _, macroCTE := range macroCTEs {
		if macroCTE.MacroPos == pos {
			cte = macroCTE.CTE
			break
		}
	}
	if cte == "" {
		return "", fmt.Errorf("cannot apply ad hoc filters: unable to resolve tableName for ad hoc filter at index %d", pos)
	}
	keys, err := mdProvider.GetKeys(cte)

	if err != nil {
		return "", fmt.Errorf("cannot apply ad hoc filters: unable to resolve keys for cte: %s", cte)
	}
	var conditions []string
	keyNames := slices.Collect(maps.Keys(keys))
	for _, filter := range query.Filters {
		if slices.Contains(keyNames, filter.Key) {
			keyType := keys[filter.Key]
			condition, err := buildFilterCondition(filter, strings.Contains(keyType, "string"))
			if err != nil {
				return "", fmt.Errorf("error building filter condition for key '%s': %w", filter.Key, err)
			}
			if condition != "" {
				conditions = append(conditions, condition)
			}
		}
	}

	if len(conditions) == 0 {
		return "1=1", nil
	}

	return strings.Join(conditions, " AND "), nil
}

// buildFilterCondition creates a SQL condition from an ad-hoc filter
func buildFilterCondition(filter AdHocFilter, isString bool) (string, error) {
	key := filter.Key
	value := filter.Value
	operator := filter.Operator
	if operator == "=|" {
		values, hasNull := getJoinedValues(filter.Values)

		var parts []string
		if hasNull {
			parts = append(parts, fmt.Sprintf("%s IS NULL", key))
		}

		if values != "" {
			parts = append(parts, fmt.Sprintf("%s IN (%s)", key, values))
		}

		return strings.Join(parts, " OR "), nil
	} else if operator == "!=|" {
		values, hasNull := getJoinedValues(filter.Values)

		var parts []string
		if hasNull {
			parts = append(parts, fmt.Sprintf("%s IS NOT NULL", key))
		}

		if values != "" {
			parts = append(parts, fmt.Sprintf("%s NOT IN (%s)", key, values))
		}

		return strings.Join(parts, " AND "), nil
	} else if strings.ToUpper(value) == "NULL" || value == SyntheticNull {
		if operator == "=" && isString {
			return fmt.Sprintf("%s IS NULL OR %s = %s", key, key, SyntheticNull), nil
		} else if operator == "!=" && isString {
			return fmt.Sprintf("%s IS NOT NULL OR %s != %s", key, key, SyntheticNull), nil
		} else if operator == "=" {
			return fmt.Sprintf("%s IS NULL", key), nil
		} else if operator == "!=" {
			return fmt.Sprintf("%s IS NOT NULL", key), nil
		} else {
			return "", fmt.Errorf("%s: operator '%s' can not be applied to NULL value", key, operator)
		}
	} else if value == "" || value == SyntheticEmpty {
		if operator == "=" {
			return fmt.Sprintf("(%s = '' OR %s = '%s')", key, key, SyntheticEmpty), nil
		} else if operator == "!=" {
			return fmt.Sprintf("(%s != '' AND %s != '%s')", key, key, SyntheticEmpty), nil
		} else {
			return "", fmt.Errorf("%s: operator '%s' can not be applied to __empty__ value", key, operator)
		}

	} else if operator == "=~" {
		return fmt.Sprintf("toString(%s) LIKE $$%s$$", key, escapeWildcard(value)), nil
	} else if operator == "!~" {
		return fmt.Sprintf("toString(%s) NOT LIKE $$%s$$", key, escapeWildcard(value)), nil
	} else {
		return fmt.Sprintf("%s %s $$%s$$", key, operator, value), nil
	}
}

func getJoinedValues(values []string) (string, bool) {
	var buffer []string
	hasNull := false
	for _, v := range values {
		if strings.ToUpper(v) == "NULL" || v == SyntheticNull {
			hasNull = true
		} else if v == SyntheticEmpty {
			buffer = append(buffer, "$$$$")
		} else {
			buffer = append(buffer, fmt.Sprintf("$$%s$$", v))
		}
	}
	return strings.Join(buffer, ", "), hasNull
}

// escapeWildcard prepares wildcard patterns for LIKE queries
func escapeWildcard(v string) string {
	chars := []rune(v)
	for i := range len(v) {
		if chars[i] == '*' && (i == 0 || chars[i-1] != '\\') {
			chars[i] = '%'
		}
	}
	v = string(chars)
	v = strings.ReplaceAll(v, `\*`, "*")
	return v
}

func Stub(_ *HDXQuery, _ []string, _ parser.Pos, _ *MetaDataProvider, _ context.Context) (string, error) {
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
	"adHocFilter":     AdHocFilterMacro,
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
