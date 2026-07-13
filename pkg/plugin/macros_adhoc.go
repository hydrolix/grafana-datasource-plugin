package plugin

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

// Synthetic value sentinels used by Grafana's ad-hoc filter UI to represent
// NULL and empty values when the column type otherwise can't.
const (
	SyntheticNull  = "__null__"
	SyntheticEmpty = "__empty__"
	RegexPrefix    = "regex:"
)

// mapTypeFilterKey matches `column['key']` syntax used by the ad-hoc filter
// UI for ClickHouse Map columns; the leading `column` is the underlying
// SQL identifier in the schema, the bracketed `key` is the map key.
var mapTypeFilterKey = regexp.MustCompile(`^(.*)\['.*']$`)

// explicitCTEArg constrains the explicit `$__adHocFilter(<arg>)` argument to a
// strict identifier, optionally `database.table`. This source never passes
// through the AST, so it needs its own gate before reaching the metadata path.
var explicitCTEArg = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

func init() {
	Macros["adHocFilter"] = AdHocFilterMacro
}

// escape returns s with characters that have special meaning inside a
// ClickHouse single-quoted literal replaced by their backslash-escape
// sequences. The result is safe to surround with single quotes to form a
// literal.
//
// This is the security-critical helper that replaces the fork's
// `$$...$$` dollar-quoted literals — `$$` inside a user-supplied value
// would terminate the literal and let subsequent bytes become SQL.
func escape(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			b.WriteString(`\'`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			b.WriteString(`\0`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// quoteIdentifier wraps s in backticks for safe use where ClickHouse expects
// an identifier (a table/database name or a map subscript column). Unlike
// escape (which handles single-quoted literals), identifiers are a distinct
// grammar. The ClickHouse SQL parser's lexer does not unescape characters
// inside backtick-quoted identifiers — it reads bytes until the first
// backtick — so an embedded backtick cannot be represented unambiguously and
// is rejected rather than emitted as something that could re-parse into
// injected SQL. NUL is rejected for the same reason. Every other byte is
// literal inside the backticks and needs no escaping.
func quoteIdentifier(s string) (string, error) {
	if strings.IndexByte(s, '`') >= 0 || strings.IndexByte(s, 0) >= 0 {
		return "", fmt.Errorf("invalid identifier %q", s)
	}
	return "`" + s + "`", nil
}

// AdHocFilterMacro implements the $__adHocFilter() macro. It accepts zero
// or one argument (an explicit CTE / table name); when omitted, the CTE
// is resolved by AST position. Returns the AND-joined per-filter
// conditions, or "1=1" when no filters apply.
func AdHocFilterMacro(ctx context.Context, query *models.HdxQuery, params []string, pos parser.Pos, mdProvider *MetadataProvider) (string, error) {
	if len(query.Filters) == 0 {
		return "1=1", nil
	}
	if len(params) > 1 {
		return "", backend.DownstreamError(fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(params)))
	}

	cteName := ""
	if len(params) == 1 {
		cteName = params[0]
		// The explicit argument bypasses the AST, so it must be a strict
		// identifier (optionally database.table) before it can drive a
		// metadata query. An empty argument falls through to AST resolution.
		if cteName != "" && !explicitCTEArg.MatchString(cteName) {
			return "", backend.DownstreamError(fmt.Errorf("invalid $__adHocFilter argument: %q", cteName))
		}
	}

	if cteName == "" {
		exprs, err := parser.NewParser(query.RawSQL).ParseStmts()
		if err != nil {
			return "", err
		}
		macroCTEs, err := cte.GetMacroCTEs(exprs)
		if err != nil {
			return "", err
		}
		for _, c := range macroCTEs {
			if c.MacroPos == pos {
				cteName = c.CTE
				break
			}
		}
	}
	if cteName == "" {
		return "", fmt.Errorf("cannot apply ad hoc filters: unable to resolve tableName for ad hoc filter at index %d", pos)
	}

	keys, err := mdProvider.GetKeys(ctx, query.Headers, cteName)
	if err != nil {
		return "", fmt.Errorf("cannot apply ad hoc filters: unable to resolve keys for cte: %s", cteName)
	}

	keyNames := slices.Collect(maps.Keys(keys))
	var conditions []string
	for _, filter := range query.Filters {
		column := filter.Key
		if mapTypeFilterKey.MatchString(filter.Key) {
			column = mapTypeFilterKey.FindStringSubmatch(filter.Key)[1]
		}
		if !slices.Contains(keyNames, column) {
			continue
		}
		condition, err := buildFilterCondition(filter, keys[column])
		if err != nil {
			return "", fmt.Errorf("error building filter condition for key '%s': %w", filter.Key, err)
		}
		if condition != "" {
			conditions = append(conditions, condition)
		}
	}

	if len(conditions) == 0 {
		return "1=1", nil
	}
	return strings.Join(conditions, " AND "), nil
}

// buildFilterCondition emits a single condition for one filter, dispatching
// on the column's ClickHouse type. Array columns route to
// buildArrayCondition; scalar/map columns are handled inline. Every
// user-supplied value reaches the wire as `'<escape(value)>'`.
func buildFilterCondition(filter models.AdHocFilter, keyType string) (string, error) {
	lower := strings.ToLower(keyType)
	isString := strings.Contains(lower, "string)") || lower == "string"
	isArray := strings.Contains(lower, "array")
	isMap := strings.Contains(lower, "map")
	if isArray {
		return buildArrayCondition(filter)
	}

	key := filter.Key
	value := filter.Value
	operator := filter.Operator

	switch {
	case operator == "=|":
		if isMap && !isString {
			return "", fmt.Errorf("cannot apply =| operator over  non string map values")
		}
		values, hasNull := getJoinedValues(filter.Values)
		var parts []string
		if hasNull {
			parts = append(parts, fmt.Sprintf("%s IS NULL", key))
		}
		if values != "" {
			parts = append(parts, fmt.Sprintf("%s IN (%s)", key, values))
		}
		switch len(parts) {
		case 0:
			return "", nil
		case 1:
			return parts[0], nil
		default:
			return fmt.Sprintf("(%s)", strings.Join(parts, " OR ")), nil
		}
	case operator == "!=|":
		if isMap && !isString {
			return "", fmt.Errorf("cannot apply !=| operator over  non string map values")
		}
		values, hasNull := getJoinedValues(filter.Values)
		var parts []string
		if hasNull {
			parts = append(parts, fmt.Sprintf("%s IS NOT NULL", key))
		}
		if values != "" {
			parts = append(parts, fmt.Sprintf("%s NOT IN (%s)", key, values))
		}
		return strings.Join(parts, " AND "), nil
	case strings.ToUpper(value) == "NULL" || value == SyntheticNull:
		switch {
		case operator == "=" && isString:
			return fmt.Sprintf("(%s IS NULL OR %s = '%s')", key, key, SyntheticNull), nil
		case operator == "!=" && isString:
			return fmt.Sprintf("(%s IS NOT NULL OR %s != '%s')", key, key, SyntheticNull), nil
		case operator == "=":
			return fmt.Sprintf("%s IS NULL", key), nil
		case operator == "!=":
			return fmt.Sprintf("%s IS NOT NULL", key), nil
		default:
			return "", fmt.Errorf("%s: operator '%s' can not be applied to NULL value", key, operator)
		}
	case value == "" || value == SyntheticEmpty:
		switch operator {
		case "=":
			return fmt.Sprintf("(%s = '' OR %s = '%s')", key, key, SyntheticEmpty), nil
		case "!=":
			return fmt.Sprintf("(%s != '' AND %s != '%s')", key, key, SyntheticEmpty), nil
		default:
			return "", fmt.Errorf("%s: operator '%s' can not be applied to __empty__ value", key, operator)
		}
	case operator == "=~":
		regex, isRegex := getRegexValue(value)
		if isRegex {
			return fmt.Sprintf("match(toString(%s), '%s')", key, escape(regex)), nil
		}
		return fmt.Sprintf("toString(%s) LIKE '%s'", key, escape(escapeWildcard(value))), nil
	case operator == "!~":
		regex, isRegex := getRegexValue(value)
		if isRegex {
			return fmt.Sprintf("not match(toString(%s), '%s')", key, escape(regex)), nil
		}
		return fmt.Sprintf("toString(%s) NOT LIKE '%s'", key, escape(escapeWildcard(value))), nil
	default:
		return fmt.Sprintf("%s %s '%s'", key, operator, escape(value)), nil
	}
}

// buildArrayCondition emits has(col, '<val>') / not has(...) clauses for
// Array columns, joined with OR for multi-value filters.
func buildArrayCondition(filter models.AdHocFilter) (string, error) {
	key := filter.Key
	value := filter.Value
	operator := filter.Operator
	switch operator {
	case "=|":
		var buffer []string
		for _, v := range filter.Values {
			buffer = append(buffer, fmt.Sprintf("has(%s, '%s')", key, escape(v)))
		}
		return fmt.Sprintf("(%s)", strings.Join(buffer, " OR ")), nil
	case "!=|":
		var buffer []string
		for _, v := range filter.Values {
			buffer = append(buffer, fmt.Sprintf("not has(%s, '%s')", key, escape(v)))
		}
		return fmt.Sprintf("(%s)", strings.Join(buffer, " OR ")), nil
	case "!=":
		return fmt.Sprintf("not has(%s, '%s')", key, escape(value)), nil
	case "=":
		return fmt.Sprintf("has(%s, '%s')", key, escape(value)), nil
	default:
		return "", fmt.Errorf("operator %s unsupported for Array value", operator)
	}
}

// getRegexValue extracts the regex pattern from a `regex:<pattern>` value;
// returns (pattern, true) when the prefix is present, ("", false) otherwise.
func getRegexValue(value string) (string, bool) {
	if strings.HasPrefix(value, RegexPrefix) {
		return value[len(RegexPrefix):], true
	}
	return "", false
}

// getJoinedValues turns a Values slice into the comma-joined body of an
// IN (...) list and a hasNull flag for any NULL sentinels in the slice.
// Empty strings serialise as '' (the fork emitted `$$$$`; the new shape
// produces the same parser-visible empty literal).
func getJoinedValues(values []string) (string, bool) {
	var buffer []string
	hasNull := false
	for _, v := range values {
		if strings.ToUpper(v) == "NULL" || v == SyntheticNull {
			hasNull = true
		} else if v == SyntheticEmpty {
			buffer = append(buffer, "''")
		} else {
			buffer = append(buffer, fmt.Sprintf("'%s'", escape(v)))
		}
	}
	return strings.Join(buffer, ", "), hasNull
}

// escapeWildcard prepares wildcard patterns for LIKE queries: `*` (unescaped)
// becomes `%`; an escaped `\*` becomes literal `*`. Run before escape() so
// the resulting `%` survives quoting.
func escapeWildcard(v string) string {
	chars := []rune(v)
	for i := 0; i < len(chars); i++ {
		if chars[i] == '*' && (i == 0 || chars[i-1] != '\\') {
			chars[i] = '%'
		}
	}
	v = string(chars)
	v = strings.ReplaceAll(v, `\*`, "*")
	return v
}
