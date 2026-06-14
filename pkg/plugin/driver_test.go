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
	"github.com/hydrolix/plugin/pkg/plugin/models"
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

// TestReadConnArg / TestGetOAuthToken / TestGetOrgId cover the flat
// connectionArgs shape that MutateQueryData writes in C4. The legacy
// HeaderKey-nested shape (populated by ForwardHeaders=true) is no longer
// the source of truth; C2 set ForwardHeaders=false and C4 inverts the
// keying flow so the plugin writes connectionArgs itself.

func TestReadConnArg(t *testing.T) {
	tests := []struct {
		name    string
		args    json.RawMessage
		key     string
		wantVal string
		wantOK  bool
	}{
		{
			name:    "nil args",
			args:    nil,
			key:     "oauthToken",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "empty args",
			args:    json.RawMessage(``),
			key:     "oauthToken",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "empty JSON object",
			args:    json.RawMessage(`{}`),
			key:     "oauthToken",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "key present",
			args:    json.RawMessage(`{"oauthToken":"abc"}`),
			key:     "oauthToken",
			wantVal: "abc",
			wantOK:  true,
		},
		{
			name:    "key empty value",
			args:    json.RawMessage(`{"oauthToken":""}`),
			key:     "oauthToken",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "different key returned independently",
			args:    json.RawMessage(`{"oauthToken":"t","orgId":"5"}`),
			key:     "orgId",
			wantVal: "5",
			wantOK:  true,
		},
		{
			name:    "malformed JSON",
			args:    json.RawMessage(`not-json`),
			key:     "oauthToken",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "wrong shape (legacy nested HeaderKey)",
			args:    json.RawMessage(`{"grafana-http-headers":{"Authorization":["Bearer abc"]}}`),
			key:     "oauthToken",
			wantVal: "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := readConnArg(tt.args, tt.key)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

func TestGetOAuthToken(t *testing.T) {
	tests := []struct {
		name      string
		args      json.RawMessage
		wantToken string
		wantOK    bool
	}{
		{
			name:      "nil args",
			args:      nil,
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "bare token (no Bearer prefix is the contract)",
			args:      json.RawMessage(`{"oauthToken":"abc123"}`),
			wantToken: "abc123",
			wantOK:    true,
		},
		{
			name:      "empty token",
			args:      json.RawMessage(`{"oauthToken":""}`),
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "complex JWT bare value",
			args:      json.RawMessage(`{"oauthToken":"eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1In0.sig"}`),
			wantToken: "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1In0.sig",
			wantOK:    true,
		},
		{
			name:      "token absent when only orgId set",
			args:      json.RawMessage(`{"orgId":"5"}`),
			wantToken: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := getOAuthToken(tt.args)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestGetOrgId(t *testing.T) {
	tests := []struct {
		name    string
		args    json.RawMessage
		wantVal string
		wantOK  bool
	}{
		{
			name:    "nil args",
			args:    nil,
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "orgId present",
			args:    json.RawMessage(`{"orgId":"1"}`),
			wantVal: "1",
			wantOK:  true,
		},
		{
			name:    "orgId absent when only oauthToken set",
			args:    json.RawMessage(`{"oauthToken":"t"}`),
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "orgId empty string",
			args:    json.RawMessage(`{"orgId":""}`),
			wantVal: "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := getOrgId(tt.args)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}

func TestSettings_ForwardHeaders(t *testing.T) {
	// Post-C2 invariant: ForwardHeaders is always false regardless of
	// credentials type. C4's OAuth-keying flow injects connectionArgs via
	// MutateQueryData, and ForwardHeaders=true would otherwise pollute the
	// connection cache key by writing the full HTTP header map into
	// ConnectionArgs.
	tests := []struct {
		name            string
		credentialsType string
		wantForward     bool
	}{
		{
			name:            "forwardOAuth still disables ForwardHeaders (C2 invariant)",
			credentialsType: "forwardOAuth",
			wantForward:     false,
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

// makeQueryDataReq builds a *backend.QueryDataRequest, using the SDK's
// SetHTTPHeader API so headers go through the same prefix-handling as in
// production. Writing directly to req.Headers would mostly work for
// Authorization but silently drop X-Grafana-Org-Id (the SDK only passes
// arbitrary headers through when stored under the `http_` prefix).
func makeQueryDataReq(t *testing.T, credentialsType string, headers map[string]string, rawJSON string) *backend.QueryDataRequest {
	t.Helper()
	settings := models.PluginSettings{
		Host:            "localhost",
		Port:            80,
		Protocol:        "http",
		CredentialsType: credentialsType,
		DialTimeout:     "10",
		QueryTimeout:    "20",
	}
	jsonData, err := json.Marshal(settings)
	assert.NoError(t, err)

	req := &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{
			DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
				JSONData:                jsonData,
				DecryptedSecureJSONData: map[string]string{},
			},
		},
		Queries: []backend.DataQuery{
			{RefID: "A", JSON: []byte(rawJSON)},
		},
	}
	for k, v := range headers {
		req.SetHTTPHeader(k, v)
	}
	return req
}

func TestMutateQueryData_InjectsConnectionArgs(t *testing.T) {
	tests := []struct {
		name            string
		credentialsType string
		oauthHeader     string
		orgHeader       string
		wantArgs        map[string]string // nil = no connectionArgs key written
	}{
		{
			name:            "forwardOAuth + Bearer token + org id → both keys",
			credentialsType: "forwardOAuth",
			oauthHeader:     "Bearer abc",
			orgHeader:       "5",
			wantArgs:        map[string]string{"oauthToken": "abc", "orgId": "5"},
		},
		{
			name:            "forwardOAuth + bare token (no Bearer prefix) → token stored as-is",
			credentialsType: "forwardOAuth",
			oauthHeader:     "abc",
			orgHeader:       "",
			wantArgs:        map[string]string{"oauthToken": "abc"},
		},
		{
			name:            "forwardOAuth + empty oauth header + no org → no connectionArgs",
			credentialsType: "forwardOAuth",
			oauthHeader:     "",
			orgHeader:       "",
			wantArgs:        nil,
		},
		{
			name:            "userAccount + oauth header is IGNORED (only orgId injected)",
			credentialsType: "userAccount",
			oauthHeader:     "Bearer abc",
			orgHeader:       "7",
			wantArgs:        map[string]string{"orgId": "7"},
		},
		{
			name:            "userAccount + no headers → no connectionArgs",
			credentialsType: "userAccount",
			oauthHeader:     "",
			orgHeader:       "",
			wantArgs:        nil,
		},
		{
			name:            "serviceAccount + only orgId → orgId only",
			credentialsType: "serviceAccount",
			oauthHeader:     "",
			orgHeader:       "9",
			wantArgs:        map[string]string{"orgId": "9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHydrolix()
			headers := map[string]string{}
			if tt.oauthHeader != "" {
				headers[backend.OAuthIdentityTokenHeaderName] = tt.oauthHeader
			}
			if tt.orgHeader != "" {
				headers[OrgIdHeaderKey] = tt.orgHeader
			}
			req := makeQueryDataReq(t, tt.credentialsType, headers, `{"rawSql":"SELECT 1"}`)

			_, out := h.MutateQueryData(context.Background(), req)

			var parsed struct {
				RawSql         string            `json:"rawSql"`
				ConnectionArgs map[string]string `json:"connectionArgs,omitempty"`
				QuerySettings  []models.QuerySetting `json:"querySettings"`
			}
			assert.NoError(t, json.Unmarshal(out.Queries[0].JSON, &parsed))
			assert.Equal(t, "SELECT 1", parsed.RawSql, "rawSql preserved")
			assert.NotNil(t, parsed.QuerySettings, "querySettings field still written (existing behaviour)")
			assert.Equal(t, tt.wantArgs, parsed.ConnectionArgs)
		})
	}
}

func TestMutateQueryData_NoCrossRequestLeak(t *testing.T) {
	h := NewHydrolix()
	reqA := makeQueryDataReq(t, "forwardOAuth", map[string]string{
		backend.OAuthIdentityTokenHeaderName: "Bearer tokenA",
	}, `{"rawSql":"A"}`)
	reqB := makeQueryDataReq(t, "forwardOAuth", map[string]string{
		backend.OAuthIdentityTokenHeaderName: "Bearer tokenB",
	}, `{"rawSql":"B"}`)

	_, outA := h.MutateQueryData(context.Background(), reqA)
	_, outB := h.MutateQueryData(context.Background(), reqB)

	var parsedA, parsedB struct {
		RawSql         string            `json:"rawSql"`
		ConnectionArgs map[string]string `json:"connectionArgs"`
	}
	assert.NoError(t, json.Unmarshal(outA.Queries[0].JSON, &parsedA))
	assert.NoError(t, json.Unmarshal(outB.Queries[0].JSON, &parsedB))

	assert.Equal(t, "tokenA", parsedA.ConnectionArgs["oauthToken"])
	assert.Equal(t, "tokenB", parsedB.ConnectionArgs["oauthToken"])
}

func TestConnect_LazyBootstrapForwardOAuth(t *testing.T) {
	// The sqlds.NewConnector bootstrap call always passes args == nil. Under
	// forwardOAuth there is no token at that point. C4 makes Connect return
	// a lazy *sql.DB (sql.OpenDB does not contact the server) so per-user
	// pools can land later via per-query Connect calls.
	h := NewHydrolix()
	settings := models.PluginSettings{
		Host:            "localhost",
		Port:            80,
		Protocol:        "http",
		CredentialsType: "forwardOAuth",
		DialTimeout:     "10",
		QueryTimeout:    "20",
	}
	jsonData, err := json.Marshal(settings)
	assert.NoError(t, err)

	db, err := h.Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, nil)

	assert.NoError(t, err, "bootstrap call must not error under forwardOAuth")
	assert.NotNil(t, db, "must return a usable *sql.DB")
	if db != nil {
		_ = db.Close()
	}
}

func TestConnect_MissingOAuthTokenWhenArgsPresent(t *testing.T) {
	// The real per-query path: args != nil but oauthToken is absent. This is
	// a real error (the request reached Connect without MutateQueryData
	// having written the token; should never happen but must surface clearly).
	h := NewHydrolix()
	settings := models.PluginSettings{
		Host:            "localhost",
		Port:            80,
		Protocol:        "http",
		CredentialsType: "forwardOAuth",
		DialTimeout:     "10",
		QueryTimeout:    "20",
	}
	jsonData, err := json.Marshal(settings)
	assert.NoError(t, err)

	db, err := h.Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{},
	}, json.RawMessage(`{"orgId":"5"}`))

	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "missing OAuth token")
}
