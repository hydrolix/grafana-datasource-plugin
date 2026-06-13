package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v5"
)

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

// NewHdxSqlDatasource constructs the wrapper. The ConnectionCacheFactory
// slot is populated by C3 (plugin-ttl-connection-cache); the Interpolator
// is populated here (C5) with the plugin-local HdxInterpolator backed by
// the package-level Macros registry.
func NewHdxSqlDatasource(driver sqlds.Driver) *HdxSqlDatasource {
	ds := sqlds.NewDatasource(driver)
	ds.EnableMultipleConnections = true
	ds.Interpolator = NewHdxInterpolator(NewMetadataProvider(), Macros)
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
