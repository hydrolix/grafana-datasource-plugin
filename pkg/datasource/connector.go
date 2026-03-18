package datasource

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
	"github.com/hydrolix/plugin/pkg/models"
	"github.com/jellydator/ttlcache/v3"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type Connector interface {
	Connect(ctx context.Context, headers http.Header) (*dbConnection, error)
	connectWithRetries(ctx context.Context, conn dbConnection, key string, headers http.Header) error
	connect(conn dbConnection) error
	ping(conn dbConnection) error
	Reconnect(ctx context.Context, dbConn dbConnection, q *sqlutil.Query, cacheKey string) (*sql.DB, error)
	getDBConnection(key string) (dbConnection, bool)
	storeDBConnection(key string, dbConn dbConnection)
	Dispose()
	GetConnectionFromQuery(ctx context.Context, q *sqlutil.Query) (string, dbConnection, error)
	GetDriver() sqlds.Driver
	GetUID() string
	getDriverSettings() sqlds.DriverSettings
	getInstanceSettings() backend.DataSourceInstanceSettings
}

type HydrolixConnector struct {
	UID              string
	connections      *ttlcache.Cache[string, dbConnection]
	Driver           sqlds.Driver
	driverSettings   sqlds.DriverSettings
	instanceSettings backend.DataSourceInstanceSettings
	pluginSettings   models.PluginSettings
}

func NewConnector(ctx context.Context, driver sqlds.Driver, settings backend.DataSourceInstanceSettings) (Connector, error) {
	pluginSettings, err := models.NewPluginSettings(ctx, settings)
	if err != nil {
		return nil, backend.DownstreamError(err)
	}
	ds := driver.Settings(ctx, settings)
	connections := *ttlcache.New[string, dbConnection](ttlcache.WithTTL[string, dbConnection](time.Hour))
	connections.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, i *ttlcache.Item[string, dbConnection]) {
		_ = i.Value().db.Close()
	})

	conn := &HydrolixConnector{
		UID:              settings.UID,
		Driver:           driver,
		driverSettings:   ds,
		instanceSettings: settings,
		pluginSettings:   pluginSettings,
		connections:      &connections,
	}
	if pluginSettings.CredentialsType != "forwardOAuth" {
		key := defaultKey(settings.UID)
		db, err := driver.Connect(ctx, settings, nil)
		if err != nil {
			return nil, backend.DownstreamError(err)
		}
		conn.storeDBConnection(key, dbConnection{db, settings})
	}

	return conn, nil
}

func (c *HydrolixConnector) Connect(ctx context.Context, headers http.Header) (*dbConnection, error) {
	key := ""
	if c.pluginSettings.CredentialsType == "forwardOAuth" {
		key = keyWithConnectionArgs(c.UID, getOAuthConnectionArgs(headers.Get(backend.OAuthIdentityTokenHeaderName)))
	} else {
		key = defaultKey(c.UID)
	}
	dbConn, ok := c.getDBConnection(key)
	if !ok {
		db, err := c.Driver.Connect(ctx, c.instanceSettings, getOAuthConnectionArgs(headers.Get(backend.OAuthIdentityTokenHeaderName)))
		if err != nil {
			return nil, err
		}
		// Assign this connection in the cache
		dbConn = dbConnection{db, c.instanceSettings}
		c.storeDBConnection(key, dbConn)
	}

	if c.driverSettings.Retries == 0 {
		err := c.connect(dbConn)
		return &dbConn, err
	}

	err := c.connectWithRetries(ctx, dbConn, key, headers)
	return &dbConn, err
}

func (c *HydrolixConnector) connectWithRetries(ctx context.Context, conn dbConnection, key string, headers http.Header) error {
	q := &sqlutil.Query{}
	if c.driverSettings.ForwardHeaders {
		applyHeaders(q, headers)
	}

	var db *sql.DB
	var err error
	for i := 0; i < c.driverSettings.Retries; i++ {
		db, err = c.Reconnect(ctx, conn, q, key)
		if err != nil {
			return err
		}
		conn := dbConnection{
			db:       db,
			settings: conn.settings,
		}
		err = c.connect(conn)
		if err == nil {
			break
		}

		if !shouldRetry(c.driverSettings.RetryOn, err.Error()) {
			break
		}

		if i+1 == c.driverSettings.Retries {
			break
		}

		if c.driverSettings.Pause > 0 {
			time.Sleep(time.Duration(c.driverSettings.Pause * int(time.Second)))
		}
		backend.Logger.Warn("connect failed", "error", err.Error(), "retry", i+1)
	}

	return err
}

func applyHeaders(query *sqlutil.Query, headers http.Header) *sqlutil.Query {
	var args map[string]interface{}
	if query.ConnectionArgs == nil {
		query.ConnectionArgs = []byte("{}")
	}
	err := json.Unmarshal(query.ConnectionArgs, &args)
	if err != nil {
		backend.Logger.Warn("Failed to apply headers", "error", err.Error())
		return query
	}
	args[HeaderKey] = headers
	raw, err := json.Marshal(args)
	if err != nil {
		backend.Logger.Warn("Failed to apply headers", "error", err.Error())
		return query
	}

	query.ConnectionArgs = raw

	return query
}

func (c *HydrolixConnector) connect(conn dbConnection) error {
	if err := c.ping(conn); err != nil {
		return backend.DownstreamError(err)
	}

	return nil
}

func (c *HydrolixConnector) ping(conn dbConnection) error {

	return conn.db.Ping()
}

func (c *HydrolixConnector) Reconnect(ctx context.Context, dbConn dbConnection, q *sqlutil.Query, cacheKey string) (*sql.DB, error) {
	if err := dbConn.db.Close(); err != nil {
		backend.Logger.Warn("closing existing connection failed", "error", err.Error())
	}

	db, err := c.Driver.Connect(ctx, dbConn.settings, q.ConnectionArgs)
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, backend.DownstreamError(err)
	}
	c.storeDBConnection(cacheKey, dbConnection{db, dbConn.settings})
	return db, nil
}

func (c *HydrolixConnector) getDBConnection(key string) (dbConnection, bool) {
	conn := c.connections.Get(key)
	if conn == nil {
		return dbConnection{}, false
	}
	return conn.Value(), true
}

func (c *HydrolixConnector) storeDBConnection(key string, dbConn dbConnection) {
	c.connections.Set(key, dbConn, ttlcache.DefaultTTL)
}

// Dispose is called when an existing SQLDatasource needs to be replaced
func (c *HydrolixConnector) Dispose() {
	c.connections.DeleteAll()
}

func (c *HydrolixConnector) getDriverSettings() sqlds.DriverSettings {
	return c.driverSettings
}

func (c *HydrolixConnector) GetDriver() sqlds.Driver {
	return c.Driver
}
func (c *HydrolixConnector) GetUID() string {
	return c.UID
}
func (c *HydrolixConnector) getInstanceSettings() backend.DataSourceInstanceSettings {
	return c.instanceSettings
}

func (c *HydrolixConnector) GetConnectionFromQuery(ctx context.Context, q *sqlutil.Query) (string, dbConnection, error) {

	// The database connection may vary depending on query arguments
	// The raw arguments are used as key to store the db connection in memory so they can be reused
	key := defaultKey(c.UID)
	dbConn, _ := c.getDBConnection(key)

	if len(q.ConnectionArgs) == 0 {
		return key, dbConn, nil
	}

	key = keyWithConnectionArgs(c.UID, q.ConnectionArgs)
	if cachedConn, ok := c.getDBConnection(key); ok {
		return key, cachedConn, nil
	}

	db, err := c.Driver.Connect(ctx, c.instanceSettings, q.ConnectionArgs)
	if err != nil {
		return "", dbConnection{}, backend.DownstreamError(err)
	}
	// Assign this connection in the cache
	dbConn = dbConnection{db, c.instanceSettings}
	c.storeDBConnection(key, dbConn)

	return key, dbConn, nil
}

func getOAuthConnectionArgs(header string) json.RawMessage {
	q := &sqlutil.Query{}
	headers := http.Header{}
	headers.Set(backend.OAuthIdentityTokenHeaderName, header)
	applyHeaders(q, headers)
	return q.ConnectionArgs
}

func shouldRetry(retryOn []string, err string) bool {
	for _, r := range retryOn {
		if strings.Contains(err, r) {
			return true
		}
	}
	return false
}
