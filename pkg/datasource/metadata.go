package datasource

import (
	"context"
	"errors"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/hydrolix/plugin/pkg/models"
	"github.com/jellydator/ttlcache/v3"
	"time"
)

var (
	PRIMARY_KEY_QUERY_STRING    = "SELECT primary_key FROM system.tables WHERE database=? AND table =?"
	PRIMARY_KEY_NOT_FOUND_ERROR = backend.PluginError(errors.New("primary key not found"))
)

type MetaDataProvider struct {
	ds    *HydrolixDatasource
	cache *ttlcache.Cache[string, string]
}

func NewMetaDataProvider(ds *HydrolixDatasource) *MetaDataProvider {
	cache := ttlcache.New[string, string](ttlcache.WithTTL[string, string](time.Hour))
	return &MetaDataProvider{ds: ds, cache: cache}
}

func (p *MetaDataProvider) GetPK(context context.Context, database string, table string) (string, error) {

	if database == "" {
		defaultDB, err := p.getDefaultDatabase(context)
		if err != nil {
			return "", err
		}
		database = defaultDB
	}

	cacheKey := fmt.Sprintf("%s_%s", database, table)

	entry := p.cache.Get(cacheKey)
	if entry == nil {
		fmt.Println("Cache miss for:", cacheKey)
		pk, err := p.QueryPK(database, table)
		if err != nil {
			return "", err
		}
		p.cache.Set(cacheKey, pk, ttlcache.DefaultTTL)

		return pk, nil
	} else {
		fmt.Println("Cache hit for:", cacheKey)
		return entry.Value(), nil
	}

}
func (p *MetaDataProvider) getDefaultDatabase(context context.Context) (string, error) {
	settings, err := models.NewPluginSettings(context, p.ds.Connector.getInstanceSettings())
	if err != nil {
		return "", err
	}
	return settings.DefaultDatabase, nil
}

func (p *MetaDataProvider) QueryPK(database string, table string) (string, error) {

	conn, ok := p.ds.Connector.getDBConnection(defaultKey(p.ds.Connector.GetUID()))
	if !ok {
		return "", PRIMARY_KEY_NOT_FOUND_ERROR
	}

	rows, err := conn.db.Query(PRIMARY_KEY_QUERY_STRING, database, table)
	if err != nil {
		return "", err
	}
	frame, err := sqlutil.FrameFromRows(rows, 1, converters.Converters...)
	if err != nil {
		return "", err
	}

	if len(frame.Fields) == 0 {
		return "", PRIMARY_KEY_NOT_FOUND_ERROR
	}
	field := frame.Fields[0]

	if field.Len() == 0 {
		return "", PRIMARY_KEY_NOT_FOUND_ERROR
	}
	v, err := p.GetStringSafe(field.At(0))

	return v, err
}

func (p *MetaDataProvider) GetStringSafe(v any) (string, error) {

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
