// Package build provides Hydrolix plugin's build information
package build

import (
	"github.com/grafana/grafana-plugin-sdk-go/build"
)

// DefaultBuilInfo is a Default build information
var DefaultBuilInfo = build.Info{
	Time:     0,
	PluginID: "hydrolix-plugin-datasource",
	Version:  "",
}

// BuildInfo is a provider of a build information
type BuildInfo struct {
	buildInfoProvider build.InfoGetterFunc
}

// GetBuildInfo retrieves Grafana plugin's build information (time, plugin id, version)
func (p BuildInfo) GetBuildInfo() build.Info {
	var provider = p.buildInfoProvider
	if provider == nil {
		provider = build.GetBuildInfo
	}
	if info, err := provider(); err == nil && info.PluginID != "" {
		return info
	} else {
		return DefaultBuilInfo
	}
}
