package cte

import (
	"errors"
	"fmt"
	"slices"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
)

// ErrNoSingleSource reports that a WITH alias or subquery a macro reads from
// has no single table under it to take the primary key from: its body is a
// JOIN, a table function, a UNION / EXCEPT / INTERSECT, or has no FROM.
var ErrNoSingleSource = errors.New("no single source table")

// maxSourceDepth bounds how many aliases and subqueries are followed. Real
// queries nest a few levels; a self-referencing alias would otherwise loop.
const maxSourceDepth = 32

// PKSource is the table a column-less time macro takes its primary key from:
// the containing SELECT's FROM, followed through WITH aliases and subqueries
// to the real table underneath.
type PKSource struct {
	// Database and Table are bare names, as system.tables stores them.
	// Database is "" when the query leaves the table unqualified.
	Database, Table string
	// Layers are the alias / subquery bodies between the macro and the
	// table, outermost first. Empty when the FROM names the table directly.
	Layers []Layer
	// Err wraps ErrNoSingleSource when some layer has no single table.
	Err error
}

// Layer is one alias or subquery body a PKSource passed through.
type Layer struct {
	// Name identifies the layer in error messages: the backquoted alias, or
	// "subquery" for an inline one.
	Name  string
	items []*parser.SelectItem
}

// Exposes reports whether the layer's SELECT list makes a column named col
// visible to its reader: through `*`, `t.*`, the column itself (bare or
// qualified), or any expression aliased to col.
func (l Layer) Exposes(col string) bool {
	for _, item := range l.items {
		if item.Alias != nil {
			if item.Alias.Name == col {
				return true
			}
			continue
		}
		var name string
		switch e := item.Expr.(type) {
		case *parser.Ident:
			name = e.Name
		case *parser.NestedIdentifier:
			if e.DotIdent != nil {
				name = e.DotIdent.Name
			}
		case *parser.Path:
			if len(e.Fields) > 0 {
				name = e.Fields[len(e.Fields)-1].Name
			}
		}
		if name == "*" || name == col {
			return true
		}
	}
	return false
}

// pkSource resolves the PKSource for a SELECT whose FROM is src, given the
// WITH scopes visible to it (outermost first). ok is false when src is not an
// alias or a subquery, so the caller keeps the plain table it already has.
func pkSource(src parser.Expr, scopes []map[string]*parser.SelectQuery) (s PKSource, ok bool) {
	for depth := 0; ; depth++ {
		body, name, bodyScopes, isBody := lookupBody(src, scopes)
		if !isBody {
			if depth == 0 {
				return PKSource{}, false
			}
			ti, isTable := src.(*parser.TableIdentifier)
			if !isTable || ti.Table == nil {
				s.Err = noSingleSource(s.Layers)
				return s, true
			}
			s.Table = ti.Table.Name
			if ti.Database != nil {
				s.Database = ti.Database.Name
			}
			return s, true
		}
		s.Layers = append(s.Layers, Layer{Name: name, items: body.SelectItems})
		if depth == maxSourceDepth || body.Union != nil || body.Except != nil || body.Intersect != nil {
			s.Err = noSingleSource(s.Layers)
			return s, true
		}
		if src = SingleSource(body); src == nil {
			s.Err = noSingleSource(s.Layers)
			return s, true
		}
		// Clip so the append allocates: bodyScopes may share the visitor's
		// scope stack, which must not be overwritten.
		scopes = append(slices.Clip(bodyScopes), withAliases(body))
	}
}

func noSingleSource(layers []Layer) error {
	return fmt.Errorf("%w in %s", ErrNoSingleSource, layers[len(layers)-1].Name)
}

// lookupBody returns the SELECT a FROM source stands for when it is a
// subquery or a WITH alias in scope, with the scopes visible to that body.
func lookupBody(src parser.Expr, scopes []map[string]*parser.SelectQuery) (*parser.SelectQuery, string, []map[string]*parser.SelectQuery, bool) {
	switch e := src.(type) {
	case *parser.SubQuery:
		return e.Select, "subquery", scopes, true
	case *parser.SelectQuery:
		return e, "subquery", scopes, true
	case *parser.TableIdentifier:
		if e.Database != nil || e.Table == nil {
			return nil, "", nil, false
		}
		for i := len(scopes) - 1; i >= 0; i-- {
			if body, ok := scopes[i][e.Table.Name]; ok {
				return body, "`" + e.Table.Name + "`", scopes[:i+1], true
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
	jte, ok := sq.From.Expr.(*parser.JoinTableExpr)
	if !ok || jte.Table == nil {
		return nil
	}
	source := jte.Table.Expr
	if alias, ok := source.(*parser.AliasExpr); ok {
		source = alias.Expr
	}
	return source
}
