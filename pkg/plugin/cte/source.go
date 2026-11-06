package cte

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
)

// How a column-less time macro finds its table:
//
//	WITH recent AS (SELECT l.timestamp, l.status
//	                FROM sample.logs AS l
//	                WHERE l.timestamp > now() - INTERVAL 1 DAY),
//	     errors AS (SELECT * FROM recent WHERE status >= 500)
//	SELECT $__timeInterval() AS t, count()
//	FROM errors
//	WHERE $__timeFilter()
//	GROUP BY t
//
// Names resolve as ClickHouse's old analyzer resolves them; Hydrolix runs
// with enable_analyzer=0. A SELECT's main body sees all of its WITH
// aliases. An alias's body sees only the aliases defined before it in that
// WITH, plus the scopes around the WITH. It never sees itself or a later
// sibling, so either name would mean a real table there.
//
// Both macros sit in the outer SELECT's main body, so both get the same CTE
// entry:
//
//   - CTE.Table / CTE.Database are "errors" / "": the FROM as written, which
//     /macroCTE reports. CTE.CTE is the body of "errors", for ad-hoc filters.
//   - visibleScopes is [[recent, errors]]: the outer WITH, all of it, since
//     the macros are in its SELECT's main body rather than in an alias body.
//
// pkSource then walks from the FROM down to a table, one sourceBody per step:
//
//  1. lookupBody(errors) finds errors second in that WITH:
//     sourceBody{query: SELECT * FROM recent …, alias: "errors",
//     scopes: [[recent]]}, the aliases defined before it. That becomes
//     layer `errors`, whose items are [*] and whose qualifiers are
//     [recent], the table its FROM names.
//  2. The body's FROM is recent, the one alias it can see:
//     sourceBody{query: SELECT l.timestamp, …, alias: "recent",
//     scopes: [[]]}. That becomes layer `recent`, with items
//     [l.timestamp, l.status] and qualifiers [l], [logs], [sample logs].
//  3. That body's FROM, sample.logs, is database-qualified, so it is never an
//     alias. lookupBody returns nil and the walk stops at the table.
//
// The result is PKSource{Database: "sample", Table: "logs",
// layers: [`errors`, `recent`]}, nearest the macro first. pkColumn (via
// getPK) looks up sample.logs's primary key, say "timestamp", and asks
// HiddenBy which layer, innermost first, does not expose it. `recent`
// exposes it as l.timestamp, because l qualifies its own table. `errors`
// exposes it through *. Both macros then expand on timestamp.
//
// Had the "recent" CTE selected `l.timestamp AS ts` instead, it would not
// expose timestamp. The macro would fail with ErrPKNotExposed naming
// `recent` instead of emitting a filter on a column errors cannot see. Had
// errors come first in the WITH, its `recent` would be a real table.

// ErrNoSingleSource reports that a WITH alias or subquery a macro reads from
// has no single table under it to take the primary key from: its body is a
// JOIN, a table function, a UNION / EXCEPT / INTERSECT, or has no FROM.
var ErrNoSingleSource = errors.New("no single source table")

// ErrSourceTooDeep reports that a macro's FROM nests aliases and subqueries
// deeper than maxSourceDepth.
var ErrSourceTooDeep = errors.New("aliases and subqueries nest too deeply")

// maxSourceDepth bounds how many aliases and subqueries are followed. Real
// queries nest a few levels.
const maxSourceDepth = 32

// PKSource is the table a column-less time macro takes its primary key from:
// the containing SELECT's FROM (its left table, for a JOIN), followed through
// WITH aliases and subqueries to the real table underneath.
type PKSource struct {
	// Database and Table are bare names, as system.tables stores them.
	// Database is "" when the query leaves the table unqualified. Both are
	// "" when Err is set.
	Database, Table string
	// Err wraps ErrNoSingleSource or ErrSourceTooDeep.
	Err error
	// layers are the alias / subquery bodies between the macro and the
	// table: the first is the one the macro's SELECT reads, the last reads
	// the table. Empty when the FROM names the table directly.
	layers []layer
}

// HiddenBy returns the name of the innermost layer that does not expose the
// source column col (the backquoted alias, or "subquery"), or "" when every
// layer does. Innermost first, so it names the layer that lost the column.
func (source PKSource) HiddenBy(col string) string {
	for _, hiding := range slices.Backward(source.layers) {
		if !hiding.exposes(col) {
			return hiding.name
		}
	}
	return ""
}

// layer is one alias or subquery body a PKSource passed through.
type layer struct {
	// name identifies the layer in error messages: the backquoted alias, or
	// "subquery" for an inline one.
	name  string
	items []*parser.SelectItem
	// qualifiers are the prefixes that name the table in the body's FROM,
	// so `q.col` in the projection is a column of that table: its `AS`
	// alias, its table name, and db.table. (Not the layer's own name.)
	qualifiers [][]string
}

// exposes reports whether the layer's SELECT list makes the source column
// col visible to its reader under that name: through `*` or `t.*` (unless an
// EXCEPT drops it by name or pattern, or a REPLACE swaps in an expression
// that does not read it), the column itself (bare or qualified by the
// layer's table), or an expression over col aliased back to col.
func (lyr layer) exposes(col string) bool {
	for _, item := range lyr.items {
		if item.Alias != nil {
			if item.Alias.Name == col && readsColumn(item.Expr, col) {
				return true
			}
			continue
		}
		switch expr := item.Expr.(type) {
		case *parser.Ident:
			if expr.Name == col || (expr.Name == "*" && starExposes(item.Modifiers, col)) {
				return true
			}
		case *parser.NestedIdentifier:
			// `t.*`; a qualified column `t.col` parses as a Path.
			if expr.DotIdent != nil && expr.DotIdent.Name == "*" && starExposes(item.Modifiers, col) {
				return true
			}
		case *parser.FunctionExpr:
			// COLUMNS('pattern') / COLUMNS(a, b) is a matcher: each column it
			// picks keeps its own name.
			if strings.EqualFold(parser.Format(expr.Name), "COLUMNS") && columnsMatch(expr, col) &&
				starExposes(item.Modifiers, col) {
				return true
			}
		case *parser.Path:
			last := len(expr.Fields) - 1
			// `db.t.*`; `t.*` parses as a NestedIdentifier.
			if last > 0 && expr.Fields[last].Name == "*" && starExposes(item.Modifiers, col) {
				return true
			}
			if last > 0 && expr.Fields[last].Name == col && lyr.qualifies(identNames(expr.Fields[:last])) {
				return true
			}
		}
	}
	return false
}

// starExposes reports whether a `*` with these modifiers still yields col.
func starExposes(modifiers []*parser.FunctionExpr, col string) bool {
	for _, modifier := range modifiers {
		if modifier.Params == nil || modifier.Params.Items == nil {
			continue
		}
		for _, arg := range modifier.Params.Items.Items {
			var alias *parser.Ident
			if column, ok := arg.(*parser.ColumnExpr); ok {
				arg, alias = column.Expr, column.Alias
			}
			// The name carries any qualifier: EXCEPT STRICT is still EXCEPT.
			keyword, _, _ := strings.Cut(parser.Format(modifier.Name), " ")
			switch keyword {
			case "EXCEPT":
				if name, ok := IdentName(arg); ok && name == col {
					return false
				}
				// A string is a pattern; ClickHouse drops every column it
				// matches anywhere in the name, with RE2 like regexp.
				if lit, ok := arg.(*parser.StringLiteral); ok {
					if matched, err := regexp.MatchString(lit.Literal, col); err == nil && matched {
						return false
					}
				}
			case "REPLACE":
				if alias != nil && alias.Name == col && !readsColumn(arg, col) {
					return false
				}
			}
		}
	}
	return true
}

// columnsMatch reports whether a COLUMNS(...) matcher picks col: a string
// argument is an RE2 pattern matched anywhere in the name, as in ClickHouse;
// a bare argument is a column name.
func columnsMatch(matcher *parser.FunctionExpr, col string) bool {
	if matcher.Params == nil || matcher.Params.Items == nil {
		return false
	}
	for _, arg := range matcher.Params.Items.Items {
		if column, ok := arg.(*parser.ColumnExpr); ok {
			arg = column.Expr
		}
		switch arg := arg.(type) {
		case *parser.StringLiteral:
			if matched, err := regexp.MatchString(arg.Literal, col); err == nil && matched {
				return true
			}
		case *parser.Ident:
			if arg.Name == col {
				return true
			}
		}
	}
	return false
}

// readsColumn reports whether expr mentions a column named col.
func readsColumn(expr parser.Expr, col string) bool {
	found := false
	parser.Walk(expr, func(node parser.Expr) bool {
		switch node := node.(type) {
		case *parser.Ident:
			found = node.Name == col
		case *parser.Path:
			found = len(node.Fields) > 0 && node.Fields[len(node.Fields)-1].Name == col
		}
		return !found
	})
	return found
}

func (lyr layer) qualifies(prefix []string) bool {
	return slices.ContainsFunc(lyr.qualifiers, func(qualifier []string) bool { return slices.Equal(qualifier, prefix) })
}

func identNames(ids []*parser.Ident) []string {
	names := make([]string, len(ids))
	for index, id := range ids {
		names[index] = id.Name
	}
	return names
}

// qualifiersOf returns the prefixes that name a body's source table.
func qualifiersOf(body *parser.SelectQuery) [][]string {
	if body.From == nil {
		return nil
	}
	jte, ok := body.From.Expr.(*parser.JoinTableExpr)
	if !ok || jte.Table == nil {
		return nil
	}
	var qs [][]string
	source := jte.Table.Expr
	if alias, ok := source.(*parser.AliasExpr); ok {
		if name, ok := IdentName(alias.Alias); ok {
			qs = append(qs, []string{name})
		}
		source = alias.Expr
	}
	if ti, ok := source.(*parser.TableIdentifier); ok && ti.Table != nil {
		qs = append(qs, []string{ti.Table.Name})
		if ti.Database != nil {
			qs = append(qs, []string{ti.Database.Name, ti.Table.Name})
		}
	}
	return qs
}

// pkSource resolves the PKSource for a SELECT whose FROM source is src,
// given the WITH scopes visible to it (outermost first). ok is false when src
// is not an alias or a subquery, so the caller keeps the plain table it
// already has.
func pkSource(src parser.Expr, scopes [][]withAlias) (source PKSource, ok bool) {
	for depth := 0; ; depth++ {
		body := lookupBody(src, scopes)
		if body == nil {
			if depth == 0 {
				return PKSource{}, false
			}
			ti, isTable := src.(*parser.TableIdentifier)
			if !isTable || ti.Table == nil {
				source.Err = fmt.Errorf("%w in %s", ErrNoSingleSource, source.layers[len(source.layers)-1].name)
				return source, true
			}
			source.Table = ti.Table.Name
			if ti.Database != nil {
				source.Database = ti.Database.Name
			}
			return source, true
		}
		name := "subquery"
		if body.alias != "" {
			name = "`" + body.alias + "`"
		}
		source.layers = append(source.layers, layer{name: name, items: body.query.SelectItems, qualifiers: qualifiersOf(body.query)})
		if depth == maxSourceDepth {
			source.Err = fmt.Errorf("%w under %s", ErrSourceTooDeep, source.layers[0].name)
			return source, true
		}
		if body.query.Union != nil || body.query.Except != nil || body.query.Intersect != nil {
			source.Err = fmt.Errorf("%w in %s", ErrNoSingleSource, name)
			return source, true
		}
		if src = SingleSource(body.query); src == nil {
			source.Err = fmt.Errorf("%w in %s", ErrNoSingleSource, name)
			return source, true
		}
		// A copy: body.scopes may share the visitor's scope stack.
		scopes = slices.Concat(body.scopes, [][]withAlias{withAliases(body.query)})
	}
}

// sourceBody is the SELECT a FROM source stands for.
type sourceBody struct {
	query *parser.SelectQuery
	// alias is the WITH alias the source named, "" for an inline subquery.
	alias string
	// scopes are the WITH scopes visible inside query. An alias's body sees
	// the aliases defined before it in its WITH and the scopes around that
	// WITH, never itself, a later sibling, or the scopes of the reference.
	scopes [][]withAlias
}

// lookupBody returns the body of a FROM source that is a subquery or a
// visible WITH alias, or nil when src is neither.
func lookupBody(src parser.Expr, scopes [][]withAlias) *sourceBody {
	switch expr := src.(type) {
	case *parser.SubQuery:
		return &sourceBody{query: expr.Select, scopes: scopes}
	case *parser.SelectQuery:
		return &sourceBody{query: expr, scopes: scopes}
	case *parser.TableIdentifier:
		if expr.Database != nil || expr.Table == nil {
			return nil
		}
		for level, scope := range slices.Backward(scopes) {
			if index, ok := findAlias(scope, expr.Table.Name); ok {
				return &sourceBody{
					query: scope[index].body,
					alias: expr.Table.Name,
					// The scopes outside this WITH, and only the aliases
					// defined before this one in it.
					scopes: slices.Concat(scopes[:level], [][]withAlias{scope[:index]}),
				}
			}
		}
	}
	return nil
}

// SingleSource returns a SELECT's only FROM source, with any `AS name`
// removed, or nil when the FROM is a JOIN or missing.
func SingleSource(sq *parser.SelectQuery) parser.Expr {
	if sq.From == nil {
		return nil
	}
	return tableSource(sq.From.Expr)
}

// leftSource returns the source a SELECT's FROM starts with: its only source,
// or the left table of a JOIN (including ARRAY JOIN and comma joins).
func leftSource(sq *parser.SelectQuery) parser.Expr {
	if sq.From == nil {
		return nil
	}
	expr := sq.From.Expr
	for {
		join, ok := expr.(*parser.JoinExpr)
		if !ok {
			return tableSource(expr)
		}
		expr = join.Left
	}
}

func tableSource(expr parser.Expr) parser.Expr {
	jte, ok := expr.(*parser.JoinTableExpr)
	if !ok || jte.Table == nil {
		return nil
	}
	source := jte.Table.Expr
	if alias, ok := source.(*parser.AliasExpr); ok {
		source = alias.Expr
	}
	return source
}
