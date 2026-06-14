package plugin

import (
	"context"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v5"
)

// connectionCacheTTL is the per-entry TTL for the per-user *sql.DB cache.
// Matches the fork's hardcoded one-hour choice; see C3's design D7.
const connectionCacheTTL = time.Hour

// HdxSqlDatasource is the Hydrolix plugin's wrapper around sqlds.SQLDatasource.
// It centralises extension-point wiring (Interpolator, ConnectionCacheFactory)
// and per-instance configuration in one constructor.
//
// The embedded *sqlds.SQLDatasource promotes every public method (QueryData,
// CheckHealth, Dispose, GetDBFromQuery, the Interpolator field, …) so call
// sites continue to use the upstream surface.
//
// CheckHealth uses the bootstrap connection (cached under "<uid>-default"),
// which means OAuth-only deployments report degraded health unless their
// bootstrap credentials are valid. Per-user health is out of scope; addressing
// it requires an upstream hook that does not exist today.
type HdxSqlDatasource struct {
	*sqlds.SQLDatasource
}

// NewHdxSqlDatasource constructs the wrapper. settings.UID is captured by the
// ConnectionCacheFactory closure so the cache can recognise the bootstrap
// key (`<uid>-default`) and exempt it from TTL eviction. The factory runs
// per call to sqlds.NewDatasource, so each (re)configuration of the
// instance gets a fresh per-instance cache.
func NewHdxSqlDatasource(driver sqlds.Driver, settings backend.DataSourceInstanceSettings) *HdxSqlDatasource {
	ds := sqlds.NewDatasource(driver)
	ds.EnableMultipleConnections = true
	ds.Interpolator = NewHdxInterpolator(NewMetadataProvider(), Macros)
	ds.ConnectionCacheFactory = func() sqlds.ConnectionCache {
		return NewTTLConnectionCache(settings.UID, connectionCacheTTL)
	}
	return &HdxSqlDatasource{SQLDatasource: ds}
}

// NewDatasource overrides the embedded SQLDatasource.NewDatasource so the
// per-instance instancemgmt.Instance is the *HdxSqlDatasource wrapper, not
// the inner *sqlds.SQLDatasource. The Grafana SDK stores whatever this
// returns, so the wrapper has to be what callers receive when they fetch
// the instance back from instancemgmt.
func (ds *HdxSqlDatasource) NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	if _, err := ds.SQLDatasource.NewDatasource(ctx, settings); err != nil {
		return nil, err
	}
	return ds, nil
}
