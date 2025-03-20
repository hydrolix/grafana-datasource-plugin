package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v4"
)

// Create Hydrolix sql ds
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	hydrolixPlugin := Hydrolix{}
	ds := sqlds.NewDatasource(&hydrolixPlugin)
	ds.EnableMultipleConnections = true // TODO: make it configurable and limit connections pool
	return ds.NewDatasource(ctx, settings)
}
