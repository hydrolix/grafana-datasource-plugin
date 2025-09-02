package datasource

import (
	"context"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Interpolator struct {
	md     *MetaDataProvider
	macros map[string]MacroFunc
}

type macroMatch struct {
	full string
	name string
	args []string
	pos  parser.Pos
}

func NewInterpolator(ds HydrolixDatasource) Interpolator {
	return Interpolator{NewMetaDataProvider(&ds), Macros}
}

// getMacroMatches extracts macro strings with their respective arguments from the sql input given
// It manually parses the string to find the closing parenthesis of the macro (because regex has no memory)
func getMacroMatches(input string, name string) ([]macroMatch, error) {
	rgx, err := regexp.Compile(fmt.Sprintf(`\$__%s\b`, name))

	if err != nil {
		return nil, err
	}

	var matches []macroMatch
	for _, window := range rgx.FindAllStringIndex(input, -1) {
		start, end := window[0], window[1]
		args, length := parseArgs(input[end:])
		if length < 0 {
			return nil, fmt.Errorf("failed to parse macro arguments (missing close bracket?)")
		}
		matches = append(matches, macroMatch{full: input[start : end+length], args: args, pos: parser.Pos(start), name: name})
	}
	return matches, nil
}

// parseArgs looks for a bracketed argument list at the beginning of argString.
// If one is present, returns a list of whitespace-trimmed arguments and the
// length of the string comprising the bracketed argument list.
func parseArgs(argString string) ([]string, int) {
	if !strings.HasPrefix(argString, "(") {
		return nil, 0 // single empty arg for backwards compatibility
	}

	var args []string
	depth := 0
	arg := []rune{}

	for i, r := range argString {
		switch r {
		case '(':
			depth++
			if depth == 1 {
				// don't include the outer bracket in the arg
				continue
			}
		case ')':
			depth--
			if depth == 0 {
				// closing bracket
				args = append(args, strings.TrimSpace(string(arg)))
				return args, i + 1
			}
		case ',':
			if depth == 1 {
				// a comma at this level is separating args
				args = append(args, strings.TrimSpace(string(arg)))
				arg = []rune{}
				continue
			}
		}
		arg = append(arg, r)
	}
	// If we get here, we have seen an open bracket but not a close bracket. This
	// would formerly cause a panic; now it is treated as an error.
	return nil, -1
}

// Interpolate returns an interpolated query string given a backend.DataQuery
func (i Interpolator) Interpolate(query *sqlutil.Query, ctx context.Context) (string, error) {
	// sort macros so longer macros are applied first to prevent it from being
	// overridden by a shorter macro that is a substring of the longer one
	sortedMacroKeys := make([]string, 0, len(i.macros))
	for key := range i.macros {
		sortedMacroKeys = append(sortedMacroKeys, key)
	}
	sort.Slice(sortedMacroKeys, func(i, j int) bool {
		return len(sortedMacroKeys[i]) > len(sortedMacroKeys[j])
	})
	rawSQL := query.RawSQL
	macroMatches := make([]macroMatch, 0)
	for _, key := range sortedMacroKeys {
		matches, err := getMacroMatches(rawSQL, key)
		if err != nil {
			return rawSQL, err
		}
		macroMatches = append(macroMatches, matches...)
	}

	sort.Slice(macroMatches, func(i, j int) bool {
		return macroMatches[i].pos > macroMatches[j].pos
	})
	for _, match := range macroMatches {
		macro := i.macros[match.name]
		res, err := macro(query.WithSQL(rawSQL), match.args, match.pos, i.md, ctx)
		if err != nil {
			return rawSQL, err
		}

		rawSQL = rawSQL[0:match.pos] + strings.Replace(rawSQL[match.pos:], match.full, res, 1)
	}
	return rawSQL, nil
}

type MacroId struct {
	Name  string     `json:"name"`
	Index parser.Pos `json:"index"`
}

type CTE struct {
	Macro    string     `json:"macro"`
	MacroPos parser.Pos `json:"macroPos"`
	CTE      string     `json:"cte"`
	Table    string     `json:"table"`
	Database string     `json:"database"`
	Pos      parser.Pos `json:"pos"`
}

// RoundTimeRange rounds the time range to provided time interval
func RoundTimeRange(timeRange backend.TimeRange, interval string) backend.TimeRange {
	if dInterval, err := time.ParseDuration(interval); err == nil && dInterval.Seconds() >= 1 {
		To := timeRange.To.Round(dInterval)
		From := timeRange.From.Round(dInterval)

		log.DefaultLogger.Debug("Time range rounded", "original", timeRange, "from", From, "to", To, "interval", interval)
		return backend.TimeRange{To: To, From: From}
	}

	log.DefaultLogger.Warn("Using default time range, provided round interval is invalid", "interval", interval)
	return timeRange
}

type macroVisitor struct {
	parser.DefaultASTVisitor
	macros []MacroId
}

func (v *macroVisitor) VisitFunctionExpr(expr *parser.FunctionExpr) error {
	if strings.HasPrefix(expr.Name.Name, "$__") {
		v.macros = append(v.macros, MacroId{Name: expr.Name.Name, Index: expr.Name.NamePos})
	}
	return nil
}

type tableVisitor struct {
	parser.DefaultASTVisitor
	pos      parser.Pos
	table    string
	database string
}

func (v *tableVisitor) VisitTableIdentifier(expr *parser.TableIdentifier) error {
	if v.pos == expr.Pos() {
		if expr.Table != nil {
			v.table = expr.Table.String()
		}
		if expr.Database != nil {
			v.database = expr.Database.String()
		} else {
			v.database = ""
		}

	}
	return nil
}

type queryVisitor struct {
	parser.DefaultASTVisitor
	macroIds map[MacroId]CTE
}

func (v *queryVisitor) VisitSelectQuery(expr *parser.SelectQuery) error {
	if expr.From != nil {
		cte := expr.From.Expr.String()
		pos := expr.Pos()
		tVisitor := tableVisitor{pos: pos}
		_ = expr.Accept(&tVisitor)
		mVisitor := macroVisitor{macros: make([]MacroId, 0)}
		_ = expr.Accept(&mVisitor)
		for _, macro := range mVisitor.macros {
			if existing, ok := v.macroIds[macro]; !ok || existing.Pos < pos {
				v.macroIds[macro] = CTE{Macro: macro.Name, MacroPos: macro.Index, CTE: cte, Pos: pos, Database: tVisitor.database, Table: tVisitor.table}
			}

		}
	}
	return nil
}

func GetMacroCTEs(ast []parser.Expr) (map[MacroId]CTE, error) {
	visitor := queryVisitor{macroIds: make(map[MacroId]CTE)}
	for _, expr := range ast {
		err := expr.Accept(&visitor)
		if err != nil {
			return nil, err
		}
	}
	return visitor.macroIds, nil
}
