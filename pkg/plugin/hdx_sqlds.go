package plugin

import (
	"context"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/plugin/pkg/plugin/models"
)

// connectionCacheTTL is the per-entry TTL for the per-user *sql.DB cache.
// Matches the fork's hardcoded one-hour choice; see C3's design D7.
const connectionCacheTTL = time.Hour

// HdxSqlDatasource is the Hydrolix plugin's wrapper around sqlds.SQLDatasource.
// It centralises extension-point wiring (Interpolator, ConnectionCacheFactory,
// MetadataProvider) and per-instance configuration in one constructor.
//
// The embedded *sqlds.SQLDatasource promotes every public method (QueryData,
// CheckHealth, Dispose, GetDBFromQuery, the Interpolator field, …) so call
// sites continue to use the upstream surface.
//
// Settings holds the parsed plugin settings (parsed once at construction).
// MetadataProvider reads Settings.DefaultDatabase via DefaultDatabase(); a
// nil Settings indicates parse failure (kept as a foreground error rather
// than crashing instance construction).
//
// CheckHealth uses the bootstrap connection (cached under "<uid>-default"),
// which means OAuth-only deployments report degraded health unless their
// bootstrap credentials are valid. Per-user health is out of scope; addressing
// it requires an upstream hook that does not exist today.
type HdxSqlDatasource struct {
	*sqlds.SQLDatasource
	// Settings is the parsed plugin settings (nil on parse failure).
	Settings *models.PluginSettings
	// instanceSettings holds the original DataSourceInstanceSettings used
	// to synthesise schema-query requests in MetadataProvider.executeQuery.
	instanceSettings backend.DataSourceInstanceSettings
}

// NewHdxSqlDatasource constructs the wrapper. settings.UID is captured by the
// ConnectionCacheFactory closure so the cache can recognise the bootstrap
// key (`<uid>-default`) and exempt it from TTL eviction. The factory runs
// per call to sqlds.NewDatasource, so each (re)configuration of the
// instance gets a fresh per-instance cache.
//
// settings.JSONData is parsed once into models.PluginSettings; parse failure
// leaves wrapper.Settings == nil so MetadataProvider's DefaultDatabase()
// returns "" and downstream callers surface a clear error rather than
// crashing on a nil deref.
func NewHdxSqlDatasource(driver sqlds.Driver, settings backend.DataSourceInstanceSettings) *HdxSqlDatasource {
	var parsed *models.PluginSettings
	if p, err := models.NewPluginSettings(context.Background(), settings); err != nil {
		log.DefaultLogger.Warn("failed to parse plugin settings; metadata lookups will fail", "err", err)
	} else {
		parsed = &p
	}

	ds := sqlds.NewDatasource(driver)
	ds.EnableMultipleConnections = true
	ds.ConnectionCacheFactory = func() sqlds.ConnectionCache {
		return NewTTLConnectionCache(settings.UID, connectionCacheTTL)
	}

	wrapper := &HdxSqlDatasource{
		SQLDatasource:    ds,
		Settings:         parsed,
		instanceSettings: settings,
	}
	// MetadataProvider closes over the wrapper (for QueryData routing and
	// DefaultDatabase access); the interpolator references the provider.
	// Order matters: the wrapper must exist before NewMetadataProvider.
	// ds.Interpolator is a func field; we install the method value, which
	// overrides the default sqlds.NewDatasource wires in.
	ds.Interpolator = NewHdxInterpolator(NewMetadataProvider(wrapper), Macros).Interpolate
	return wrapper
}

// InstanceSettings returns the original DataSourceInstanceSettings captured
// at construction. Satisfies metadataDS so MetadataProvider can build the
// PluginContext for synthetic schema-query requests.
func (ds *HdxSqlDatasource) InstanceSettings() backend.DataSourceInstanceSettings {
	return ds.instanceSettings
}

// DefaultDatabase returns the configured default database, or "" when the
// plugin settings failed to parse. Returning "" rather than panicking keeps
// the error path explicit: callers detect the missing default and surface
// a useful error to the operator.
func (ds *HdxSqlDatasource) DefaultDatabase() string {
	if ds.Settings == nil {
		return ""
	}
	return ds.Settings.DefaultDatabase
}

// NewDatasource overrides the embedded SQLDatasource.NewDatasource so the
// per-instance instancemgmt.Instance is the *HdxSqlDatasource wrapper, not
// the inner *sqlds.SQLDatasource. The Grafana SDK stores whatever this
// returns, so the wrapper has to be what callers receive when they fetch
// the instance back from instancemgmt.
func (ds *HdxSqlDatasource) NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	if _, err := ds.SQLDatasource.NewDatasource(ctx, settings); err != nil {
		return nil, err
	}
	return ds, nil
}
