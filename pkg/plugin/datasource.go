package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/hydrolix/plugin/pkg/api"
	"github.com/hydrolix/sqlds/v5"
)

func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	conn, err := sqlds.NewConnector(ctx, NewHydrolix(), settings)
	if err != nil {
		return nil, backend.DownstreamError(err)
	}
	ds := &sqlds.HydrolixDatasource{
		Connector: conn,
	}
	ds.RegisterRoutes(api.Routes(ds))
	newDatasource, err := ds.NewDatasource(ctx, settings)
	return newDatasource, err
}
