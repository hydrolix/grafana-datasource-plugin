package plugin

import (
	"context"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

// MacroFunc is the plugin-internal macro signature. The interpolator
// dispatches every macro at its AST position with the parsed HdxQuery,
// the arg list, the byte position in the rawSQL, and a reference to the
// MetadataProvider for macros that need PK / column-type lookups.
//
// The signature mirrors the fork's (hydrolix/sqlds@v5.0.1, macros.go:27)
// to keep the macro ports mechanical.
type MacroFunc func(
	ctx context.Context,
	query *models.HdxQuery,
	args []string,
	pos parser.Pos,
	md *MetadataProvider,
) (string, error)

// Macros is the package-level registry the HdxInterpolator dispatches
// through. C6 (ClickHouse time/date macros) and C7 (adHocFilter)
// populate this map via init() in their respective files. The map is
// read-only after init — concurrent reads from many interpolator
// dispatches are safe without synchronization.
var Macros = map[string]MacroFunc{}

// Stub is a placeholder macro that expands to the SQL identity `1=1`. The
// `conditionalAll` macro is the only consumer today — until a real
// implementation lands (matching Grafana's "if All is selected, expand to
// truth") the stub keeps queries parseable rather than emitting empty
// strings that would render `SELECT  FROM t`. Matches the fork at 0f83082.
func Stub(_ context.Context, _ *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	return "1=1", nil
}

func init() {
	// conditionalAll is a no-op in the Hydrolix plugin; same as the fork.
	Macros["conditionalAll"] = Stub
}
