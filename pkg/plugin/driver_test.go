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

			// datasource-level settings that were not overridden keep original values.
			// hdx_query_admin_comment has the managed Grafana fragment appended, so
			// only the prefix is asserted here; full coverage lives in dedicated tests.
			for _, v := range dsQuerySettings {
				if slices.Contains([]string{"hdx_query_timerange_required", "hdx_query_max_result_rows"}, v.Setting) {
					continue
				}
				if v.Setting == adminCommentSetting {
					assert.True(t, strings.HasPrefix(findSettingValue(actualSettings, v.Setting), v.Value+"; "))
					continue
				}
				assert.EqualValues(t, v.Value, findSettingValue(actualSettings, v.Setting))
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
				if slices.Contains([]string{"hdx_query_timerange_required", "hdx_query_max_result_rows"}, v.Setting) {
					continue
				}
				if v.Setting == adminCommentSetting {
					assert.True(t, strings.HasPrefix(findSettingValue(actualSettings, v.Setting), v.Value+"; "))
					continue
				}
				assert.EqualValues(t, v.Value, findSettingValue(actualSettings, v.Setting))
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

// testPluginJSONData builds a DataSourceInstanceSettings.JSONData payload
// suitable for driving MutateQueryData in tests. Pass includeUserIdentity
// to flip the PII gate.
func testPluginJSONData(t *testing.T, includeUserIdentity bool) []byte {
	t.Helper()
	jsonData, err := json.Marshal(map[string]any{
		"host":                             "localhost",
		"port":                             80,
		"protocol":                         "http",
		"dialTimeout":                      "10",
		"queryTimeout":                     "20",
		"includeUserIdentityInAttribution": includeUserIdentity,
	})
	if err != nil {
		t.Fatal(err)
	}
	return jsonData
}

// adminCommentValueFromMutate runs MutateQueryData against an empty-settings
// data source and returns the resulting hdx_query_admin_comment value. The
// default JSONData enables the PII gate so existing tests can assert on
// user_email / user_login values; tests of the off-state construct their
// own request directly.
func adminCommentValueFromMutate(t *testing.T, plugin *Hydrolix, user *backend.User, queryJSON string) (settings []models.QuerySetting) {
	t.Helper()
	jsonData := testPluginJSONData(t, true)
	var refIDProbe struct {
		RefID string `json:"refId"`
	}
	_ = json.Unmarshal([]byte(queryJSON), &refIDProbe)
	req := &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			User: user,
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				Name:     "ds",
				JSONData: jsonData,
			},
		},
		Queries: []backend.DataQuery{{RefID: refIDProbe.RefID, JSON: []byte(queryJSON)}},
	}
	_, req = plugin.MutateQueryData(context.Background(), req)
	var out struct {
		QuerySettings []models.QuerySetting `json:"querySettings"`
	}
	if err := json.Unmarshal(req.Queries[0].JSON, &out); err != nil {
		t.Fatal(err)
	}
	return out.QuerySettings
}

func TestGrafanaAdminCommentInjection(t *testing.T) {
	plugin := &Hydrolix{querySettingsContextHandler: testContextHandler}

	t.Run("no existing settings — admin comment is created from PluginContext.User", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A"}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "user_email=alice@example.com")
		assert.Contains(t, got, "user_login=alice")
		assert.Contains(t, got, "ref_id=A")
		assert.Contains(t, got, "panel_id=unknown")
	})

	t.Run("nil PluginContext.User emits unknown", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin, nil,
			`{"rawSql":"SELECT 1","refId":"B"}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "user_email=unknown")
		assert.Contains(t, got, "user_login=unknown")
		assert.Contains(t, got, "ref_id=B")
	})

	t.Run("empty email but populated login", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A"}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "user_email=unknown")
		assert.Contains(t, got, "user_login=alice")
	})

	t.Run("panel metadata from query JSON is included when present", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A","meta":{"grafana":{"panelId":7,"panelName":"Requests","panelPluginId":"timeseries","dashboardUID":"abc123","dashboardTitle":"Production","app":"dashboard","requestId":"Q123"}}}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Equal(t,
			"grafana_meta_start; user_email=alice@example.com; user_login=alice; user_name=unknown; user_role=unknown; org_id=unknown; panel_id=7; panel_name=Requests; panel_plugin_id=timeseries; dashboard_uid=abc123; dashboard_title=Production; app=dashboard; ref_id=A; request_id=Q123; grafana_meta_end",
			got)
	})

	t.Run("missing panel metadata emits unknown", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A","meta":{"grafana":{"app":"explore"}}}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "panel_id=unknown")
		assert.Contains(t, got, "panel_name=unknown")
		assert.Contains(t, got, "panel_plugin_id=unknown")
		assert.Contains(t, got, "dashboard_uid=unknown")
		assert.Contains(t, got, "dashboard_title=unknown")
		assert.Contains(t, got, "app=explore")
		assert.Contains(t, got, "request_id=unknown")
	})

	t.Run("user Name and Role are forwarded when set", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice", Name: "Alice Example", Role: "Editor"},
			`{"rawSql":"SELECT 1","refId":"A"}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "user_name=Alice Example")
		assert.Contains(t, got, "user_role=Editor")
	})

	t.Run("missing user Name and Role fall back to unknown", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A"}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "user_name=unknown")
		assert.Contains(t, got, "user_role=unknown")
	})

	t.Run("PII is gated by includeUserIdentityInAttribution flag", func(t *testing.T) {
		// When the flag is FALSE (default), email/login/name must always be
		// "unknown" even though Grafana provided real values. Role and OrgID
		// are not gated.
		jsonData := testPluginJSONData(t, false)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				OrgID: 7,
				User: &backend.User{
					Email: "alice@example.com",
					Login: "alice",
					Name:  "Alice Example",
					Role:  "Editor",
				},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSql":"SELECT 1","refId":"A"}`)}},
		}
		_, req = plugin.MutateQueryData(context.Background(), req)
		var out struct {
			QuerySettings []models.QuerySetting `json:"querySettings"`
		}
		assert.NoError(t, json.Unmarshal(req.Queries[0].JSON, &out))
		got := findSettingValue(out.QuerySettings, adminCommentSetting)

		// PII redacted.
		assert.Contains(t, got, "user_email=unknown")
		assert.Contains(t, got, "user_login=unknown")
		assert.Contains(t, got, "user_name=unknown")
		assert.NotContains(t, got, "alice@example.com")
		assert.NotContains(t, got, "Alice Example")
		// Non-PII still emitted.
		assert.Contains(t, got, "user_role=Editor")
		assert.Contains(t, got, "org_id=7")
	})

	t.Run("PII included when includeUserIdentityInAttribution is true", func(t *testing.T) {
		jsonData := testPluginJSONData(t, true)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				User: &backend.User{
					Email: "alice@example.com",
					Login: "alice",
					Name:  "Alice Example",
				},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSql":"SELECT 1","refId":"A"}`)}},
		}
		_, req = plugin.MutateQueryData(context.Background(), req)
		var out struct {
			QuerySettings []models.QuerySetting `json:"querySettings"`
		}
		assert.NoError(t, json.Unmarshal(req.Queries[0].JSON, &out))
		got := findSettingValue(out.QuerySettings, adminCommentSetting)
		assert.Contains(t, got, "user_email=alice@example.com")
		assert.Contains(t, got, "user_login=alice")
		assert.Contains(t, got, "user_name=Alice Example")
	})

	t.Run("PII gate defaults to off when flag absent from JSONData", func(t *testing.T) {
		// JSONData has no includeUserIdentityInAttribution field at all.
		jsonData, _ := json.Marshal(map[string]any{
			"host": "h", "port": 80, "protocol": "http",
			"dialTimeout": "1", "queryTimeout": "1",
		})
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				User:                       &backend.User{Email: "alice@example.com", Login: "alice"},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSql":"SELECT 1","refId":"A"}`)}},
		}
		_, req = plugin.MutateQueryData(context.Background(), req)
		var out struct {
			QuerySettings []models.QuerySetting `json:"querySettings"`
		}
		assert.NoError(t, json.Unmarshal(req.Queries[0].JSON, &out))
		got := findSettingValue(out.QuerySettings, adminCommentSetting)
		assert.Contains(t, got, "user_email=unknown")
		assert.Contains(t, got, "user_login=unknown")
	})

	t.Run("OrgID is forwarded when set, falls back to unknown for 0", func(t *testing.T) {
		jsonData := testPluginJSONData(t, true)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				OrgID:                      42,
				User:                       &backend.User{Email: "alice@example.com", Login: "alice"},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSql":"SELECT 1","refId":"A"}`)}},
		}
		_, req = plugin.MutateQueryData(context.Background(), req)
		var out struct {
			QuerySettings []models.QuerySetting `json:"querySettings"`
		}
		assert.NoError(t, json.Unmarshal(req.Queries[0].JSON, &out))
		assert.Contains(t, findSettingValue(out.QuerySettings, adminCommentSetting), "org_id=42")

		// OrgID == 0 → unknown.
		req2 := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				User:                       &backend.User{Email: "alice@example.com", Login: "alice"},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSql":"SELECT 1","refId":"A"}`)}},
		}
		_, req2 = plugin.MutateQueryData(context.Background(), req2)
		assert.NoError(t, json.Unmarshal(req2.Queries[0].JSON, &out))
		assert.Contains(t, findSettingValue(out.QuerySettings, adminCommentSetting), "org_id=unknown")
	})

	t.Run("requestId and panelPluginId are forwarded from frontend meta", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A","meta":{"grafana":{"panelPluginId":"timeseries","requestId":"Q-42"}}}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "panel_plugin_id=timeseries")
		assert.Contains(t, got, "request_id=Q-42")
	})

	t.Run("malformed query JSON does not block later queries in the same request", func(t *testing.T) {
		jsonData := testPluginJSONData(t, true)
		malformed := []byte(`{"rawSql":"SELECT 1","refId":"A"`)
		wellFormed := []byte(`{"rawSql":"SELECT 2","refId":"B"}`)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				User:                       &backend.User{Email: "a@b", Login: "a"},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: malformed},
				{RefID: "B", JSON: wellFormed},
			},
		}
		_, req = plugin.MutateQueryData(context.Background(), req)

		// Query 0 (malformed) is left untouched.
		assert.Equal(t, string(malformed), string(req.Queries[0].JSON))

		// Query 1 still receives the managed admin comment — proves the loop
		// did not exit early after the failed jsonSet on query 0.
		var out struct {
			QuerySettings []models.QuerySetting `json:"querySettings"`
		}
		assert.NoError(t, json.Unmarshal(req.Queries[1].JSON, &out))
		got := findSettingValue(out.QuerySettings, adminCommentSetting)
		assert.Contains(t, got, "user_email=a@b")
		assert.Contains(t, got, "ref_id=B")
	})

	t.Run("malformed query JSON does not fail injection", func(t *testing.T) {
		jsonData := testPluginJSONData(t, true)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				User:                       &backend.User{Email: "a@b", Login: "a"},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"rawSql":"SELECT 1","refId":"A"`)}},
		}
		_, _ = plugin.MutateQueryData(context.Background(), req)
		// Malformed JSON cannot be re-serialised via jsonSet, so the query JSON
		// is left untouched — verify that the call did not panic.
		assert.Equal(t, `{"rawSql":"SELECT 1","refId":"A"`, string(req.Queries[0].JSON))
	})

	t.Run("existing query-level admin comment is preserved and appended", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A","querySettings":[{"setting":"hdx_query_admin_comment","value":"custom=foo"}]}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.True(t, strings.HasPrefix(got, "custom=foo; "))
		assert.Contains(t, got, "user_email=alice@example.com")
	})

	t.Run("query-level setting cannot remove managed user metadata", func(t *testing.T) {
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "alice@example.com", Login: "alice"},
			`{"rawSql":"SELECT 1","refId":"A","querySettings":[{"setting":"hdx_query_admin_comment","value":""}]}`)
		got := findSettingValue(settings, adminCommentSetting)
		assert.Contains(t, got, "user_email=alice@example.com")
		assert.Contains(t, got, "user_login=alice")
	})

	t.Run("repeated MutateQueryData does not duplicate managed fragment", func(t *testing.T) {
		jsonData := testPluginJSONData(t, true)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				OrgID:                      1,
				User:                       &backend.User{Email: "alice@example.com", Login: "alice"},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{
				RefID: "A",
				JSON:  []byte(`{"rawSql":"SELECT 1","refId":"A","querySettings":[{"setting":"hdx_query_admin_comment","value":"custom=foo"}]}`),
			}},
		}
		_, req = plugin.MutateQueryData(context.Background(), req)
		_, req = plugin.MutateQueryData(context.Background(), req)
		_, req = plugin.MutateQueryData(context.Background(), req)

		var out struct {
			QuerySettings []models.QuerySetting `json:"querySettings"`
		}
		assert.NoError(t, json.Unmarshal(req.Queries[0].JSON, &out))
		got := findSettingValue(out.QuerySettings, adminCommentSetting)

		// User fragment is preserved exactly once.
		assert.Equal(t, 1, strings.Count(got, "custom=foo"))
		// Managed block is present exactly once.
		assert.Equal(t, 1, strings.Count(got, "grafana_meta_start"))
		assert.Equal(t, 1, strings.Count(got, "grafana_meta_end"))
		assert.Equal(t, 1, strings.Count(got, "user_email=alice@example.com"))
		// Ordering: user content first, then managed block.
		assert.True(t, strings.HasPrefix(got, "custom=foo; grafana_meta_start; "))
		assert.True(t, strings.HasSuffix(got, "; grafana_meta_end"))
	})

	t.Run("managed block emitted from a prior run is stripped on re-mutate", func(t *testing.T) {
		// Simulate a request whose querySettings already contain a managed
		// block as if it had been mutated by an earlier pass.
		stale := "grafana_meta_start; user_email=stale@example.com; user_login=stale; grafana_meta_end"
		settings := adminCommentValueFromMutate(t, plugin,
			&backend.User{Email: "fresh@example.com", Login: "fresh"},
			`{"rawSql":"SELECT 1","refId":"A","querySettings":[{"setting":"hdx_query_admin_comment","value":"`+stale+`"}]}`)
		got := findSettingValue(settings, adminCommentSetting)
		// Stale block is gone.
		assert.NotContains(t, got, "stale@example.com")
		assert.NotContains(t, got, "user_login=stale")
		// Fresh managed block replaces it.
		assert.Contains(t, got, "user_email=fresh@example.com")
		assert.Equal(t, 1, strings.Count(got, "grafana_meta_start"))
	})

	t.Run("MutateQuery converts managed comment into clickhouse.CustomSetting", func(t *testing.T) {
		jsonData := testPluginJSONData(t, true)
		req := &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				User:                       &backend.User{Email: "alice@example.com", Login: "alice"},
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{JSONData: jsonData},
			},
			Queries: []backend.DataQuery{{
				RefID: "A",
				JSON:  []byte(`{"rawSql":"SELECT 1","refId":"A"}`),
			}},
		}
		ctx, req := plugin.MutateQueryData(context.Background(), req)
		ctx, _ = plugin.MutateQuery(ctx, req.Queries[0])
		ctxSettings := ctx.Value("querySettings").(map[string]any)
		v, ok := ctxSettings[adminCommentSetting].(clickhouse.CustomSetting)
		assert.True(t, ok)
		assert.Contains(t, v.Value, "user_email=alice@example.com")
	})
}

func TestNormalizeAdminCommentValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "unknown"},
		{"   ", "unknown"},
		{"  foo  ", "foo"},
		{"line1\nline2", "line1 line2"},
		{"a;b;c", "a b c"},
		{"  a;\nb  ", "a  b"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, normalizeAdminCommentValue(c.in), "input=%q", c.in)
	}
}

func TestStripManagedAdminCommentFragment(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "value without markers is returned unchanged",
			in:   "custom=foo; other=bar",
			want: "custom=foo; other=bar",
		},
		{
			name: "managed block on its own is removed",
			in:   "grafana_meta_start; user_email=a@b; user_login=a; grafana_meta_end",
			want: "",
		},
		{
			name: "user prefix is preserved when managed block follows",
			in:   "custom=foo; grafana_meta_start; user_email=a@b; grafana_meta_end",
			want: "custom=foo",
		},
		{
			name: "user suffix is preserved when managed block precedes",
			in:   "grafana_meta_start; user_email=a@b; grafana_meta_end; custom=foo",
			want: "custom=foo",
		},
		{
			name: "user content sandwiched between two managed blocks is preserved",
			in:   "grafana_meta_start; user_email=a@b; grafana_meta_end; custom=foo; grafana_meta_start; user_email=c@d; grafana_meta_end",
			want: "custom=foo",
		},
		{
			name: "unterminated managed block is fully stripped",
			in:   "custom=foo; grafana_meta_start; user_email=a@b",
			want: "custom=foo",
		},
		{
			name: "escaped semicolon inside a user value is not a separator",
			in:   `note=hello\; world; grafana_meta_start; user_email=a@b; grafana_meta_end`,
			want: `note=hello\; world`,
		},
		{
			name: "escaped backslash is preserved verbatim",
			in:   `path=c:\\users; grafana_meta_start; user_email=a@b; grafana_meta_end`,
			want: `path=c:\\users`,
		},
		{
			name: "escaped semicolon adjacent to a real separator",
			in:   `a=foo\;bar; b=baz; grafana_meta_start; user_email=a@b; grafana_meta_end`,
			want: `a=foo\;bar; b=baz`,
		},
		{
			name: "separator without trailing space still splits",
			in:   `custom=foo;grafana_meta_start;user_email=a@b;grafana_meta_end`,
			want: "custom=foo",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, stripManagedAdminCommentFragment(c.in))
		})
	}
}

func TestSplitAdminCommentSegments(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "single segment", in: "foo", want: []string{"foo"}},
		{name: "canonical separator", in: "foo; bar", want: []string{"foo", "bar"}},
		{name: "separator without trailing space", in: "foo;bar", want: []string{"foo", "bar"}},
		{name: "escaped semicolon stays inside segment", in: `foo\; bar; baz`, want: []string{`foo\; bar`, "baz"}},
		{name: "escaped backslash preserved", in: `a\\b; c`, want: []string{`a\\b`, "c"}},
		{name: "trailing backslash kept literally", in: `foo\`, want: []string{`foo\`}},
		{name: "trailing separator yields empty segment", in: "foo; ", want: []string{"foo", ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, splitAdminCommentSegments(c.in))
		})
	}
}

func TestPanelIDString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty raw", in: "", want: "unknown"},
		{name: "whitespace only", in: "   ", want: "unknown"},
		{name: "json null", in: "null", want: "unknown"},
		{name: "integer", in: "7", want: "7"},
		{name: "negative integer", in: "-1", want: "-1"},
		{name: "zero", in: "0", want: "0"},
		{name: "float", in: "7.5", want: "7.5"},
		{name: "quoted numeric string", in: `"7"`, want: "7"},
		{name: "quoted string with space", in: `"with space"`, want: "with space"},
		{name: "empty quoted string falls back to unknown", in: `""`, want: "unknown"},
		{name: "string containing semicolon is sanitised", in: `"a;b"`, want: "a b"},
		// Bug #5 regressions: previously slipped through to the admin comment.
		{name: "lone unbalanced quote", in: `"`, want: "unknown"},
		{name: "unterminated quoted string", in: `"foo`, want: "unknown"},
		{name: "json object is not a valid panel id", in: `{}`, want: "unknown"},
		{name: "json array is not a valid panel id", in: `[1]`, want: "unknown"},
		{name: "json boolean is not a valid panel id", in: `true`, want: "unknown"},
		{name: "garbage bytes", in: `???`, want: "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, panelIDString(json.RawMessage(c.in)))
		})
	}
}
