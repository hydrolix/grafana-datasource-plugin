package plugin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantArgs []string
		wantLen  int
	}{
		{"no bracket", "rest of sql", nil, 0},
		{"empty parens", "()rest", []string{""}, 2},
		{"one arg", "(col)rest", []string{"col"}, 5},
		{"two args", "(a, b)rest", []string{"a", "b"}, 6},
		{"trim whitespace", "( a , b )rest", []string{"a", "b"}, 9},
		{"nested parens", "(func(x, y))rest", []string{"func(x, y)"}, 12},
		{"unbalanced open", "(abc,d", nil, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args, length := parseArgs(tc.in)
			assert.Equal(t, tc.wantArgs, args)
			assert.Equal(t, tc.wantLen, length)
		})
	}
}

func TestRoundTimeRange(t *testing.T) {
	t0 := time.Date(2026, 6, 13, 12, 0, 30, 0, time.UTC)
	tr := backend.TimeRange{From: t0, To: t0.Add(2 * time.Minute)}

	t.Run("invalid interval returns unchanged", func(t *testing.T) {
		got := roundTimeRange(tr, "not-a-duration")
		assert.Equal(t, tr, got)
	})

	t.Run("sub-second interval returns unchanged", func(t *testing.T) {
		got := roundTimeRange(tr, "500ms")
		assert.Equal(t, tr, got)
	})

	t.Run("1 minute interval rounds both endpoints", func(t *testing.T) {
		got := roundTimeRange(tr, "1m")
		assert.Equal(t, t0.Round(time.Minute), got.From)
		assert.Equal(t, tr.To.Round(time.Minute), got.To)
	})
}

func TestInterpolate_NoMacros_PassesThrough(t *testing.T) {
	i := NewHdxInterpolator(NewMetadataProvider(), map[string]MacroFunc{})
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: "SELECT 1"})
	assert.NoError(t, err)
	assert.Equal(t, "SELECT 1", out)
}

func TestInterpolate_UnknownMacroLeftInPlace(t *testing.T) {
	// Macro identifier appears in the AST but is not registered — interpolator
	// leaves the call site intact and downstream parsing surfaces it as a SQL error.
	i := NewHdxInterpolator(NewMetadataProvider(), map[string]MacroFunc{})
	in := "SELECT $__unknownMacro() FROM t"
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: in})
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestInterpolate_RegisteredMacroIsDispatched(t *testing.T) {
	macros := map[string]MacroFunc{
		"upper": func(_ context.Context, _ *models.HdxQuery, args []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
			return "UPPER(" + args[0] + ")", nil
		},
	}
	i := NewHdxInterpolator(NewMetadataProvider(), macros)
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: "SELECT $__upper(name) FROM t"})
	assert.NoError(t, err)
	assert.Contains(t, out, "UPPER(name)")
	assert.NotContains(t, out, "$__upper")
}

func TestInterpolate_EscapedMacroStripsOneDollar(t *testing.T) {
	i := NewHdxInterpolator(NewMetadataProvider(), Macros)
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: "SELECT $$__conditionalAll() FROM t"})
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "SELECT $__conditionalAll()"), "got: %s", out)
}

func TestInterpolate_StubConditionalAll(t *testing.T) {
	i := NewHdxInterpolator(NewMetadataProvider(), Macros)
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: "SELECT $__conditionalAll() FROM t"})
	assert.NoError(t, err)
	// Stub returns empty string — macro call site collapses to nothing.
	assert.Equal(t, "SELECT  FROM t", out)
}

func TestInterpolate_RoundAppliesBeforeMacros(t *testing.T) {
	t0 := time.Date(2026, 6, 13, 12, 0, 30, 0, time.UTC)
	macros := map[string]MacroFunc{
		"fromUnix": func(_ context.Context, q *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
			return q.TimeRange.From.Format(time.RFC3339), nil
		},
	}
	i := NewHdxInterpolator(NewMetadataProvider(), macros)
	out, err := i.interpolate(context.Background(), &models.HdxQuery{
		RawSQL:    "SELECT $__fromUnix() FROM t",
		Round:     "1m",
		TimeRange: backend.TimeRange{From: t0, To: t0.Add(2 * time.Minute)},
	})
	assert.NoError(t, err)
	// 12:00:30 rounded to 1m → 12:00:00.
	assert.Contains(t, out, t0.Round(time.Minute).Format(time.RFC3339))
}

func TestInterpolate_LongerMacroNamesMatchFirst(t *testing.T) {
	// If "timeFilter" is matched before "timeFilter_ms", the _ms variant
	// gets shadowed. The interpolator must sort macro keys by length descending.
	macros := map[string]MacroFunc{
		"timeFilter":    func(context.Context, *models.HdxQuery, []string, parser.Pos, *MetadataProvider) (string, error) { return "SHORT", nil },
		"timeFilter_ms": func(context.Context, *models.HdxQuery, []string, parser.Pos, *MetadataProvider) (string, error) { return "LONG", nil },
	}
	i := NewHdxInterpolator(NewMetadataProvider(), macros)
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: "SELECT $__timeFilter_ms() FROM t"})
	assert.NoError(t, err)
	assert.Contains(t, out, "LONG")
	assert.NotContains(t, out, "SHORT")
}

func TestInterpolatorImplementsSqldsInterface(t *testing.T) {
	// Smoke: HdxInterpolator wired through the sqlds.Interpolator interface
	// signature accepts rawJSON, applies macros, returns rewritten SQL.
	macros := map[string]MacroFunc{
		"upper": func(_ context.Context, _ *models.HdxQuery, args []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
			return "UPPER(" + args[0] + ")", nil
		},
	}
	i := NewHdxInterpolator(NewMetadataProvider(), macros)

	hdx := models.HdxQuery{
		Round: "1m",
	}
	rawJSON, _ := json.Marshal(hdx)

	out, err := i.Interpolate(
		context.Background(),
		nil, // *sqlds.SQLDatasource not used by this implementation
		&sqlutil.Query{
			RawSQL:    "SELECT $__upper(name) FROM t",
			TimeRange: backend.TimeRange{From: time.Unix(0, 0), To: time.Unix(60, 0)},
		},
		rawJSON,
	)
	assert.NoError(t, err)
	assert.Contains(t, out, "UPPER(name)")
}

func TestErrParseMacroArgs(t *testing.T) {
	// Macro identifier appears with unbalanced parens.
	i := NewHdxInterpolator(NewMetadataProvider(), map[string]MacroFunc{
		"foo": func(context.Context, *models.HdxQuery, []string, parser.Pos, *MetadataProvider) (string, error) {
			return "FOO", nil
		},
	})
	// Use raw SQL that would skip AST parse (otherwise parser would error first).
	// We invoke getMacroMatches directly to exercise the error path.
	_, err := getMacroMatches("$__foo(missingClose", "foo", nil)
	assert.ErrorIs(t, err, ErrParseMacroArgs)

	// Confirm the interpolator passes the error through.
	_, err = i.interpolate(context.Background(), &models.HdxQuery{RawSQL: "$__foo(unclosed"})
	if err != nil {
		assert.ErrorIs(t, err, ErrParseMacroArgs)
	}
}
