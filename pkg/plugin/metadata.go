package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/cte"
	"github.com/jellydator/ttlcache/v3"
)

const (
	// PrimaryKeyQuery looks up the primary key for a (database, table) pair.
	PrimaryKeyQuery = "SELECT primary_key FROM system.tables WHERE database='%s' AND table ='%s'"
	// AdHocKeyQuery describes the column types of a CTE / table reference
	// used by the ad-hoc filter macro to dispatch on column type.
	AdHocKeyQuery = "DESCRIBE %s"
)

var (
	// ErrPrimaryKeyNotFound is returned by QueryPK when the schema query
	// yields no rows for the requested (database, table).
	ErrPrimaryKeyNotFound = backend.PluginError(errors.New("primary key not found"))
	// ErrAdHocKeysNotFound is returned by QueryKeys when DESCRIBE yields
	// fewer than two columns (name + type) — the ad-hoc filter macro
	// needs both to operate.
	ErrAdHocKeysNotFound = backend.PluginError(errors.New("adHocFilter keys not found"))
)

// metadataDS is the narrow surface MetadataProvider needs from the wrapper.
// It mirrors the methods used to issue schema queries against the upstream
// Hydrolix cluster. *HdxSqlDatasource satisfies it; tests pass fakes.
type metadataDS interface {
	QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error)
	InstanceSettings() backend.DataSourceInstanceSettings
	DefaultDatabase() string
}

// MetadataProvider caches per-(database, table) primary-key lookups and
// per-CTE column-type maps. Both caches use ttlcache with a 1-hour TTL —
// matches the connection cache (C3) and the fork's hardcoded choice.
//
// Schema queries are issued through ds.QueryData(...), so they participate
// in OAuth-keyed pooling (C4) and the TTL connection cache (C3) identically
// to user-driven panel queries.
type MetadataProvider struct {
	ds       metadataDS
	pkCache  *ttlcache.Cache[string, string]
	keyCache *ttlcache.Cache[string, map[string]string]
}

// NewMetadataProvider builds a MetadataProvider with both caches initialised
// and their sweep goroutines started.
func NewMetadataProvider(ds metadataDS) *MetadataProvider {
	pkCache := ttlcache.New[string, string](
		ttlcache.WithTTL[string, string](time.Hour),
	)
	keyCache := ttlcache.New[string, map[string]string](
		ttlcache.WithTTL[string, map[string]string](time.Hour),
	)
	go pkCache.Start()
	go keyCache.Start()
	return &MetadataProvider{ds: ds, pkCache: pkCache, keyCache: keyCache}
}

// GetPK returns the primary-key column name for (database, table). If
// database is empty, the wrapper's configured default database is used.
// Cache miss issues a schema query; subsequent calls within the TTL hit
// the cache.
func (p *MetadataProvider) GetPK(ctx context.Context, headers http.Header, database, table string) (string, error) {
	if database == "" {
		defaultDB, err := p.getDefaultDatabase()
		if err != nil {
			return "", err
		}
		database = defaultDB
	}
	cacheKey := database + "_" + table

	if entry := p.pkCache.Get(cacheKey); entry != nil {
		log.DefaultLogger.Debug("MetadataProvider: PK cache hit", "key", cacheKey)
		return entry.Value(), nil
	}

	log.DefaultLogger.Debug("MetadataProvider: PK cache miss", "key", cacheKey)
	pk, err := p.QueryPK(ctx, headers, database, table)
	if err != nil {
		return "", err
	}
	p.pkCache.Set(cacheKey, pk, ttlcache.DefaultTTL)
	return pk, nil
}

// GetKeys returns the column-name → column-type map for the given CTE or
// table reference. Cache miss issues a DESCRIBE; subsequent calls within
// the TTL hit the cache.
func (p *MetadataProvider) GetKeys(ctx context.Context, headers http.Header, cte string) (map[string]string, error) {
	if entry := p.keyCache.Get(cte); entry != nil {
		log.DefaultLogger.Debug("MetadataProvider: keys cache hit", "key", cte)
		return entry.Value(), nil
	}

	log.DefaultLogger.Debug("MetadataProvider: keys cache miss", "key", cte)
	keys, err := p.QueryKeys(ctx, headers, cte)
	if err != nil {
		return nil, err
	}
	p.keyCache.Set(cte, keys, ttlcache.DefaultTTL)
	return keys, nil
}

// QueryPK issues the primary-key lookup SQL and returns the first cell of
// the first column. Empty result → ErrPrimaryKeyNotFound.
func (p *MetadataProvider) QueryPK(ctx context.Context, headers http.Header, database, table string) (string, error) {
	sql := fmt.Sprintf(PrimaryKeyQuery, database, table)
	frame, err := p.executeQuery(ctx, headers, sql, "pk_query")
	if err != nil {
		return "", err
	}
	if len(frame.Fields) == 0 || frame.Fields[0].Len() == 0 {
		return "", ErrPrimaryKeyNotFound
	}
	return GetStringSafe(frame.Fields[0].At(0))
}

// QueryKeys issues DESCRIBE <cte> and assembles the column-name → column-type
// map. Sub-SELECTs in the CTE source are wrapped in parentheses to satisfy
// ClickHouse's DESCRIBE grammar.
func (p *MetadataProvider) QueryKeys(ctx context.Context, headers http.Header, cte string) (map[string]string, error) {
	if strings.Contains(strings.ToUpper(cte), "SELECT") {
		cte = "(" + cte + ")"
	}
	sql := fmt.Sprintf(AdHocKeyQuery, cte)
	frame, err := p.executeQuery(ctx, headers, sql, "key_query")
	if err != nil {
		return nil, err
	}
	if len(frame.Fields) < 2 {
		return nil, ErrAdHocKeysNotFound
	}
	nameField := frame.Fields[0]
	typeField := frame.Fields[1]
	keys := make(map[string]string, nameField.Len())
	for i := 0; i < nameField.Len(); i++ {
		name, err := GetStringSafe(nameField.At(i))
		if err != nil {
			return nil, err
		}
		t, err := GetStringSafe(typeField.At(i))
		if err != nil {
			return nil, err
		}
		keys[name] = t
	}
	return keys, nil
}

// executeQuery synthesises a *backend.QueryDataRequest carrying the schema
// SQL and routes it through ds.QueryData. Headers are propagated via
// SetHTTPHeader so non-special headers (notably X-Grafana-Org-Id) survive
// the SDK's getHTTPHeadersFromStringMap round-trip.
func (p *MetadataProvider) executeQuery(ctx context.Context, headers http.Header, sql, queryID string) (*data.Frame, error) {
	queryJSON, err := json.Marshal(map[string]any{
		"rawSql": sql,
		"format": 1,
	})
	if err != nil {
		return nil, err
	}

	settings := p.ds.InstanceSettings()
	req := &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
		Queries: []backend.DataQuery{
			{RefID: queryID, JSON: queryJSON},
		},
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.SetHTTPHeader(k, v)
		}
	}

	resp, err := p.ds.QueryData(ctx, req)
	if err != nil {
		return nil, err
	}
	dataResp, ok := resp.Responses[queryID]
	if !ok {
		return nil, fmt.Errorf("no response for query %s", queryID)
	}
	if dataResp.Error != nil {
		return nil, dataResp.Error
	}
	if len(dataResp.Frames) == 0 {
		return nil, fmt.Errorf("no frames in response")
	}
	return dataResp.Frames[0], nil
}

func (p *MetadataProvider) getDefaultDatabase() (string, error) {
	db := p.ds.DefaultDatabase()
	if db == "" {
		return "", backend.PluginError(errors.New("default database is not configured"))
	}
	return db, nil
}

// GetStringSafe extracts a string from a frame cell. Hydrolix string fields
// arrive as *string (nullable) or string depending on column nullability;
// both are handled.
func GetStringSafe(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case *string:
		if x == nil {
			return "", nil
		}
		return *x, nil
	}
	return "", errors.New("invalid type")
}

// getPK is the macro-facing helper. It parses rawSQL, finds the CTE entry
// at pos, and delegates to MetadataProvider.GetPK. Used by C6's PK-lookup
// macros (`TimeFilter`, `TimeFilterMs`, `TimeInterval`, `TimeIntervalMs`)
// when their column argument is omitted.
func getPK(ctx context.Context, rawSQL string, pos parser.Pos, mdProvider *MetadataProvider, headers http.Header) (string, error) {
	exprs, err := parser.NewParser(rawSQL).ParseStmts()
	if err != nil {
		return rawSQL, err
	}
	macroCTEs, err := cte.GetMacroCTEs(exprs)
	if err != nil {
		return rawSQL, err
	}
	for _, c := range macroCTEs {
		if c.MacroPos == pos {
			return mdProvider.GetPK(ctx, headers, c.Database, c.Table)
		}
	}
	return rawSQL, fmt.Errorf("no CTE found for macro at pos %d", pos)
}
