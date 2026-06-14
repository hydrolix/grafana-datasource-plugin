package plugin

import (
	"context"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/hydrolix/plugin/pkg/api"
)

// NewDatasource is the instance-factory the Grafana SDK calls per
// DataSourceInstanceSettings. It constructs the HdxSqlDatasource wrapper,
// wires HTTP routes onto the upstream CallResourceHandler, and delegates
// the per-instance bootstrap to the embedded *sqlds.SQLDatasource.
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	ds := NewHdxSqlDatasource(NewHydrolix(), settings)
	registerRoutes(ds, api.Routes(ds.SQLDatasource))
	return ds.NewDatasource(ctx, settings)
}

// registerRoutes installs the plugin's HTTP routes onto the embedded
// SQLDatasource's CallResourceHandler. Upstream sqlds no longer provides
// the RegisterRoutes method the fork carried; routes attach directly.
func registerRoutes(ds *HdxSqlDatasource, routes map[string]func(http.ResponseWriter, *http.Request)) {
	mux := http.NewServeMux()
	for route, handler := range routes {
		mux.HandleFunc(route, handler)
	}
	ds.CallResourceHandler = httpadapter.New(mux)
}
