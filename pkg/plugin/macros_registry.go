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

// Stub is a no-op macro that returns the empty string. Useful as a
// placeholder for "conditionalAll"-style passthrough macros that exist
// only so other tools (e.g. Grafana's variable-substitution preview)
// recognise the name.
func Stub(_ context.Context, _ *models.HdxQuery, _ []string, _ parser.Pos, _ *MetadataProvider) (string, error) {
	return "", nil
}

func init() {
	// conditionalAll is a no-op in the Hydrolix plugin; same as the fork.
	Macros["conditionalAll"] = Stub
}
