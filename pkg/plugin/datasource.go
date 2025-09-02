package plugin

import (
	"context"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/hydrolix/plugin/pkg/api"
	"github.com/hydrolix/plugin/pkg/datasource"
)

func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	ds := &datasource.HydrolixDatasource{
		Connector: &datasource.HydrolixConnector{Driver: NewHydrolix()},
	}
	ds.EnableMultipleConnections = true
	ds.RegisterRoutes(api.Routes(ds))
	return ds.NewDatasource(ctx, settings)
}
