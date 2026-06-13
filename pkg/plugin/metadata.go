package plugin

import (
	"context"
	"errors"
	"net/http"

	"github.com/hydrolix/clickhouse-sql-parser/parser"
)

// MetadataProvider is a stub placeholder at C5. The real implementation —
// TTL-cached primary-key and column-type lookups against the datasource —
// lands in C7 (plugin-adhoc-filter-macro-secure). C5 defines the type so
// the interpolator and macro registry compile against it; C7 replaces
// this file with the real provider plus the getPK helper.
//
// Macros that need metadata (currently none registered in C5) MUST
// tolerate `md == nil` by returning a descriptive error rather than
// panicking on a nil deref. C6's time/date macros that fall through to
// PK lookup will surface ErrMetadataProviderUnavailable when invoked
// against a C5-only build; C7's full provider replaces that.
type MetadataProvider struct{}

// ErrMetadataProviderUnavailable is returned by getPK when the provider
// is the C5 stub. Macros use this to fail-fast rather than panic.
var ErrMetadataProviderUnavailable = errors.New("metadata provider is not yet available (C5 stub; replaced by C7)")

// NewMetadataProvider returns the stub at C5. C7 changes the signature to
// take *HdxSqlDatasource and wire the real caches.
func NewMetadataProvider() *MetadataProvider {
	return &MetadataProvider{}
}

// getPK is a stub at C5; C7 supplies the real implementation backed by
// MetadataProvider's PK cache. Macros that need PK resolution (timeFilter,
// timeInterval, …) error out cleanly when invoked against the stub.
// The function is referenced by C6's macros that ship into the same
// package; the unused-warning suppression is intentional for C5.
//
//nolint:unused // exists for C6's PK-lookup macros to call; live as of C6.
func getPK(_ context.Context, _ string, _ parser.Pos, _ *MetadataProvider, _ http.Header) (string, error) {
	return "", ErrMetadataProviderUnavailable
}
