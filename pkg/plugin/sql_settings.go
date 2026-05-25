package plugin

import (
	"strings"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
)

// rewriteAdminCommentInSettings merges the managed Grafana attribution
// fragment into a user-supplied hdx_query_admin_comment value inside the
// trailing SETTINGS clause of the last SELECT statement in sql. This rewrite
// exists purely to prevent a user-authored SQL-level hdx_query_admin_comment
// from silently overriding the session-level managed fragment carried by the
// driver context. If the existing value contains a previously-injected managed
// block (bracketed by the adminCommentManagedStart / adminCommentManagedEnd
// markers), that block is stripped first so repeated invocations don't
// accumulate copies.
//
// Returns the rewritten SQL and ok=true on success. Returns ok=false (and an
// empty string) when:
//   - sql does not parse (e.g. dialect quirk or pre-interpolation macro),
//   - the last statement is not a SELECT,
//   - the SELECT has no trailing SETTINGS clause, or
//   - the SETTINGS clause does not contain hdx_query_admin_comment — there is
//     no override to defend against, and the session-level injection already
//     carries the managed fragment.
//
// In the ok=false case the caller should leave the SQL untouched; the
// session-level injection via clickhouse driver settings still applies.
func rewriteAdminCommentInSettings(sql, managed string) (string, bool) {
	stmts, err := parser.NewParser(sql).ParseStmts()
	if err != nil || len(stmts) == 0 {
		return "", false
	}
	// Only the last top-level statement is rewritten; earlier statements
	// keep their original SETTINGS (intentional — safer than touching every
	// statement). Rare in Grafana panels.
	sel, ok := stmts[len(stmts)-1].(*parser.SelectQuery)
	if !ok || sel.Settings == nil {
		return "", false
	}
	clause := sel.Settings

	if !hasAdminCommentItem(clause) {
		return "", false
	}

	spliceEnd := settingsClauseEndOffset(clause)

	merged := buildMergedAdminCommentValue(clause, managed)
	updateAdminCommentItem(clause, merged)

	// Splice preserves whitespace/comments outside the clause. clause.String()
	// always emits uppercase "SETTINGS " regardless of source casing —
	// cosmetic only (ClickHouse is case-insensitive), accepted trade-off.
	return sql[:clause.SettingsPos] + clause.String() + sql[spliceEnd:], true
}

// hasAdminCommentItem reports whether clause carries an hdx_query_admin_comment
// entry. Matching is case-insensitive on the key.
func hasAdminCommentItem(clause *parser.SettingsClause) bool {
	for _, item := range clause.Items {
		if item.Name != nil && strings.EqualFold(item.Name.Name, adminCommentSetting) {
			return true
		}
	}
	return false
}

// settingsClauseEndOffset returns the byte index one past the end of clause
// in the source SQL. SettingsClause.ListEnd is unfortunately inconsistent
// across value types in the upstream parser: NumberLiteral.End() is
// half-open (one-past-end), whereas StringLiteral.End() points at the
// closing quote itself. Without compensation, splicing a clause whose last
// value is a string literal would leak a duplicate closing quote into the
// output.
func settingsClauseEndOffset(clause *parser.SettingsClause) int {
	end := int(clause.ListEnd)
	if len(clause.Items) == 0 {
		return end
	}
	if _, ok := clause.Items[len(clause.Items)-1].Expr.(*parser.StringLiteral); ok {
		return end + 1
	}
	return end
}

// decodeSQLStringLiteral converts a ClickHouse single-quoted string literal's
// source form (with `''` doubling its embedded apostrophes) into its raw
// character form. The upstream parser stores StringLiteral.Literal in source
// form, so anything read from a parsed literal must be decoded before being
// concatenated with raw Go strings.
func decodeSQLStringLiteral(source string) string {
	return strings.ReplaceAll(source, "''", "'")
}

// encodeSQLStringLiteral is the inverse: it doubles any embedded `'` so the
// result is safe to put into a StringLiteral.Literal field. The upstream
// parser's StringLiteral.String() does NOT escape — it just wraps the field
// in `'…'` — so an unescaped apostrophe in the value would emit syntactically
// invalid SQL (and is an injection vector through user-controlled fields like
// panel name or display name).
func encodeSQLStringLiteral(raw string) string {
	return strings.ReplaceAll(raw, "'", "''")
}

// buildMergedAdminCommentValue extracts the current hdx_query_admin_comment
// value from clause (caller guarantees the key is present via
// hasAdminCommentItem), strips any previously-injected managed block, and
// concatenates the new managed fragment. The returned value is in RAW form —
// apostrophes are not doubled. updateAdminCommentItem re-encodes them before
// constructing the new StringLiteral.
func buildMergedAdminCommentValue(clause *parser.SettingsClause, managed string) string {
	existing := ""
	for _, item := range clause.Items {
		if item.Name == nil {
			continue
		}
		if strings.EqualFold(item.Name.Name, adminCommentSetting) {
			if lit, ok := item.Expr.(*parser.StringLiteral); ok {
				existing = decodeSQLStringLiteral(lit.Literal)
			}
			break
		}
	}
	stripped := stripManagedAdminCommentFragment(existing)
	if strings.TrimSpace(stripped) == "" {
		return managed
	}
	return stripped + "; " + managed
}

// updateAdminCommentItem replaces the existing hdx_query_admin_comment entry
// in clause with a string-literal carrying value. Caller must have verified
// the key is present via hasAdminCommentItem — a missing key is a no-op here.
// Matching is case-insensitive on the key. value is in raw form — apostrophes
// are encoded here so the emitted SQL is well-formed even when the value
// originated from a name like "O'Brien" or a user-controlled panel title.
func updateAdminCommentItem(clause *parser.SettingsClause, value string) {
	lit := &parser.StringLiteral{Literal: encodeSQLStringLiteral(value)}
	for _, item := range clause.Items {
		if item.Name != nil && strings.EqualFold(item.Name.Name, adminCommentSetting) {
			item.Expr = lit
			return
		}
	}
}
