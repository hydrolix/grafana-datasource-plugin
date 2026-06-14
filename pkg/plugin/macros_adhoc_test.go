package plugin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fooSchema is the standard column-type map used by the macro test matrix.
// Mirrors the fork's TestAdHocFilterMacro fixture so ported cases match.
var fooSchema = map[string]string{
	"column":      "Nullable(String)",
	"column2":     "UInt64",
	"arrayColumn": "Array(String)",
	"mapColumn":   "Map(String, String)",
}

// preseededProvider returns a *MetadataProvider with the keyCache pre-seeded
// for cte → schema, so calls to GetKeys bypass the schema-query path.
func preseededProvider(cte string, schema map[string]string) *MetadataProvider {
	p := NewMetadataProvider(nopMetadataDS{})
	p.keyCache.Set(cte, schema, ttlcache.DefaultTTL)
	return p
}

func TestEscape(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"plain ascii", "hello", "hello"},
		{"single quote", "O'Reilly", `O\'Reilly`},
		{"backslash", `a\b`, `a\\b`},
		{"backslash plus quote", `a\'b`, `a\\\'b`},
		{"dollar dollar payload", "payload$$end", "payload$$end"}, // $$ is benign INSIDE a single-quoted literal
		{"newline", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"NUL byte", "a\x00b", `a\0b`},
		{"utf-8 passes through", "héllo", "héllo"},
		{"multi-byte cyrillic", "Привет", "Привет"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, escape(tc.in))
		})
	}
}

func TestEscape_QuotedRoundTrip(t *testing.T) {
	// Property test: for a corpus covering every byte from 1..127 plus
	// a handful of Unicode codepoints, surrounding `escape(v)` with single
	// quotes lexes back as the original value under ClickHouse's literal
	// grammar (modelled below: read characters between unescaped `'`,
	// interpret `\<c>` for known escapes).
	var corpus []string
	for b := 1; b < 128; b++ { // skip NUL since it's a special early case below
		corpus = append(corpus, string(rune(b)))
	}
	corpus = append(corpus, "\x00")
	corpus = append(corpus, "héllo", "Привет", "🦀", "O'Reilly", `a\b`, "$$", "$$$$end", "a\nb\tc\rd")

	for _, v := range corpus {
		quoted := "'" + escape(v) + "'"
		got, ok := parseClickhouseLiteral(quoted)
		require.True(t, ok, "must parse as a single literal: %q (input %q)", quoted, v)
		assert.Equal(t, v, got, "input %q quoted as %q parsed back to %q", v, quoted, got)
	}
}

// parseClickhouseLiteral models the relevant subset of ClickHouse's
// single-quoted literal grammar to confirm round-trip parity.
// Returns the decoded value and true if `quoted` is exactly one literal.
func parseClickhouseLiteral(quoted string) (string, bool) {
	if len(quoted) < 2 || quoted[0] != '\'' || quoted[len(quoted)-1] != '\'' {
		return "", false
	}
	inner := quoted[1 : len(quoted)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\\' {
			if inner[i] == '\'' {
				return "", false // unescaped quote inside — would terminate early
			}
			b.WriteByte(inner[i])
			continue
		}
		if i+1 >= len(inner) {
			return "", false
		}
		switch inner[i+1] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '0':
			b.WriteByte(0)
		case '\\':
			b.WriteByte('\\')
		case '\'':
			b.WriteByte('\'')
		default:
			return "", false
		}
		i++
	}
	return b.String(), true
}

func TestEscapeWildcard(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"foo", "foo"},
		{"*foo*", "%foo%"},
		{"*foo", "%foo"},
		{"foo*", "foo%"},
		{`\*foo`, "*foo"},
		{`foo\*bar`, "foo*bar"},
		{`*foo\*bar*`, `%foo*bar%`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, escapeWildcard(tc.in))
		})
	}
}

func TestGetRegexValue(t *testing.T) {
	cases := []struct {
		in        string
		want      string
		wantIsRgx bool
	}{
		{"plain", "", false},
		{"regex:abc", "abc", true},
		{"regex:", "", true},
		{"prefix-regex:abc", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := getRegexValue(tc.in)
			assert.Equal(t, tc.wantIsRgx, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAdHocFilterMacro_NoFiltersReturnsTrue(t *testing.T) {
	p := preseededProvider("foo", fooSchema)
	got, err := AdHocFilterMacro(context.Background(), &models.HdxQuery{}, []string{"foo"}, parser.Pos(0), p)
	require.NoError(t, err)
	assert.Equal(t, "1=1", got)
}

func TestAdHocFilterMacro_TooManyParamsErrors(t *testing.T) {
	p := preseededProvider("foo", fooSchema)
	query := &models.HdxQuery{
		Filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: "x"}},
	}
	_, err := AdHocFilterMacro(context.Background(), query, []string{"a", "b"}, parser.Pos(0), p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument")
}

func TestAdHocFilterMacro_UnresolvedCTEErrors(t *testing.T) {
	p := preseededProvider("foo", fooSchema)
	// SQL has no FROM clause that aligns with pos; CTE resolution must fail.
	query := &models.HdxQuery{
		RawSQL:  "SELECT 1",
		Filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: "x"}},
	}
	_, err := AdHocFilterMacro(context.Background(), query, nil, parser.Pos(0), p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to resolve tableName")
}

func TestAdHocFilterMacro_ExplicitCTEArgumentResolves(t *testing.T) {
	p := preseededProvider("events", map[string]string{"id": "UInt64"})
	query := &models.HdxQuery{
		RawSQL:  "SELECT 1 WHERE $__adHocFilter(events)",
		Filters: []models.AdHocFilter{{Key: "id", Operator: "=", Value: "42"}},
	}
	got, err := AdHocFilterMacro(context.Background(), query, []string{"events"}, parser.Pos(0), p)
	require.NoError(t, err)
	assert.Contains(t, got, "id = '42'")
}

func TestAdHocFilterMacro_Matrix(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		filters []models.AdHocFilter
	}{
		{
			name:    "single equals filter",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: "test"}},
			want:    "column = 'test'",
		},
		{
			name: "multiple filters",
			filters: []models.AdHocFilter{
				{Key: "column", Operator: "=", Value: "test"},
				{Key: "column2", Operator: "!=", Value: "value2"},
			},
			want: "column = 'test' AND column2 != 'value2'",
		},
		{
			name:    "null value filter on string column",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: "null"}},
			want:    "(column IS NULL OR column = '__null__')",
		},
		{
			name:    "null value on non-string column",
			filters: []models.AdHocFilter{{Key: "column2", Operator: "=", Value: "null"}},
			want:    "column2 IS NULL",
		},
		{
			name:    "empty value filter",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: ""}},
			want:    "(column = '' OR column = '__empty__')",
		},
		{
			name:    "regex wildcard LIKE filter",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=~", Value: "*pattern*"}},
			want:    "toString(column) LIKE '%pattern%'",
		},
		{
			name:    "regex prefix match filter",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=~", Value: "regex:^foo"}},
			want:    "match(toString(column), '^foo')",
		},
		{
			name:    "regex NOT LIKE filter",
			filters: []models.AdHocFilter{{Key: "column", Operator: "!~", Value: "pattern"}},
			want:    "toString(column) NOT LIKE 'pattern'",
		},
		{
			name:    "multi-value IN filter",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=|", Values: []string{"a", "b", "c"}}},
			want:    "column IN ('a', 'b', 'c')",
		},
		{
			name:    "multi-value NOT IN filter",
			filters: []models.AdHocFilter{{Key: "column", Operator: "!=|", Values: []string{"a", "b", "c"}}},
			want:    "column NOT IN ('a', 'b', 'c')",
		},
		{
			name:    "multi-value IN with null sentinel",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=|", Values: []string{"a", "null", "c"}}},
			want:    "(column IS NULL OR column IN ('a', 'c'))",
		},
		{
			name:    "multi-value NOT IN with null sentinel",
			filters: []models.AdHocFilter{{Key: "column", Operator: "!=|", Values: []string{"a", "null", "c"}}},
			want:    "column IS NOT NULL AND column NOT IN ('a', 'c')",
		},
		{
			name:    "filter on non-existent column dropped",
			filters: []models.AdHocFilter{{Key: "nonexistent", Operator: "=", Value: "test"}},
			want:    "1=1",
		},
		{
			name:    "value with single quotes is escaped",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: "val'ue"}},
			want:    `column = 'val\'ue'`,
		},
		{
			name:    "value with $$ sequence is contained",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: "payload$$end"}},
			want:    "column = 'payload$$end'",
		},
		{
			name:    "multi-value IN with synthetic empty",
			filters: []models.AdHocFilter{{Key: "column", Operator: "=|", Values: []string{SyntheticEmpty, "b"}}},
			want:    "column IN ('', 'b')",
		},
		{
			name:    "array column equals",
			filters: []models.AdHocFilter{{Key: "arrayColumn", Operator: "=", Value: "value"}},
			want:    "has(arrayColumn, 'value')",
		},
		{
			name:    "array column not equals",
			filters: []models.AdHocFilter{{Key: "arrayColumn", Operator: "!=", Value: "test"}},
			want:    "not has(arrayColumn, 'test')",
		},
		{
			name:    "array column multi-value IN",
			filters: []models.AdHocFilter{{Key: "arrayColumn", Operator: "=|", Values: []string{"a", "b", "c"}}},
			want:    "(has(arrayColumn, 'a') OR has(arrayColumn, 'b') OR has(arrayColumn, 'c'))",
		},
		{
			name:    "array column multi-value NOT IN",
			filters: []models.AdHocFilter{{Key: "arrayColumn", Operator: "!=|", Values: []string{"x", "y"}}},
			want:    "(not has(arrayColumn, 'x') OR not has(arrayColumn, 'y'))",
		},
		{
			name: "mixed string and array columns",
			filters: []models.AdHocFilter{
				{Key: "column", Operator: "=", Value: "test"},
				{Key: "arrayColumn", Operator: "=", Value: "prod"},
			},
			want: "column = 'test' AND has(arrayColumn, 'prod')",
		},
		{
			name:    "map column with key syntax",
			filters: []models.AdHocFilter{{Key: "mapColumn['key1']", Operator: "=", Value: "value1"}},
			want:    "mapColumn['key1'] = 'value1'",
		},
		{
			name:    "map column with multi-value IN",
			filters: []models.AdHocFilter{{Key: "mapColumn['status']", Operator: "=|", Values: []string{"active", "pending"}}},
			want:    "mapColumn['status'] IN ('active', 'pending')",
		},
		{
			name: "mixed string and map columns",
			filters: []models.AdHocFilter{
				{Key: "column", Operator: "=", Value: "test"},
				{Key: "mapColumn['env']", Operator: "=", Value: "prod"},
			},
			want: "column = 'test' AND mapColumn['env'] = 'prod'",
		},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("[%d] %s", i+1, tc.name), func(t *testing.T) {
			p := preseededProvider("foo", fooSchema)
			query := &models.HdxQuery{
				RawSQL:  "SELECT * FROM foo WHERE 1=1",
				Filters: tc.filters,
			}
			got, err := AdHocFilterMacro(context.Background(), query, []string{"foo"}, parser.Pos(0), p)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBuildFilterCondition(t *testing.T) {
	tests := []struct {
		name, keyType, expected string
		filter                  models.AdHocFilter
		wantErr                 bool
	}{
		{name: "equals string",
			filter: models.AdHocFilter{Key: "c", Operator: "=", Value: "v"}, keyType: "String",
			expected: "c = 'v'"},
		{name: "equals empty on string",
			filter: models.AdHocFilter{Key: "c", Operator: "=", Value: ""}, keyType: "String",
			expected: "(c = '' OR c = '__empty__')"},
		{name: "equals null on string",
			filter: models.AdHocFilter{Key: "c", Operator: "=", Value: "null"}, keyType: "String",
			expected: "(c IS NULL OR c = '__null__')"},
		{name: "not equals string",
			filter: models.AdHocFilter{Key: "c", Operator: "!=", Value: "v"}, keyType: "String",
			expected: "c != 'v'"},
		{name: "not equals empty on string",
			filter: models.AdHocFilter{Key: "c", Operator: "!=", Value: ""}, keyType: "String",
			expected: "(c != '' AND c != '__empty__')"},
		{name: "not equals null on string",
			filter: models.AdHocFilter{Key: "c", Operator: "!=", Value: "null"}, keyType: "String",
			expected: "(c IS NOT NULL OR c != '__null__')"},
		{name: "regex match prefix",
			filter: models.AdHocFilter{Key: "c", Operator: "=~", Value: "regex:^foo"}, keyType: "String",
			expected: "match(toString(c), '^foo')"},
		{name: "regex LIKE wildcard",
			filter: models.AdHocFilter{Key: "c", Operator: "=~", Value: "*pattern*"}, keyType: "String",
			expected: "toString(c) LIKE '%pattern%'"},
		{name: "regex NOT LIKE",
			filter: models.AdHocFilter{Key: "c", Operator: "!~", Value: "p"}, keyType: "String",
			expected: "toString(c) NOT LIKE 'p'"},
		{name: "generic operator",
			filter: models.AdHocFilter{Key: "c", Operator: ">", Value: "5"}, keyType: "UInt64",
			expected: "c > '5'"},
		{name: "value escaped (single quote)",
			filter: models.AdHocFilter{Key: "c", Operator: "=", Value: "a'b"}, keyType: "String",
			expected: `c = 'a\'b'`},
		{name: "NULL operator unsupported",
			filter: models.AdHocFilter{Key: "c", Operator: ">", Value: "null"}, keyType: "String",
			wantErr: true},
		{name: "empty operator unsupported",
			filter: models.AdHocFilter{Key: "c", Operator: ">", Value: ""}, keyType: "String",
			wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildFilterCondition(tc.filter, tc.keyType)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestBuildArrayCondition(t *testing.T) {
	tests := []struct {
		name, expected string
		filter         models.AdHocFilter
		wantErr        bool
	}{
		{name: "equals", filter: models.AdHocFilter{Key: "a", Operator: "=", Value: "v"}, expected: "has(a, 'v')"},
		{name: "not equals", filter: models.AdHocFilter{Key: "a", Operator: "!=", Value: "v"}, expected: "not has(a, 'v')"},
		{name: "multi IN", filter: models.AdHocFilter{Key: "a", Operator: "=|", Values: []string{"x", "y"}}, expected: "(has(a, 'x') OR has(a, 'y'))"},
		{name: "multi NOT IN", filter: models.AdHocFilter{Key: "a", Operator: "!=|", Values: []string{"x"}}, expected: "(not has(a, 'x'))"},
		{name: "escaping applied", filter: models.AdHocFilter{Key: "a", Operator: "=", Value: "a'b"}, expected: `has(a, 'a\'b')`},
		{name: "unsupported operator", filter: models.AdHocFilter{Key: "a", Operator: ">", Value: "v"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildArrayCondition(tc.filter)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestBuildFilterConditionWithMaps(t *testing.T) {
	// =| / !=| over non-string Map should error
	_, err := buildFilterCondition(models.AdHocFilter{Key: "m['k']", Operator: "=|", Values: []string{"v"}}, "Map(String, UInt64)")
	assert.Error(t, err)
	_, err = buildFilterCondition(models.AdHocFilter{Key: "m['k']", Operator: "!=|", Values: []string{"v"}}, "Map(String, UInt64)")
	assert.Error(t, err)

	// =| over Map(String, String) succeeds
	got, err := buildFilterCondition(models.AdHocFilter{Key: "m['k']", Operator: "=|", Values: []string{"a", "b"}}, "Map(String, String)")
	require.NoError(t, err)
	assert.Equal(t, "m['k'] IN ('a', 'b')", got)
}

func TestMacrosRegistry_HasAdHocFilter(t *testing.T) {
	// init() in macros_adhoc.go must register the macro.
	assert.NotNil(t, Macros["adHocFilter"], "adHocFilter must be registered in Macros")
}
