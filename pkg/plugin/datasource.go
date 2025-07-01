package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v4"
	"github.com/hydrolix/plugin/pkg/api"
)

// NewDatasource creates Hydrolix SQLDS datasource
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	ds := sqlds.NewDatasource(NewHydrolix())
	ds.EnableMultipleConnections = true
	ds.CustomRoutes = api.Routes()
	return ds.NewDatasource(ctx, settings)
}
