package cte

import (
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
)

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
	// Layers are the alias / subquery bodies between the macro and the
	// table: the first is the one the macro's SELECT reads, the last reads
	// the table. Empty when the FROM names the table directly.
	Layers []Layer
	// Err wraps ErrNoSingleSource or ErrSourceTooDeep.
	Err error
}

// Layer is one alias or subquery body a PKSource passed through.
type Layer struct {
	// Name identifies the layer in error messages: the backquoted alias, or
	// "subquery" for an inline one.
	Name  string
	items []*parser.SelectItem
	// qualifiers are the prefixes that name the table in the body's FROM,
	// so `q.col` in the projection is a column of that table: its `AS`
	// alias, its table name, and db.table. (Not the layer's own name.)
	qualifiers [][]string
}

// Exposes reports whether the layer's SELECT list makes the source column
// col visible to its reader under that name: through `*` or `t.*` (unless an
// EXCEPT drops it by name or pattern, or a REPLACE swaps in an expression
// that does not read it), the column itself (bare or qualified by the
// layer's table), or an expression over col aliased back to col.
func (l Layer) Exposes(col string) bool {
	for _, item := range l.items {
		if item.Alias != nil {
			if item.Alias.Name == col && readsColumn(item.Expr, col) {
				return true
			}
			continue
		}
		switch e := item.Expr.(type) {
		case *parser.Ident:
			if e.Name == col || (e.Name == "*" && starExposes(item.Modifiers, col)) {
				return true
			}
		case *parser.NestedIdentifier:
			// `t.*`; a qualified column `t.col` parses as a Path.
			if e.DotIdent != nil && e.DotIdent.Name == "*" && starExposes(item.Modifiers, col) {
				return true
			}
		case *parser.Path:
			if n := len(e.Fields); n > 1 && e.Fields[n-1].Name == col && l.qualifies(identNames(e.Fields[:n-1])) {
				return true
			}
		}
	}
	return false
}

// starExposes reports whether a `*` with these modifiers still yields col.
func starExposes(modifiers []*parser.FunctionExpr, col string) bool {
	for _, m := range modifiers {
		if m.Params == nil || m.Params.Items == nil {
			continue
		}
		for _, arg := range m.Params.Items.Items {
			var alias *parser.Ident
			if c, ok := arg.(*parser.ColumnExpr); ok {
				arg, alias = c.Expr, c.Alias
			}
			switch parser.Format(m.Name) {
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

// readsColumn reports whether expr mentions a column named col.
func readsColumn(expr parser.Expr, col string) bool {
	found := false
	parser.Walk(expr, func(node parser.Expr) bool {
		switch n := node.(type) {
		case *parser.Ident:
			found = n.Name == col
		case *parser.Path:
			found = len(n.Fields) > 0 && n.Fields[len(n.Fields)-1].Name == col
		}
		return !found
	})
	return found
}

func (l Layer) qualifies(prefix []string) bool {
	return slices.ContainsFunc(l.qualifiers, func(q []string) bool { return slices.Equal(q, prefix) })
}

func identNames(ids []*parser.Ident) []string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = id.Name
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
// given the WITH scopes visible to it (outermost first) and the alias names
// hidden there because the SELECT sits inside their bodies. ok is false when
// src is not an alias or a subquery, so the caller keeps the plain table it
// already has.
func pkSource(src parser.Expr, scopes []map[string]*parser.SelectQuery, hidden []string) (s PKSource, ok bool) {
	for depth := 0; ; depth++ {
		body, alias, bodyScopes, isBody := lookupBody(src, scopes, hidden)
		if !isBody {
			if depth == 0 {
				return PKSource{}, false
			}
			ti, isTable := src.(*parser.TableIdentifier)
			if !isTable || ti.Table == nil {
				s.Err = fmt.Errorf("%w in %s", ErrNoSingleSource, s.Layers[len(s.Layers)-1].Name)
				return s, true
			}
			s.Table = ti.Table.Name
			if ti.Database != nil {
				s.Database = ti.Database.Name
			}
			return s, true
		}
		name := "subquery"
		if alias != "" {
			name = "`" + alias + "`"
			// Inside its own body an alias's name means a real table, even
			// where an outer scope defines another alias of that name.
			hidden = append(slices.Clip(hidden), alias)
		}
		s.Layers = append(s.Layers, Layer{Name: name, items: body.SelectItems, qualifiers: qualifiersOf(body)})
		if depth == maxSourceDepth {
			s.Err = fmt.Errorf("%w under %s", ErrSourceTooDeep, s.Layers[0].Name)
			return s, true
		}
		if body.Union != nil || body.Except != nil || body.Intersect != nil {
			s.Err = fmt.Errorf("%w in %s", ErrNoSingleSource, name)
			return s, true
		}
		if src = SingleSource(body); src == nil {
			s.Err = fmt.Errorf("%w in %s", ErrNoSingleSource, name)
			return s, true
		}
		// Clip so the append allocates: bodyScopes may share the visitor's
		// scope stack, which must not be overwritten.
		scopes = append(slices.Clip(bodyScopes), withAliases(body))
	}
}

// lookupBody returns the SELECT a FROM source stands for when it is a
// subquery or a visible WITH alias, the alias name ("" for a subquery), and
// the scopes visible to that body: an alias's body sees the scope that
// defines it and the scopes around that, not the scopes of the reference.
func lookupBody(src parser.Expr, scopes []map[string]*parser.SelectQuery, hidden []string) (*parser.SelectQuery, string, []map[string]*parser.SelectQuery, bool) {
	switch e := src.(type) {
	case *parser.SubQuery:
		return e.Select, "", scopes, true
	case *parser.SelectQuery:
		return e, "", scopes, true
	case *parser.TableIdentifier:
		if e.Database != nil || e.Table == nil || slices.Contains(hidden, e.Table.Name) {
			return nil, "", nil, false
		}
		for i := len(scopes) - 1; i >= 0; i-- {
			if body, ok := scopes[i][e.Table.Name]; ok {
				return body, e.Table.Name, scopes[:i+1], true
			}
		}
	}
	return nil, "", nil, false
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
	e := sq.From.Expr
	for {
		j, ok := e.(*parser.JoinExpr)
		if !ok {
			return tableSource(e)
		}
		e = j.Left
	}
}

func tableSource(e parser.Expr) parser.Expr {
	jte, ok := e.(*parser.JoinTableExpr)
	if !ok || jte.Table == nil {
		return nil
	}
	source := jte.Table.Expr
	if alias, ok := source.(*parser.AliasExpr); ok {
		source = alias.Expr
	}
	return source
}
