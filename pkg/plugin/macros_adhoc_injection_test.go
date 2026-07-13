package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFilterCondition_OperatorAllowlist(t *testing.T) {
	// Every allowlisted scalar comparison operator yields a non-empty
	// condition with no error (guards against an over-strict allowlist).
	for _, op := range []string{"=", "!=", "<", "<=", ">", ">="} {
		f := models.AdHocFilter{Key: "c", Operator: op, Value: "v"}
		got, err := buildFilterCondition(f, "String", "c")
		require.NoError(t, err, "operator %q", op)
		assert.NotEmpty(t, got)
		assert.Contains(t, got, op)
	}
}

func TestBuildFilterCondition_RejectsInjectedOperator(t *testing.T) {
	for _, op := range []string{
		"= 'x' OR 1=1 -- ",
		"= 'x') OR (1=1",
		"BETWEEN",
		"; DROP TABLE t",
		"",
		"  ",
	} {
		f := models.AdHocFilter{Key: "c", Operator: op, Value: "x"}
		got, err := buildFilterCondition(f, "String", "c")
		assert.Error(t, err, "operator %q must be rejected", op)
		assert.Empty(t, got)
	}
}

func TestBuildFilterCondition_ValueMetacharactersEscaped(t *testing.T) {
	f := models.AdHocFilter{Key: "c", Operator: "=", Value: `a'; DROP TABLE t; --\`}
	got, err := buildFilterCondition(f, "String", "c")
	require.NoError(t, err)
	// The value stays wholly inside the quoted literal; quote and backslash
	// are escaped so neither terminates it.
	assert.Contains(t, got, `\'`)
	assert.Contains(t, got, `\\`)
	assert.True(t, strings.HasPrefix(got, "c = '"), "got %q", got)
	assert.True(t, strings.HasSuffix(got, "'"), "got %q", got)
}

func TestAdHocFilterMacro_RejectsInjectedOperator(t *testing.T) {
	p := preseededProvider("foo", fooSchema)
	q := &models.HdxQuery{
		RawSQL:  "SELECT * FROM foo WHERE 1=1",
		Filters: []models.AdHocFilter{{Key: "column", Operator: "= 'x' OR 1=1 -- ", Value: "x"}},
	}
	_, err := AdHocFilterMacro(context.Background(), q, []string{"foo"}, parser.Pos(0), p)
	assert.Error(t, err)
}

func TestAdHocFilterMacro_MapKeySubscriptCannotBreakOut(t *testing.T) {
	p := preseededProvider("foo", fooSchema)
	q := &models.HdxQuery{
		RawSQL:  "SELECT * FROM foo WHERE 1=1",
		Filters: []models.AdHocFilter{{Key: "mapColumn['a'] OR 1=1 --']", Operator: "=", Value: "v"}},
	}
	got, err := AdHocFilterMacro(context.Background(), q, []string{"foo"}, parser.Pos(0), p)
	require.NoError(t, err)
	// The base column is backtick-quoted and the whole injected subscript is
	// escaped inside the map-access literal, so the quote cannot terminate it.
	assert.Equal(t, "`mapColumn`['a\\'] OR 1=1 --'] = 'v'", got)
	assert.Contains(t, got, `\'`)
}

func TestAdHocFilterMacro_MapKeyHonestIsQuoted(t *testing.T) {
	p := preseededProvider("foo", fooSchema)
	q := &models.HdxQuery{
		RawSQL:  "SELECT * FROM foo WHERE 1=1",
		Filters: []models.AdHocFilter{{Key: "mapColumn['env']", Operator: "=", Value: "prod"}},
	}
	got, err := AdHocFilterMacro(context.Background(), q, []string{"foo"}, parser.Pos(0), p)
	require.NoError(t, err)
	assert.Equal(t, "`mapColumn`['env'] = 'prod'", got)
}

func TestAdHocFilterMacro_MapKeySubscriptQuoteEscaped(t *testing.T) {
	p := preseededProvider("foo", fooSchema)
	q := &models.HdxQuery{
		RawSQL:  "SELECT * FROM foo WHERE 1=1",
		Filters: []models.AdHocFilter{{Key: "mapColumn['a''b']", Operator: "=", Value: "v"}},
	}
	got, err := AdHocFilterMacro(context.Background(), q, []string{"foo"}, parser.Pos(0), p)
	require.NoError(t, err)
	assert.Equal(t, "`mapColumn`['a\\'\\'b'] = 'v'", got)
}
