package plugin

import (
	"context"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/hydrolix/plugin/pkg/api"
	"github.com/hydrolix/plugin/pkg/datasource"
)

func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	conn, err := datasource.NewConnector(ctx, NewHydrolix(), settings, true)
	if err != nil {
		return nil, backend.DownstreamError(err)
	}
	ds := &datasource.HydrolixDatasource{
		Connector: conn,
	}
	routes, err := api.Routes(ds, ctx, settings)
	ds.RegisterRoutes(routes)
	if err != nil {
		return nil, err
	}
	newDatasource, err := ds.NewDatasource(ctx, settings)
	return newDatasource, err
}
