package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/hydrolix/plugin/pkg/api"
)

// NewDatasource is the instance-factory the Grafana SDK calls per
// DataSourceInstanceSettings. It constructs the HdxSqlDatasource wrapper,
// hands the plugin's HTTP routes to sqlds via CustomRoutes, and delegates
// the per-instance bootstrap to the embedded *sqlds.SQLDatasource.
//
// CustomRoutes must be set BEFORE ds.NewDatasource: sqlds's NewDatasource
// builds its own mux (with /tables, /schemas, /columns + everything in
// CustomRoutes) and overwrites CallResourceHandler. Setting routes via
// CustomRoutes is the only way the plugin's /interpolate + /macroCTE
// handlers survive that overwrite.
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	ds := NewHdxSqlDatasource(NewHydrolix(), settings)
	ds.SQLDatasource.CustomRoutes = api.Routes(ds.SQLDatasource)
	return ds.NewDatasource(ctx, settings)
}
