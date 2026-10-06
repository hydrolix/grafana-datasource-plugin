package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

// validationTimeout bounds the EXPLAIN dry-run. Plain EXPLAIN reads no table
// data, but ClickHouse evaluates scalar subqueries during analysis, so a
// pathological query can still run long.
const validationTimeout = 10 * time.Second

const msgValidationTimedOut = "Validation timed out"

// QueryValidator backs the /validate resource: it interpolates the editor's
// query, dry-runs it with EXPLAIN through the regular sqlds query path, and
// warns when a table's primary (timestamp) key is not filtered.
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

// Validate reports query problems in the result and returns an error only
// when the caller's context is done (the client is gone, so the result would
// be discarded anyway).
func (v *QueryValidator) Validate(ctx context.Context, q models.HdxQuery) (models.ValidationResult, error) {
	sql, err := v.interpolator.interpolate(ctx, &q)
	if err != nil {
		if ctx.Err() != nil {
			return models.ValidationResult{}, ctx.Err()
		}
		return models.ValidationResult{Error: err.Error()}, nil
	}

	// A parse failure still goes to EXPLAIN: ClickHouse is the authority on
	// syntax and the plugin's parser has known gaps. Only the PK check, which
	// needs the AST, is skipped.
	stmts, parseErr := parser.NewParser(sql).ParseStmts()
	var selectQuery *parser.SelectQuery
	if parseErr == nil && len(stmts) == 1 {
		selectQuery, _ = stmts[0].(*parser.SelectQuery)
	}
	if parseErr == nil && selectQuery == nil {
		return models.ValidationResult{Skipped: true}, nil
	}

	problem, err := v.explain(ctx, q, sql)
	if err != nil {
		return models.ValidationResult{}, err
	}
	if problem != nil {
		return *problem, nil
	}

	if selectQuery != nil {
		if tables := unconstrainedPKTables(ctx, selectQuery, q.Headers, v.metadataProvider.GetPK); len(tables) > 0 {
			return models.ValidationResult{Warning: pkWarning(tables)}, nil
		}
	}
	return models.ValidationResult{}, nil
}

// explain dry-runs sql and returns the problem it found, or nil when the
// cluster accepted the query.
func (v *QueryValidator) explain(ctx context.Context, q models.HdxQuery, sql string) (*models.ValidationResult, error) {
	explainCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// sql is already interpolated; without skipInterpolation the sqlds path
	// would run the macros again over the EXPLAIN text.
	payload := map[string]any{
		"rawSql":            "EXPLAIN " + sql,
		"format":            1,
		"querySettings":     q.QuerySettings,
		"skipInterpolation": true,
	}
	_, err := v.metadataProvider.executeQueryJSON(explainCtx, q.Headers, payload, "validate_query")
	switch {
	case err == nil:
		return nil, nil
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case errors.Is(explainCtx.Err(), context.DeadlineExceeded):
		return &models.ValidationResult{Warning: msgValidationTimedOut}, nil
	default:
		return &models.ValidationResult{Error: err.Error()}, nil
	}
}

func pkWarning(tables []string) string {
	return fmt.Sprintf("%s not filtered in WHERE; add $__timeFilter() to limit the scan.", strings.Join(tables, ", "))
}

// pkResolver matches MetadataProvider.GetPK; injected so the PK check can be
// tested without a schema query.
type pkResolver func(ctx context.Context, headers http.Header, database, table string) (string, error)

// unconstrainedPKTables returns a description of every single-table SELECT in
// stmt whose leading primary-key column is not referenced by its WHERE or
// PREWHERE. Lookup failures and tables without a usable key are skipped:
// a metadata problem must never surface as a query warning.
func unconstrainedPKTables(ctx context.Context, stmt *parser.SelectQuery, headers http.Header, resolve pkResolver) []string {
	cteNames := map[string]bool{}
	var selects []*parser.SelectQuery
	parser.Walk(stmt, func(node parser.Expr) bool {
		switch n := node.(type) {
		case *parser.CTEStmt:
			// Subquery-form CTEs store the name in Expr and the body in Alias.
			if _, ok := n.Alias.(*parser.SelectQuery); ok {
				if name, ok := cte.IdentName(n.Expr); ok {
					cteNames[name] = true
				}
			}
		case *parser.SelectQuery:
			selects = append(selects, n)
		}
		return true
	})

	var found []string
	for _, sq := range selects {
		table := singleTable(sq)
		if table == nil || (table.Database == nil && cteNames[table.Table.Name]) {
			continue
		}
		database := ""
		if table.Database != nil {
			database = table.Database.Name
		}
		primaryKey, err := resolve(ctx, headers, database, table.Table.Name)
		if err != nil {
			log.DefaultLogger.Debug("query validation: skipping PK check", "table", table.Table.Name, "err", err)
			continue
		}
		column, ok := leadingPKColumn(primaryKey)
		if !ok {
			continue
		}
		if sq.Prewhere != nil && referencesColumn(sq.Prewhere.Expr, column) {
			continue
		}
		if sq.Where != nil && referencesColumn(sq.Where.Expr, column) {
			continue
		}
		found = append(found, fmt.Sprintf("Primary key `%s` of `%s`", column, parser.Format(table)))
	}
	return found
}

// singleTable returns the table a SELECT reads when its FROM is exactly one
// plain table reference; JOINs, subqueries and table functions yield nil.
func singleTable(sq *parser.SelectQuery) *parser.TableIdentifier {
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
	ti, ok := source.(*parser.TableIdentifier)
	if !ok || ti.Table == nil {
		return nil
	}
	return ti
}

// leadingPKColumn extracts the first column of a system.tables.primary_key
// value. Only the leading key column lets the sparse primary index skip data,
// so it is the one a filter must hit. Expression keys (toStartOfHour(ts))
// have no single column to look for and report false.
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

// referencesColumn reports whether expr mentions col anywhere, bare or
// qualified (`t.col` parses to a Path whose fields are visited as Idents).
// ClickHouse identifiers are case-sensitive, so the match is exact. Mentions
// inside a nested subquery count too: that can only suppress a warning, never
// raise a false one, which is the right side to err on.
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
