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

// NewHdxSqlDatasource constructs the wrapper. The Interpolator and
// ConnectionCacheFactory slots are populated by subsequent changes (C5, C3)
// that bring those extension implementations into the plugin.
func NewHdxSqlDatasource(driver sqlds.Driver) *HdxSqlDatasource {
	ds := sqlds.NewDatasource(driver)
	ds.EnableMultipleConnections = true
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
