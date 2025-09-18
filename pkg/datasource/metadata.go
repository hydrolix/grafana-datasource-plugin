package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/hydrolix/plugin/pkg/models"
	"github.com/jellydator/ttlcache/v3"
	"strings"
	"time"
)

var (
	PRIMARY_KEY_QUERY_STRING    = "SELECT primary_key FROM system.tables WHERE database='%s' AND table ='%s'"
	AD_HOC_KEY_QUERY            = "DESCRIBE %s"
	PRIMARY_KEY_NOT_FOUND_ERROR = backend.PluginError(errors.New("primary key not found"))
	KEYS_NOT_FOUND_ERROR        = backend.PluginError(errors.New("adHocFilter keys not found"))
)

type MetaDataProvider struct {
	ds       *HydrolixDatasource
	pkCache  *ttlcache.Cache[string, string]
	keyCache *ttlcache.Cache[string, map[string]string]
}

func NewMetaDataProvider(ds *HydrolixDatasource) *MetaDataProvider {
	pkCache := ttlcache.New[string, string](ttlcache.WithTTL[string, string](time.Hour))
	keyCache := ttlcache.New[string, map[string]string](ttlcache.WithTTL[string, map[string]string](time.Hour))
	return &MetaDataProvider{ds: ds, pkCache: pkCache, keyCache: keyCache}
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

	entry := p.pkCache.Get(cacheKey)
	if entry == nil {
		fmt.Println("Cache miss for:", cacheKey)
		pk, err := p.QueryPK(database, table)
		if err != nil {
			return "", err
		}
		p.pkCache.Set(cacheKey, pk, ttlcache.DefaultTTL)

		return pk, nil
	} else {
		fmt.Println("Cache hit for:", cacheKey)
		return entry.Value(), nil
	}

}

func (p *MetaDataProvider) GetKeys(cte string) (map[string]string, error) {
	cacheKey := cte

	entry := p.keyCache.Get(cacheKey)
	if entry == nil {
		fmt.Println("Cache miss for:", cacheKey)
		keys, err := p.QueryKeys(cte)
		if err != nil {
			return nil, err
		}
		p.keyCache.Set(cacheKey, keys, ttlcache.DefaultTTL)

		return keys, nil
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

// executeQuery executes a SQL query using the QueryData method and returns the resulting frame
func (p *MetaDataProvider) executeQuery(sql string, queryID string) (*data.Frame, error) {
	// Create a query using QueryData method
	queryJSON, err := json.Marshal(map[string]interface{}{
		"rawSql": sql,
		"format": 1,
	})
	if err != nil {
		return nil, err
	}

	dataQuery := backend.DataQuery{
		RefID: queryID,
		JSON:  queryJSON,
	}

	settings := p.ds.Connector.getInstanceSettings()
	req := &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &settings,
		},
		Queries: []backend.DataQuery{dataQuery},
	}

	// Execute the query using QueryData
	response, err := p.ds.QueryData(context.Background(), req)
	if err != nil {
		return nil, err
	}

	// Check for errors in the response
	dataResponse, ok := response.Responses[dataQuery.RefID]
	if !ok {
		return nil, fmt.Errorf("no response for query %s", queryID)
	}
	if dataResponse.Error != nil {
		return nil, dataResponse.Error
	}

	// Extract the frame from the response
	if len(dataResponse.Frames) == 0 {
		return nil, fmt.Errorf("no frames in response")
	}

	return dataResponse.Frames[0], nil
}

func (p *MetaDataProvider) QueryPK(database string, table string) (string, error) {
	// Format the SQL query with actual parameter values
	formattedSQL := fmt.Sprintf(PRIMARY_KEY_QUERY_STRING, database, table)

	frame, err := p.executeQuery(formattedSQL, "pk_query")
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

func (p *MetaDataProvider) QueryKeys(cte string) (map[string]string, error) {
	if strings.Contains(strings.ToUpper(cte), "SELECT") {
		cte = fmt.Sprintf("(%s)", cte)
	}
	formattedSQL := fmt.Sprintf(AD_HOC_KEY_QUERY, cte)

	frame, err := p.executeQuery(formattedSQL, "key_query")
	if err != nil {
		return nil, err
	}
	if len(frame.Fields) < 2 {
		return nil, KEYS_NOT_FOUND_ERROR
	}
	keyFiled := frame.Fields[0]
	typeFiled := frame.Fields[1]

	keys := make(map[string]string, keyFiled.Len())

	for i := range keyFiled.Len() {
		key, err := p.GetStringSafe(keyFiled.At(i))
		if err != nil {
			return nil, err
		}
		keyType, err := p.GetStringSafe(typeFiled.At(i))
		if err != nil {
			return nil, err
		}
		keys[key] = keyType

	}

	return keys, err
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
