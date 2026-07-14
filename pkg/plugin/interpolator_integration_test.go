package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests drive the real interpolation pipeline end to end: the real
// HdxInterpolator over the real package-level Macros registry, with a
// MetadataProvider whose caches are pre-seeded so nopMetadataDS is never
// queried. Where the per-macro unit tests call a single macro directly, these
// exercise dispatch (AST site-matching, reverse-order replacement, PK/schema
// resolution glue) as one flow — the integration seam that had no coverage.
//
// nopMetadataDS panics on any QueryData call, so a green run doubles as an
// assertion that every metadata lookup hit a pre-seeded cache key.

// integrationRange is a fixed time range so time-macro output is deterministic:
// timeToDateTime emits toDateTime(<unix>), i.e. toDateTime(1000)/toDateTime(2000).
var (
	integrationFrom = time.Unix(1000, 0).UTC()
	integrationTo   = time.Unix(2000, 0).UTC()
)

// newIntegrationInterpolator builds a real interpolator over the real Macros
// registry. seed pre-populates the provider's caches (nil for none).
func newIntegrationInterpolator(seed func(p *MetadataProvider)) *HdxInterpolator {
	p := NewMetadataProvider(nopMetadataDS{})
	if seed != nil {
		seed(p)
	}
	return NewHdxInterpolator(p, Macros)
}

func TestInterpolate_Integration_AdHocFilterAndTimeMacro(t *testing.T) {
	// One query carrying both $__adHocFilter() (schema resolution) and
	// $__timeFilter(ts) (explicit column, time bounds). Proves the two macros
	// coexist through a single dispatch pass with correct reverse-order rewrite.
	i := newIntegrationInterpolator(func(p *MetadataProvider) {
		p.keyCache.Set("foo", fooSchema, ttlcache.DefaultTTL)
	})
	q := &models.HdxQuery{
		RawSQL:    "SELECT * FROM foo WHERE $__adHocFilter() AND $__timeFilter(ts)",
		Filters:   []models.AdHocFilter{{Key: "column", Operator: "=", Value: "test"}},
		TimeRange: backend.TimeRange{From: integrationFrom, To: integrationTo},
	}

	out, err := i.interpolate(context.Background(), q)
	require.NoError(t, err)
	assert.Contains(t, out, "column = 'test'")
	assert.Contains(t, out, "ts >= toDateTime(1000)")
	assert.Contains(t, out, "ts <= toDateTime(2000)")
	assert.NotContains(t, out, "$__", "no macro call site should survive")
}

func TestInterpolate_Integration_AdHocFilterOverWithCTE(t *testing.T) {
	// $__adHocFilter() with no explicit argument over a WITH-CTE FROM. The
	// macro re-parses, resolves the alias `x` to its defining subquery via
	// cte.GetMacroCTEs, and looks up the schema under that resolved key. This
	// ties the resolve-adhoc-with-cte work to real dispatch.
	sql := "WITH x AS (SELECT status FROM events) SELECT $__adHocFilter() FROM x"

	// Derive the resolved cache key the macro will use, so the seed matches the
	// lookup exactly (rather than hardcoding the subquery string).
	exprs, err := parser.NewParser(sql).ParseStmts()
	require.NoError(t, err)
	m, err := cte.GetMacroCTEs(exprs)
	require.NoError(t, err)
	require.Len(t, m, 1)
	var resolvedKey string
	for _, c := range m {
		resolvedKey = c.CTE
	}
	require.Equal(t, "(SELECT status FROM events)", resolvedKey)

	i := newIntegrationInterpolator(func(p *MetadataProvider) {
		p.keyCache.Set(resolvedKey, map[string]string{"status": "String"}, ttlcache.DefaultTTL)
	})
	q := &models.HdxQuery{
		RawSQL:  sql,
		Filters: []models.AdHocFilter{{Key: "status", Operator: "=", Value: "active"}},
	}

	out, err := i.interpolate(context.Background(), q)
	require.NoError(t, err)
	assert.Contains(t, out, "status = 'active'")
	assert.Contains(t, out, "WITH x AS", "the WITH clause is preserved around the rewritten filter")
	assert.NotContains(t, out, "$__adHocFilter")
}

func TestInterpolate_Integration_TimeMacroResolvesPKFromCache(t *testing.T) {
	// $__timeFilter with no column argument must resolve the primary key of the
	// FROM table. Pre-seed pkCache under the GetPK cache key ("<db>_<table>")
	// so the resolution hits the cache and nopMetadataDS is never queried.
	i := newIntegrationInterpolator(func(p *MetadataProvider) {
		p.pkCache.Set("mydb_events", "ts", ttlcache.DefaultTTL)
	})
	q := &models.HdxQuery{
		RawSQL:    "SELECT $__timeFilter FROM mydb.events",
		TimeRange: backend.TimeRange{From: integrationFrom, To: integrationTo},
	}

	out, err := i.interpolate(context.Background(), q)
	require.NoError(t, err)
	assert.Contains(t, out, "ts >= toDateTime(1000) AND ts <= toDateTime(2000)")
	assert.NotContains(t, out, "$__timeFilter")
}

func TestInterpolate_Integration_InjectedOperatorRejected(t *testing.T) {
	// The operator allowlist is unit-tested on buildFilterCondition directly;
	// this asserts the rejection propagates all the way out of the real
	// pipeline — dispatch returns the macro's error rather than a rewrite that
	// smuggled the injected operator into the SQL.
	i := newIntegrationInterpolator(func(p *MetadataProvider) {
		p.keyCache.Set("foo", fooSchema, ttlcache.DefaultTTL)
	})
	q := &models.HdxQuery{
		RawSQL:  "SELECT * FROM foo WHERE $__adHocFilter()",
		Filters: []models.AdHocFilter{{Key: "column", Operator: "= 'x' OR 1=1 -- ", Value: "x"}},
	}

	out, err := i.interpolate(context.Background(), q)
	require.Error(t, err, "injected operator must be rejected by the pipeline")
	assert.NotContains(t, out, "OR 1=1", "no rewrite carrying the injection may be produced")
}

func TestInterpolate_Integration_InjectedValueStaysEscaped(t *testing.T) {
	// A value crafted to break out of the string literal must stay wholly
	// inside it after the full rewrite: the quotes are escaped so the trailing
	// `OR '1'='1` is inert text, not SQL.
	i := newIntegrationInterpolator(func(p *MetadataProvider) {
		p.keyCache.Set("foo", fooSchema, ttlcache.DefaultTTL)
	})
	q := &models.HdxQuery{
		RawSQL:  "SELECT * FROM foo WHERE $__adHocFilter()",
		Filters: []models.AdHocFilter{{Key: "column", Operator: "=", Value: "x' OR '1'='1"}},
	}

	out, err := i.interpolate(context.Background(), q)
	require.NoError(t, err)
	// The whole value is one escaped literal; every embedded quote is \'-escaped.
	assert.Contains(t, out, `column = 'x\' OR \'1\'=\'1'`)
}

func TestInterpolate_Integration_InjectedMapSubscriptStaysEscaped(t *testing.T) {
	// A Map subscript crafted to break out (`mapColumn['a'] OR 1=1 --']`) must
	// end up as a backtick-quoted column plus a single escaped subscript literal
	// once it has flowed through the real pipeline.
	i := newIntegrationInterpolator(func(p *MetadataProvider) {
		p.keyCache.Set("foo", fooSchema, ttlcache.DefaultTTL)
	})
	q := &models.HdxQuery{
		RawSQL:  "SELECT * FROM foo WHERE $__adHocFilter()",
		Filters: []models.AdHocFilter{{Key: "mapColumn['a'] OR 1=1 --']", Operator: "=", Value: "v"}},
	}

	out, err := i.interpolate(context.Background(), q)
	require.NoError(t, err)
	assert.Contains(t, out, "`mapColumn`['a\\'] OR 1=1 --'] = 'v'")
}

func TestInterpolate_Integration_UnknownMacroLeftInPlace(t *testing.T) {
	// Over the REAL registry, a macro name that is not registered is left
	// verbatim (the empty-registry variant lives in interpolator_test.go).
	i := newIntegrationInterpolator(nil)
	in := "SELECT $__unknownMacro() FROM t"
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: in})
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestInterpolate_Integration_EscapedMacroStripsOneDollar(t *testing.T) {
	// $$__<macro> strips one leading $ and is not dispatched, even though the
	// macro (conditionalAll) exists in the real registry.
	i := newIntegrationInterpolator(nil)
	out, err := i.interpolate(context.Background(), &models.HdxQuery{RawSQL: "SELECT $$__conditionalAll() FROM t"})
	require.NoError(t, err)
	assert.Contains(t, out, "$__conditionalAll()")
	assert.NotContains(t, out, "1=1", "escaped macro must not expand")
}
