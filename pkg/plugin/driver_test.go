package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/plugin/pkg/models"
	"github.com/stretchr/testify/assert"
	"slices"
	"strings"
	"testing"
	"time"
)

// testValueCtx mock for context
type testValueCtx struct {
	context.Context
	Key, Val any
}

func (c *testValueCtx) Value(key any) any {
	if c.Key == key {
		return c.Val
	}
	return c.Context.Value(key)
}

func testContextHandler(ctx context.Context, settings map[string]any) context.Context {
	return &testValueCtx{Context: ctx, Key: "querySettings", Val: settings}
}

// TestMutateDataQuery verify allowed query options are properly merged and set into context
func TestQueryCustomSettingsPropagation(t *testing.T) {

	for _, protocol := range []string{"http", "native"} {

		plugin := &Hydrolix{querySettingsContextHandler: testContextHandler}

		querySettings := map[string]string{
			"hdx_query_max_rows":                  "100",
			"hdx_query_max_attempts":              "2",
			"hdx_query_max_result_bytes":          "10000",
			"hdx_query_max_result_rows":           "10",
			"hdx_query_max_timerange_sec":         "60",
			"hdx_query_timerange_required":        "1",
			"hdx_query_max_partitions":            "1000",
			"hdx_query_max_peers":                 "100",
			"hdx_query_pool_name":                 "test pool",
			"hdx_query_max_concurrent_partitions": "10",
			"hdx_http_proxy_enabled":              "1",
			"hdx_http_proxy_ttl":                  "15",
			"hdx_invalid":                         "10",
			"hdx_query_admin_comment":             "db=dashboard; dp=Hits per status code; du=testuser",
		}
		dsQuerySettings := []models.QuerySetting{}

		for k, v := range querySettings {
			dsQuerySettings = append(dsQuerySettings, models.QuerySetting{Value: v, Setting: k})
		}

		settings := models.PluginSettings{
			Host:            "localhost",
			Port:            80,
			Protocol:        protocol,
			UserName:        "default",
			Password:        "pass",
			Secure:          true,
			Path:            "/query",
			SkipTlsVerify:   true,
			DialTimeout:     "10",
			QueryTimeout:    "20",
			DefaultDatabase: "dbdb",
			QuerySettings:   dsQuerySettings,
			Other:           nil,
		}
		jsonData, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}

		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
					Name:                    "test-hydrolix-http-datasource",
					JSONData:                jsonData,
					DecryptedSecureJSONData: map[string]string{"password": settings.Password},
				},
			},
			Queries: []backend.DataQuery{
				{
					RefID: "A", QueryType: "", MaxDataPoints: 400, Interval: time.Hour,
					TimeRange: backend.TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
					JSON: []byte(`{
					"rawSql": "SELECT version()",
					"refId":  "0.538154071285475",
					"meta":   {"timezone": "Asia/Singapore"},
					"querySettings": {
						"hdx_query_timerange_required": "0",
						"hdx_query_max_result_rows":    "20"
					},
					"datasource":   {"type": "test-datasource", "uid": "a6835544-2bfe-4f3a-98da-524301ae2280"},
					"datasourceId": 1
				}`),
				},
			},
		}

		var dataQuery struct {
			RawSql        string         `json:"rawSql,omitempty"`
			QuerySettings map[string]any `json:"querySettings,omitempty"`
		}

		ctx, req := plugin.MutateQueryData(context.Background(), req)
		ctx0, qr0 := plugin.MutateQuery(ctx, req.Queries[0])

		t.Run(strings.ToUpper(protocol)+" MutateDataQuery JSON & Context data", func(t *testing.T) {

			if err := json.Unmarshal(req.Queries[0].JSON, &dataQuery); err != nil {
				t.Fatal("Query Settings unmarshal error:", err)
			}

			actualSettings := dataQuery.QuerySettings
			assert.Len(t, actualSettings, len(dsQuerySettings))

			// custom settings set on query level
			assert.Equal(t, "0", actualSettings["hdx_query_timerange_required"])
			assert.Equal(t, "20", actualSettings["hdx_query_max_result_rows"])

			for _, v := range dsQuerySettings {
				if !slices.Contains([]string{"hdx_query_timerange_required", "hdx_query_max_result_rows"}, v.Setting) {
					assert.EqualValues(t, v.Value, actualSettings[v.Setting])
				}
			}
			actualSettings = ctx.Value("querySettings").(map[string]any)
			assert.Len(t, actualSettings, len(dsQuerySettings))
			for _, v := range dsQuerySettings {
				assert.EqualValues(t, v.Value, actualSettings[v.Setting])
			}

			assert.NotContains(t, strings.ToLower(dataQuery.RawSql), " querySettings ")

		})

		t.Run(strings.ToUpper(protocol)+" MutateQuery JSON & Context", func(t *testing.T) {

			if err := json.Unmarshal(qr0.JSON, &dataQuery); err != nil {
				t.Fatal("Query Settings unmarshal error:", err)
			}

			actualSettings := dataQuery.QuerySettings
			assert.Len(t, actualSettings, len(dsQuerySettings))

			// custom settings set on query level
			assert.Equal(t, "0", actualSettings["hdx_query_timerange_required"])
			assert.Equal(t, "20", actualSettings["hdx_query_max_result_rows"])

			for _, v := range dsQuerySettings {
				if !slices.Contains([]string{"hdx_query_timerange_required", "hdx_query_max_result_rows"}, v.Setting) {
					assert.EqualValues(t, v.Value, actualSettings[v.Setting])
				}
			}

			actualSettings = ctx0.Value("querySettings").(map[string]any)
			assert.Len(t, actualSettings, len(dataQuery.QuerySettings))

			for k, v := range dataQuery.QuerySettings {
				assert.EqualValues(t, clickhouse.CustomSetting{Value: fmt.Sprintf("%v", v)}, actualSettings[k])
			}

			assert.NotContains(t, strings.ToLower(dataQuery.RawSql), " querySettings ")

		})
	}
}
