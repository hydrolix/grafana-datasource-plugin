package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/sqlds/v5/models"
	"github.com/stretchr/testify/assert"
)

func findSettingValue(settings []models.QuerySetting, name string) string {
	for _, s := range settings {
		if s.Setting == name {
			return s.Value
		}
	}
	return ""
}

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
func TestGetHeader(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		jmsg       json.RawMessage
		wantVal    string
		wantOK     bool
	}{
		{
			name:       "nil message returns empty",
			headerName: "Authorization",
			jmsg:       nil,
			wantVal:    "",
			wantOK:     false,
		},
		{
			name:       "empty JSON object returns empty",
			headerName: "Authorization",
			jmsg:       json.RawMessage(`{}`),
			wantVal:    "",
			wantOK:     false,
		},
		{
			name:       "missing header key in message",
			headerName: "Authorization",
			jmsg:       json.RawMessage(`{"other-key": {"Authorization": ["Bearer token"]}}`),
			wantVal:    "",
			wantOK:     false,
		},
		{
			name:       "header present with value",
			headerName: "Authorization",
			jmsg:       json.RawMessage(`{"grafana-http-headers": {"Authorization": ["Bearer my-token"]}}`),
			wantVal:    "Bearer my-token",
			wantOK:     true,
		},
		{
			name:       "header present with multiple values returns first",
			headerName: "Authorization",
			jmsg:       json.RawMessage(`{"grafana-http-headers": {"Authorization": ["Bearer first", "Bearer second"]}}`),
			wantVal:    "Bearer first",
			wantOK:     true,
		},
		{
			name:       "header present with empty array",
			headerName: "Authorization",
			jmsg:       json.RawMessage(`{"grafana-http-headers": {"Authorization": []}}`),
			wantVal:    "",
			wantOK:     false,
		},
		{
			name:       "different header name",
			headerName: OrgIdHeaderKey,
			jmsg:       json.RawMessage(fmt.Sprintf(`{"grafana-http-headers": {"%s": ["42"]}}`, OrgIdHeaderKey)),
			wantVal:    "42",
			wantOK:     true,
		},
		{
			name:       "invalid JSON returns empty",
			headerName: "Authorization",
			jmsg:       json.RawMessage(`not-json`),
			wantVal:    "",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := getHeader(tt.headerName, tt.jmsg)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

func TestGetOAuthToken(t *testing.T) {
	tests := []struct {
		name      string
		jmsg      json.RawMessage
		wantToken string
		wantOK    bool
	}{
		{
			name:      "nil message",
			jmsg:      nil,
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "valid Bearer token",
			jmsg:      json.RawMessage(`{"grafana-http-headers": {"Authorization": ["Bearer abc123"]}}`),
			wantToken: "abc123",
			wantOK:    true,
		},
		{
			name:      "missing Bearer prefix",
			jmsg:      json.RawMessage(`{"grafana-http-headers": {"Authorization": ["abc123"]}}`),
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "empty token value",
			jmsg:      json.RawMessage(`{"grafana-http-headers": {"Authorization": [""]}}`),
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "Bearer with complex JWT token",
			jmsg:      json.RawMessage(`{"grafana-http-headers": {"Authorization": ["Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig"]}}`),
			wantToken: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := getOAuthToken(tt.jmsg)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestGetOrgId(t *testing.T) {
	tests := []struct {
		name    string
		jmsg    json.RawMessage
		wantVal string
		wantOK  bool
	}{
		{
			name:    "nil message",
			jmsg:    nil,
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "org id present",
			jmsg:    json.RawMessage(fmt.Sprintf(`{"grafana-http-headers": {"%s": ["1"]}}`, OrgIdHeaderKey)),
			wantVal: "1",
			wantOK:  true,
		},
		{
			name:    "org id missing",
			jmsg:    json.RawMessage(`{"grafana-http-headers": {"Authorization": ["Bearer token"]}}`),
			wantVal: "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := getOrgId(tt.jmsg)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

func TestSettings_ForwardHeaders(t *testing.T) {
	tests := []struct {
		name            string
		credentialsType string
		wantForward     bool
	}{
		{
			name:            "forwardOAuth enables ForwardHeaders",
			credentialsType: "forwardOAuth",
			wantForward:     true,
		},
		{
			name:            "userAccount disables ForwardHeaders",
			credentialsType: "userAccount",
			wantForward:     false,
		},
		{
			name:            "serviceAccount disables ForwardHeaders",
			credentialsType: "serviceAccount",
			wantForward:     false,
		},
		{
			name:            "empty credentialsType disables ForwardHeaders",
			credentialsType: "",
			wantForward:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHydrolix()
			settings := models.PluginSettings{
				Host:            "localhost",
				Port:            80,
				Protocol:        "http",
				CredentialsType: tt.credentialsType,
				DialTimeout:     "10",
				QueryTimeout:    "20",
			}
			jsonData, err := json.Marshal(settings)
			assert.NoError(t, err)

			config := backend.DataSourceInstanceSettings{
				JSONData:                jsonData,
				DecryptedSecureJSONData: map[string]string{},
			}

			ds := h.Settings(context.Background(), config)
			assert.Equal(t, tt.wantForward, ds.ForwardHeaders)
		})
	}
}

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
					"querySettings": [
						{"setting": "hdx_query_timerange_required", "value": "0"},
						{"setting": "hdx_query_max_result_rows", "value": "20"}
					],
					"datasource":   {"type": "test-datasource", "uid": "a6835544-2bfe-4f3a-98da-524301ae2280"},
					"datasourceId": 1
				}`),
				},
			},
		}

		var dataQuery struct {
			RawSql        string                `json:"rawSql,omitempty"`
			QuerySettings []models.QuerySetting `json:"querySettings,omitempty"`
		}

		ctx, req := plugin.MutateQueryData(context.Background(), req)
		ctx0, qr0 := plugin.MutateQuery(ctx, req.Queries[0])

		t.Run(strings.ToUpper(protocol)+" MutateDataQuery JSON & Context data", func(t *testing.T) {

			if err := json.Unmarshal(req.Queries[0].JSON, &dataQuery); err != nil {
				t.Fatal("Query Settings unmarshal error:", err)
			}

			actualSettings := dataQuery.QuerySettings
			assert.Len(t, actualSettings, len(dsQuerySettings))

			// custom settings set on query level override datasource-level
			assert.Equal(t, "0", findSettingValue(actualSettings, "hdx_query_timerange_required"))
			assert.Equal(t, "20", findSettingValue(actualSettings, "hdx_query_max_result_rows"))

			// datasource-level settings that were not overridden keep original values
			for _, v := range dsQuerySettings {
				if !slices.Contains([]string{"hdx_query_timerange_required", "hdx_query_max_result_rows"}, v.Setting) {
					assert.EqualValues(t, v.Value, findSettingValue(actualSettings, v.Setting))
				}
			}

			assert.NotContains(t, strings.ToLower(dataQuery.RawSql), " querysettings ")

		})

		t.Run(strings.ToUpper(protocol)+" MutateQuery JSON & Context", func(t *testing.T) {

			if err := json.Unmarshal(qr0.JSON, &dataQuery); err != nil {
				t.Fatal("Query Settings unmarshal error:", err)
			}

			actualSettings := dataQuery.QuerySettings
			assert.Len(t, actualSettings, len(dsQuerySettings))

			// custom settings set on query level
			assert.Equal(t, "0", findSettingValue(actualSettings, "hdx_query_timerange_required"))
			assert.Equal(t, "20", findSettingValue(actualSettings, "hdx_query_max_result_rows"))

			for _, v := range dsQuerySettings {
				if !slices.Contains([]string{"hdx_query_timerange_required", "hdx_query_max_result_rows"}, v.Setting) {
					assert.EqualValues(t, v.Value, findSettingValue(actualSettings, v.Setting))
				}
			}

			ctxSettings := ctx0.Value("querySettings").(map[string]any)
			assert.Len(t, ctxSettings, len(dataQuery.QuerySettings))

			for _, qs := range dataQuery.QuerySettings {
				assert.EqualValues(t, clickhouse.CustomSetting{Value: fmt.Sprintf("%v", qs.Value)}, ctxSettings[qs.Setting])
			}

			assert.NotContains(t, strings.ToLower(dataQuery.RawSql), " querysettings ")

		})
	}
}
