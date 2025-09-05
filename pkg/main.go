// Package main is a root package and provides entrypoint for Hydrolix plugin as well.
package main

import (
	"github.com/hydrolix/plugin/pkg/build"
	"github.com/hydrolix/plugin/pkg/plugin"
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func main() {
	// Start listening to requests sent from Grafana. This call is blocking so
	// it won't finish until Grafana shuts down the process or the plugin choose
	// to exit by itself using os.Exit. Manage automatically manages life cycle
	// of datasource instances. It accepts datasource instance factory as first
	// argument. This factory will be automatically called on incoming request
	// from Grafana to create different instances of SampleDatasource (per datasource
	// ID). When datasource configuration changed Dispose method will be called and
	// new datasource instance created using NewSampleDatasource factory.

	var info = build.BuildInfo{}.GetBuildInfo()
	log.DefaultLogger.Info("Plugin build info", "PluginID", info.Version, "Version", info.Version)

	if err := datasource.Manage(info.PluginID, plugin.NewDatasource, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		os.Exit(1)
	}
}
