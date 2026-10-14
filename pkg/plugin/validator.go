package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

// EXPLAIN reads no data, but ClickHouse evaluates scalar subqueries during
// analysis, so a dry-run can still run long.
const validationTimeout = 10 * time.Second

const msgValidationTimedOut = "Validation timed out"

const explainPrefix = "EXPLAIN "

var (
	syntaxErrorPosition = regexp.MustCompile(`failed at position (\d+)`)
	firstLineColumn     = regexp.MustCompile(`\(line 1, col (\d+)\)`)
)

// QueryValidator backs the /validate resource.
type QueryValidator struct {
	interpolator     *HdxInterpolator
	metadataProvider *MetadataProvider
	timeout          time.Duration
}

func NewQueryValidator(interp *HdxInterpolator, md *MetadataProvider) *QueryValidator {
	return &QueryValidator{
		interpolator:     interp,
		metadataProvider: md,
		timeout:          validationTimeout,
	}
}

// Validate reports query problems in the result; it returns an error only
// when the caller is gone.
func (v *QueryValidator) Validate(ctx context.Context, q models.HdxQuery) (models.ValidationResult, error) {
	validateCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	sql, err := v.interpolator.interpolate(validateCtx, &q)
	if err != nil {
		return failure(ctx, validateCtx, err)
	}

	stmts, parseErr := parser.NewParser(sql).ParseStmts()
	var selectQuery *parser.SelectQuery
	if parseErr == nil && len(stmts) == 1 {
		selectQuery, _ = stmts[0].(*parser.SelectQuery)
	}
	// Our parser has gaps, so unparseable but query-like SQL still goes to
	// EXPLAIN; only the PK check needs the AST.
	if selectQuery == nil && (parseErr == nil || !looksLikeQuery(sql)) {
		return models.ValidationResult{Skipped: true}, nil
	}

	if err := v.explain(validateCtx, q, sql); err != nil {
		result, err := failure(ctx, validateCtx, err)
		result.Error = positionsInUserSQL(result.Error)
		return result, err
	}

	if selectQuery == nil {
		return models.ValidationResult{}, nil
	}
	if tables := unconstrainedPKTables(validateCtx, selectQuery, q.Headers, v.metadataProvider.GetPK); len(tables) > 0 {
		return models.ValidationResult{Warning: pkWarning(tables)}, nil
	}
	return models.ValidationResult{}, nil
}

func (v *QueryValidator) explain(ctx context.Context, q models.HdxQuery, sql string) error {
	payload := map[string]any{
		"rawSql":        explainPrefix + sql,
		"format":        1,
		"querySettings": q.QuerySettings,
	}
	_, err := v.metadataProvider.executeQueryJSON(withoutInterpolation(ctx), q.Headers, payload, "validate_query")
	return err
}

// failure treats any timeout as a warning: sqlds applies the datasource's own
// query timeout, which may expire before ours.
func failure(ctx, validateCtx context.Context, err error) (models.ValidationResult, error) {
	switch {
	case ctx.Err() != nil:
		return models.ValidationResult{}, ctx.Err()
	case validateCtx.Err() != nil || errors.Is(err, context.DeadlineExceeded):
		return models.ValidationResult{Warning: msgValidationTimedOut}, nil
	default:
		return models.ValidationResult{Error: errorText(err)}, nil
	}
}

// errorText keeps an empty error message from reading as "valid".
func errorText(err error) string {
	if msg := err.Error(); strings.TrimSpace(msg) != "" {
		return msg
	}
	return fmt.Sprintf("query rejected (%T)", err)
}

// positionsInUserSQL removes the EXPLAIN prefix from syntax-error offsets;
// only the absolute position and line-1 columns include it.
func positionsInUserSQL(msg string) string {
	msg = shiftOffset(msg, syntaxErrorPosition, "failed at position %d")
	return shiftOffset(msg, firstLineColumn, "(line 1, col %d)")
}

func shiftOffset(msg string, re *regexp.Regexp, format string) string {
	return re.ReplaceAllStringFunc(msg, func(match string) string {
		n, err := strconv.Atoi(re.FindStringSubmatch(match)[1])
		if err != nil || n <= len(explainPrefix) {
			return match
		}
		return fmt.Sprintf(format, n-len(explainPrefix))
	})
}

// looksLikeQuery reports whether sql starts like a SELECT, ignoring leading
// comments.
func looksLikeQuery(sql string) bool {
	rest := strings.TrimSpace(sql)
	for {
		switch {
		case strings.HasPrefix(rest, "--"), strings.HasPrefix(rest, "#"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				return false
			}
			rest = strings.TrimSpace(rest[end+1:])
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest, "*/")
			if end < 0 {
				return false
			}
			rest = strings.TrimSpace(rest[end+2:])
		default:
			return strings.HasPrefix(rest, "(") || startsWithKeyword(rest, "SELECT") || startsWithKeyword(rest, "WITH")
		}
	}
}

func startsWithKeyword(s, keyword string) bool {
	if len(s) < len(keyword) || !strings.EqualFold(s[:len(keyword)], keyword) {
		return false
	}
	if len(s) == len(keyword) {
		return true
	}
	return !isIdentifierByte(s[len(keyword)])
}

func isIdentifierByte(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

func pkWarning(tables []string) string {
	return fmt.Sprintf("%s not filtered in WHERE; add $__timeFilter() to limit the scan.", strings.Join(tables, ", "))
}

// pkResolver matches MetadataProvider.GetPK, injectable for tests.
type pkResolver func(ctx context.Context, headers http.Header, database, table string) (string, error)

// unconstrainedPKTables lists tables whose leading primary-key column no
// filter references.
func unconstrainedPKTables(ctx context.Context, stmt *parser.SelectQuery, headers http.Header, resolve pkResolver) []string {
	graph := newSelectGraph(stmt)
	var found []string
	for _, sq := range graph.selects {
		if ctx.Err() != nil {
			break
		}
		table := graph.baseTable(sq)
		if table == nil {
			continue
		}
		column, ok := primaryKeyColumn(ctx, headers, resolve, table)
		if !ok || graph.filters(sq, column) {
			continue
		}
		description := fmt.Sprintf("Primary key `%s` of `%s`", column, parser.Format(table))
		if !slices.Contains(found, description) {
			found = append(found, description)
		}
	}
	return found
}

// primaryKeyColumn returns table's leading primary-key column. Lookup failures
// are skipped: a metadata problem must not become a query warning.
func primaryKeyColumn(ctx context.Context, headers http.Header, resolve pkResolver, table *parser.TableIdentifier) (string, bool) {
	database := ""
	if table.Database != nil {
		database = table.Database.Name
	}
	primaryKey, err := resolve(ctx, headers, database, table.Table.Name)
	if err != nil {
		if !errors.Is(err, ErrPrimaryKeyNotFound) && ctx.Err() == nil {
			log.DefaultLogger.Warn("query validation: skipping PK check", "database", database, "table", table.Table.Name, "err", err)
		}
		return "", false
	}
	return leadingPKColumn(primaryKey)
}

// selectGraph holds every SELECT in a statement and, for each FROM subquery or
// CTE body, the SELECTs reading it. ClickHouse pushes a reader's WHERE down
// into its source.
type selectGraph struct {
	selects   []*parser.SelectQuery
	cteBodies map[string]*parser.SelectQuery
	readers   map[*parser.SelectQuery][]*parser.SelectQuery
}

func newSelectGraph(stmt *parser.SelectQuery) selectGraph {
	graph := selectGraph{
		cteBodies: map[string]*parser.SelectQuery{},
		readers:   map[*parser.SelectQuery][]*parser.SelectQuery{},
	}
	parser.Walk(stmt, func(node parser.Expr) bool {
		switch n := node.(type) {
		case *parser.CTEStmt:
			// Subquery-form CTEs store the name in Expr and the body in Alias.
			if body, ok := n.Alias.(*parser.SelectQuery); ok {
				if name, ok := cte.IdentName(n.Expr); ok {
					graph.cteBodies[name] = body
				}
			}
		case *parser.SelectQuery:
			graph.selects = append(graph.selects, n)
		}
		return true
	})
	for _, sq := range graph.selects {
		for _, branch := range setBranches(graph.source(sq)) {
			graph.readers[branch] = append(graph.readers[branch], sq)
		}
	}
	return graph
}

// source returns the SELECT sq reads from — a FROM subquery or a CTE body.
func (g selectGraph) source(sq *parser.SelectQuery) *parser.SelectQuery {
	switch s := singleSource(sq).(type) {
	case *parser.SubQuery:
		return s.Select
	case *parser.SelectQuery:
		return s
	case *parser.TableIdentifier:
		if s.Database == nil && s.Table != nil {
			return g.cteBodies[s.Table.Name]
		}
	}
	return nil
}

// baseTable returns the table sq reads when its only FROM source is a table,
// not a CTE.
func (g selectGraph) baseTable(sq *parser.SelectQuery) *parser.TableIdentifier {
	ti, ok := singleSource(sq).(*parser.TableIdentifier)
	if !ok || ti.Table == nil || (ti.Database == nil && g.cteBodies[ti.Table.Name] != nil) {
		return nil
	}
	return ti
}

// filters reports whether sq, or every reader of sq, filters on col.
func (g selectGraph) filters(sq *parser.SelectQuery, col string) bool {
	visiting := map[*parser.SelectQuery]bool{} // CTEs can read each other
	var check func(*parser.SelectQuery) bool
	check = func(s *parser.SelectQuery) bool {
		if whereReferences(s, col) {
			return true
		}
		if len(g.readers[s]) == 0 || visiting[s] {
			return false
		}
		visiting[s] = true
		defer delete(visiting, s)
		for _, reader := range g.readers[s] {
			if !check(reader) {
				return false
			}
		}
		return true
	}
	return check(sq)
}

func whereReferences(sq *parser.SelectQuery, col string) bool {
	return (sq.Prewhere != nil && referencesColumn(sq.Prewhere.Expr, col)) ||
		(sq.Where != nil && referencesColumn(sq.Where.Expr, col))
}

// setBranches returns sq and its UNION / EXCEPT / INTERSECT branches.
func setBranches(sq *parser.SelectQuery) []*parser.SelectQuery {
	var branches []*parser.SelectQuery
	for sq != nil {
		branches = append(branches, sq)
		switch {
		case sq.Union != nil:
			sq = sq.Union
		case sq.Except != nil:
			sq = sq.Except
		default:
			sq = sq.Intersect
		}
	}
	return branches
}

// singleSource returns a SELECT's only FROM source, or nil for a JOIN.
func singleSource(sq *parser.SelectQuery) parser.Expr {
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

// leadingPKColumn returns the first column of a primary_key value: the one
// the sparse index mainly skips data on. Expression keys report false.
func leadingPKColumn(primaryKey string) (string, bool) {
	if strings.TrimSpace(primaryKey) == "" {
		return "", false
	}
	stmts, err := parser.NewParser("SELECT " + primaryKey).ParseStmts()
	if err != nil || len(stmts) != 1 {
		return "", false
	}
	sel, ok := stmts[0].(*parser.SelectQuery)
	if !ok || len(sel.SelectItems) == 0 {
		return "", false
	}
	return cte.IdentName(sel.SelectItems[0].Expr)
}

// referencesColumn matches col exactly (ClickHouse is case-sensitive), bare or
// qualified. Mentions in nested subqueries count; that can only hide a warning.
func referencesColumn(expr parser.Expr, col string) bool {
	found := false
	parser.Walk(expr, func(node parser.Expr) bool {
		if id, ok := node.(*parser.Ident); ok && id.Name == col {
			found = true
		}
		return !found
	})
	return found
}
