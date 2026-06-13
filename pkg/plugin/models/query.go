// Package models defines the Hydrolix plugin's data shapes:
// the per-query JSON (HdxQuery), the ad-hoc filter wire format
// (AdHocFilter), and the datasource configuration (PluginSettings,
// QuerySetting). These are pure data shapes consumed by both
// pkg/api (HTTP layer) and pkg/plugin (datasource runtime).
package models

import (
	"net/http"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// HdxQuery is the per-query shape the plugin reads from
// backend.DataQuery.JSON and sets at request time. JSON-tagged fields
// round-trip with the dashboard's stored query; the remaining fields
// (TimeRange, Interval, Headers) are populated by the plugin at
// request time and are deliberately excluded from JSON serialisation.
type HdxQuery struct {
	RawSQL        string         `json:"rawSql"`
	Format        int            `json:"format"`
	Round         string         `json:"round,omitempty"`
	QuerySettings []QuerySetting `json:"querySettings,omitempty"`
	Filters       []AdHocFilter  `json:"filters,omitempty"`
	Meta          struct {
		TimeZone string `json:"timezone"`
	} `json:"meta"`
	TimeRange backend.TimeRange `json:"-"`
	Interval  time.Duration     `json:"-"`
	Headers   http.Header       `json:"-"`
}

// AdHocFilter is the wire format the Grafana ad-hoc filter UI sends
// to the plugin per filter. Single-value operators read Value; the
// multi-value operators ("=|", "!=|") read Values.
type AdHocFilter struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Value    string   `json:"value"`
	Values   []string `json:"values,omitempty"`
}
