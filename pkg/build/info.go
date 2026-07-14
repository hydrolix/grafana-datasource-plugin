// Package build provides Hydrolix plugin's build information
package build

import (
	"github.com/grafana/grafana-plugin-sdk-go/build/buildinfo"
)

// DefaultBuilInfo is a Default build information
var DefaultBuilInfo = buildinfo.Info{
	Time:     0,
	PluginID: "hydrolix-hydrolix-datasource",
	Version:  "",
}

// BuildInfo is a provider of a build information
type BuildInfo struct {
	buildInfoProvider buildinfo.GetterFunc
}

// GetBuildInfo retrieves Grafana plugin's build information (time, plugin id, version)
func (p BuildInfo) GetBuildInfo() buildinfo.Info {
	var provider = p.buildInfoProvider
	if provider == nil {
		provider = buildinfo.GetBuildInfo
	}
	if info, err := provider(); err == nil && info.PluginID != "" {
		return info
	} else {
		return DefaultBuilInfo
	}
}
