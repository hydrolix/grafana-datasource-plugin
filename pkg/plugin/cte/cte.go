// Package cte holds the CTE / MacroId data shapes and the AST visitors
// that extract macro-to-CTE associations from a parsed ClickHouse query.
//
// The package is a sibling of pkg/plugin to break a potential import
// cycle: pkg/plugin/datasource.go imports pkg/api (for route
// registration), and pkg/api/routes.go's MacroCTEs handler needs the
// CTE types and GetMacroCTEs function — putting them in a leaf package
// satisfies both consumers.
package cte

import (
	"strings"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
)

// MacroId identifies a macro call site by name and byte position.
type MacroId struct {
	Name  string     `json:"name"`
	Index parser.Pos `json:"index"`
}

// CTE describes a single macro invocation's surrounding context: which
// macro, where in the SQL, which CTE alias (the FROM expression of the
// containing SELECT), and the resolved table / database.
type CTE struct {
	Macro    string     `json:"macro"`
	MacroPos parser.Pos `json:"macroPos"`
	CTE      string     `json:"cte"`
	Table    string     `json:"table"`
	Database string     `json:"database"`
	Pos      parser.Pos `json:"pos"`
}

// GetMacroCTEs walks the AST and returns the macro-to-CTE map used by
// the /macroCTE HTTP route and by ad-hoc-filter resolution. Behaviour
// is preserved from the fork (hydrolix/sqlds@v5.0.1, interpolator.go:255).
func GetMacroCTEs(ast []parser.Expr) (map[MacroId]CTE, error) {
	visitor := queryVisitor{macroIds: make(map[MacroId]CTE)}
	for _, expr := range ast {
		if err := expr.Accept(&visitor); err != nil {
			return nil, err
		}
	}
	return visitor.macroIds, nil
}

// MacroPositions returns the byte positions where the AST recognises
// macro identifiers (any Ident starting with `$__`). Used by the
// interpolator to restrict regex matches to real macro call sites
// (not occurrences inside string literals).
func MacroPositions(input string) ([]parser.Pos, error) {
	exps, err := parser.NewParser(input).ParseStmts()
	if err != nil {
		return nil, err
	}
	v := macroVisitor{macros: make([]MacroId, 0)}
	for _, expr := range exps {
		if err := expr.Accept(&v); err != nil {
			return nil, err
		}
	}
	positions := make([]parser.Pos, 0, len(v.macros))
	for _, m := range v.macros {
		positions = append(positions, m.Index)
	}
	return positions, nil
}

// macroVisitor collects MacroIds from the AST. Identifiers starting with
// `$__` are treated as macro call sites.
type macroVisitor struct {
	parser.DefaultASTVisitor
	macros []MacroId
}

func (v *macroVisitor) VisitIdent(expr *parser.Ident) error {
	if strings.HasPrefix(expr.Name, "$__") {
		v.macros = append(v.macros, MacroId{Name: expr.Name, Index: expr.NamePos})
	}
	return nil
}

// tableVisitor resolves a FROM <table> reference at a given byte position
// to (database, table). Supports `database.table` and `table`-only forms.
type tableVisitor struct {
	parser.DefaultASTVisitor
	pos      parser.Pos
	table    string
	database string
}

func (v *tableVisitor) VisitTableIdentifier(expr *parser.TableIdentifier) error {
	if v.pos == expr.Pos() {
		if expr.Table != nil {
			v.table = parser.Format(expr.Table)
		}
		if expr.Database != nil {
			v.database = parser.Format(expr.Database)
		} else {
			v.database = ""
		}
	}
	return nil
}

// queryVisitor walks every SELECT in the AST. For each, it captures the
// FROM expression (the CTE / table reference) and every macro invocation
// inside that SELECT, mapping macros to their CTE context.
//
// scopeStack tracks the WITH-clause aliases of each enclosing SELECT so a
// FROM reference to a CTE alias can be resolved to its defining subquery.
// SelectQuery.Accept calls Enter(s) before, and Leave(s) (deferred) after,
// VisitSelectQuery(s) and all child recursion, so during VisitSelectQuery
// the stack holds [ancestors…, s] — searched top-down, nearest scope wins.
type queryVisitor struct {
	parser.DefaultASTVisitor
	macroIds   map[MacroId]CTE
	scopeStack []map[string]*parser.SelectQuery
}

func (v *queryVisitor) Enter(expr parser.Expr) {
	sq, ok := expr.(*parser.SelectQuery)
	if !ok {
		return
	}
	aliases := map[string]*parser.SelectQuery{}
	if sq.With != nil {
		for _, c := range sq.With.CTEs {
			// The parser stores CTEs as `<Expr> AS <Alias>`; for the subquery
			// form `name AS (SELECT …)` the name is in Expr and the body in
			// Alias. Expression-form CTEs (`1 AS a`) have an *Ident Alias and
			// are skipped — they are not table sources.
			body, ok := c.Alias.(*parser.SelectQuery)
			if !ok {
				continue
			}
			name, ok := identName(c.Expr)
			if !ok {
				continue
			}
			aliases[name] = body
		}
	}
	v.scopeStack = append(v.scopeStack, aliases)
}

func (v *queryVisitor) Leave(expr parser.Expr) {
	if _, ok := expr.(*parser.SelectQuery); ok && len(v.scopeStack) > 0 {
		v.scopeStack = v.scopeStack[:len(v.scopeStack)-1]
	}
}

// resolveWithAlias returns the defining subquery for a bare-identifier FROM
// reference that matches a WITH alias in scope (nearest enclosing scope
// first), or (nil, false) if the FROM is not a bare identifier or matches no
// alias.
func (v *queryVisitor) resolveWithAlias(fromExpr parser.Expr) (*parser.SelectQuery, bool) {
	name, ok := bareIdentName(fromExpr)
	if !ok {
		return nil, false
	}
	for i := len(v.scopeStack) - 1; i >= 0; i-- {
		if body, ok := v.scopeStack[i][name]; ok {
			return body, true
		}
	}
	return nil, false
}

// identName returns the name of a plain identifier expression.
func identName(e parser.Expr) (string, bool) {
	if id, ok := e.(*parser.Ident); ok {
		return id.Name, true
	}
	return "", false
}

// bareIdentName returns the single-part table name of a FROM expression that
// is a bare identifier (no database qualifier, no subquery/function/JOIN),
// unwrapping the JoinTableExpr → TableExpr → TableIdentifier chain.
func bareIdentName(fromExpr parser.Expr) (string, bool) {
	e := fromExpr
	if jte, ok := e.(*parser.JoinTableExpr); ok {
		if jte.Table == nil {
			return "", false
		}
		e = jte.Table
	}
	if te, ok := e.(*parser.TableExpr); ok {
		e = te.Expr
	}
	if ti, ok := e.(*parser.TableIdentifier); ok && ti.Database == nil && ti.Table != nil {
		return ti.Table.Name, true
	}
	return "", false
}

func (v *queryVisitor) VisitSelectQuery(expr *parser.SelectQuery) error {
	if expr.From == nil {
		return nil
	}
	pos := expr.Pos()
	scope := parser.Format(expr.From.Expr)
	// If the FROM references a WITH alias in scope, describe the alias's
	// subquery instead of the (non-existent) table named by the alias.
	if body, ok := v.resolveWithAlias(expr.From.Expr); ok {
		scope = "(" + parser.Format(body) + ")"
	}
	tPos := expr.From.Expr.Pos()
	tVisitor := tableVisitor{pos: tPos}
	_ = expr.Accept(&tVisitor)
	mVisitor := macroVisitor{macros: make([]MacroId, 0)}
	_ = expr.Accept(&mVisitor)
	for _, macro := range mVisitor.macros {
		// Prefer the innermost SELECT for each macro (smallest containing
		// scope wins).
		if existing, ok := v.macroIds[macro]; !ok || existing.Pos < pos {
			v.macroIds[macro] = CTE{
				Macro:    macro.Name,
				MacroPos: macro.Index,
				CTE:      scope,
				Pos:      pos,
				Database: tVisitor.database,
				Table:    tVisitor.table,
			}
		}
	}
	return nil
}
