package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

// ErrParseMacroArgs is returned when a macro's argument list is opened
// with `(` but never closed. Wraps the legacy fork's error text for
// compatibility with consumers that match on it.
var ErrParseMacroArgs = errors.New("failed to parse macro arguments (missing close bracket?)")

// HdxInterpolator owns the SQL-rewrite pipeline for the Hydrolix plugin.
// Its Interpolate method satisfies sqlds.Interpolator — the func-typed
// extension surface on sqlds.SQLDatasource. It is installed as a method
// value in NewHdxSqlDatasource.
//
// The interpolator is safe for concurrent use across queries: it carries
// no mutable state. The `macros` map is populated at init() time by C6 /
// C7 and is read-only thereafter; the metadata provider is the only
// other field and its public surface is also concurrency-safe.
type HdxInterpolator struct {
	md     *MetadataProvider
	macros map[string]MacroFunc
}

// Compile-time assertion that the Interpolate method value conforms to the
// sqlds.Interpolator func type. If sqlds changes the signature, this breaks
// here rather than at the wiring site in NewHdxSqlDatasource.
var _ sqlds.Interpolator = (&HdxInterpolator{}).Interpolate

// NewHdxInterpolator constructs the interpolator with the provided
// MetadataProvider and macro registry. The caller (NewHdxSqlDatasource)
// passes the package-level Macros map; tests can pass a fixture map to
// exercise individual macros in isolation.
func NewHdxInterpolator(md *MetadataProvider, macros map[string]MacroFunc) *HdxInterpolator {
	return &HdxInterpolator{md: md, macros: macros}
}

// Interpolate implements sqlds.Interpolator. The func type passes
// (*sqlutil.Query, json.RawMessage); the plugin reconstitutes the
// Hydrolix-specific HdxQuery by unmarshalling rawJSON and overlaying the
// runtime fields from query (TimeRange, Interval). The datasource itself
// is not a parameter — it is captured via md/macros at construction.
//
// Headers are not part of the sqlds Interpolator signature. The macros
// that need them (C7's adHocFilter) pull headers from context; the
// route handler at /interpolate (pkg/api/routes.go) is responsible for
// putting them there.
func (i *HdxInterpolator) Interpolate(
	ctx context.Context,
	query *sqlutil.Query,
	rawJSON json.RawMessage,
) (string, error) {
	hdx := &models.HdxQuery{}
	if len(rawJSON) > 0 {
		if err := json.Unmarshal(rawJSON, hdx); err != nil {
			return "", backend.DownstreamError(fmt.Errorf("interpolator: unmarshal HdxQuery: %w", err))
		}
	}
	// Runtime fields from sqlds-side query take precedence over anything
	// the rawJSON may have carried — the sqlds-derived values are what
	// the request actually executed with.
	hdx.RawSQL = query.RawSQL
	hdx.TimeRange = query.TimeRange
	hdx.Interval = query.Interval

	return i.interpolate(ctx, hdx)
}

// interpolate is the package-private dispatch routine. Takes an HdxQuery
// (which carries all Hydrolix-specific context) and returns the rewritten
// SQL. Port of the fork's Interpolator.Interpolate (hydrolix/sqlds@v5.0.1
// interpolator.go:122).
func (i *HdxInterpolator) interpolate(ctx context.Context, query *models.HdxQuery) (string, error) {
	if query.Round != "" && query.Round != "0" {
		query.TimeRange = roundTimeRange(query.TimeRange, query.Round)
	}

	// Sort macros so longer names match first; prevents "timeFilter" from
	// shadowing "timeFilter_ms" at the regex level.
	sortedMacroKeys := make([]string, 0, len(i.macros))
	for key := range i.macros {
		sortedMacroKeys = append(sortedMacroKeys, key)
	}
	sort.Slice(sortedMacroKeys, func(a, b int) bool {
		return len(sortedMacroKeys[a]) > len(sortedMacroKeys[b])
	})

	rawSQL := query.RawSQL
	positions, err := cte.MacroPositions(rawSQL)
	if err != nil {
		// Parse failed — fall back to matching every macro everywhere
		// (positions == nil disables the AST-anchored filter). The macros
		// themselves may still parse-fail downstream, surfacing as proper
		// errors to the caller.
		positions = nil
	}

	macroMatches := make([]macroMatch, 0)
	for _, key := range sortedMacroKeys {
		matches, err := getMacroMatches(rawSQL, key, positions)
		if err != nil {
			return rawSQL, err
		}
		macroMatches = append(macroMatches, matches...)
	}

	// Apply replacements in reverse byte order so earlier offsets stay
	// valid as later regions shrink/grow.
	sort.Slice(macroMatches, func(a, b int) bool {
		return macroMatches[a].pos > macroMatches[b].pos
	})

	for _, match := range macroMatches {
		if match.escaped {
			// Escaped macros ($$__name) strip one leading $ and pass through
			// verbatim — used by dashboard authors who want a literal "$__"
			// in the output SQL.
			rawSQL = rawSQL[0:match.pos] + strings.Replace(rawSQL[match.pos:], "$", "", 1)
			continue
		}
		macro, ok := i.macros[match.name]
		if !ok {
			// Macro is in the AST but not registered. Leave it in place —
			// downstream parsing will surface the issue as a SQL error.
			continue
		}
		res, err := macro(ctx, query.WithSQL(rawSQL), match.args, match.pos, i.md)
		if err != nil {
			return rawSQL, err
		}
		rawSQL = rawSQL[0:match.pos] + strings.Replace(rawSQL[match.pos:], match.full, res, 1)
	}
	return rawSQL, nil
}

// macroMatch is one matched macro call site: the full text (`$__name(args)`),
// its name, parsed args, escape flag, and byte position.
type macroMatch struct {
	full    string
	name    string
	args    []string
	escaped bool
	pos     parser.Pos
}

// getMacroMatches walks `input` looking for macros with the given name.
// Position filtering (when `positions != nil`) restricts matches to byte
// offsets the AST visitor recognised — this prevents inadvertent matching
// of `$__name` inside string literals.
func getMacroMatches(input, name string, positions []parser.Pos) ([]macroMatch, error) {
	rgx, err := regexp.Compile(fmt.Sprintf(`\$+__%s\b`, name))
	if err != nil {
		return nil, err
	}

	var matches []macroMatch
	for _, window := range rgx.FindAllStringIndex(input, -1) {
		start, end := window[0], window[1]
		args, length := parseArgs(input[end:])
		if length < 0 {
			return nil, ErrParseMacroArgs
		}
		if positions == nil || slices.Contains(positions, parser.Pos(start)) {
			matches = append(matches, macroMatch{
				full:    input[start : end+length],
				args:    args,
				escaped: input[start+1] == '$',
				pos:     parser.Pos(start),
				name:    name,
			})
		}
	}
	return matches, nil
}

// parseArgs scans a bracketed argument list at the start of `argString`.
// Returns the trimmed args and the consumed length, or (nil, -1) when an
// open bracket has no matching close. (nil, 0) means no bracketed list
// was present at all — the macro takes no arguments.
func parseArgs(argString string) ([]string, int) {
	if !strings.HasPrefix(argString, "(") {
		return nil, 0
	}

	var args []string
	depth := 0
	arg := []rune{}

	for i, r := range argString {
		switch r {
		case '(':
			depth++
			if depth == 1 {
				continue
			}
		case ')':
			depth--
			if depth == 0 {
				args = append(args, strings.TrimSpace(string(arg)))
				return args, i + 1
			}
		case ',':
			if depth == 1 {
				args = append(args, strings.TrimSpace(string(arg)))
				arg = []rune{}
				continue
			}
		}
		arg = append(arg, r)
	}
	return nil, -1
}

// roundTimeRange rounds a TimeRange's endpoints to the nearest multiple of
// `interval`. Returns the original range unchanged if `interval` is
// non-parseable or sub-second. Port of the fork's RoundTimeRange (interpolator.go:185).
func roundTimeRange(timeRange backend.TimeRange, interval string) backend.TimeRange {
	dInterval, err := time.ParseDuration(interval)
	if err != nil || dInterval.Seconds() < 1 {
		log.DefaultLogger.Warn("Using default time range, provided round interval is invalid", "interval", interval)
		return timeRange
	}
	to := timeRange.To.Round(dInterval)
	from := timeRange.From.Round(dInterval)
	log.DefaultLogger.Debug("Time range rounded", "original", timeRange, "from", from, "to", to, "interval", interval)
	return backend.TimeRange{To: to, From: from}
}
