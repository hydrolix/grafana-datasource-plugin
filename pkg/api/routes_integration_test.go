package api_test

// This file lives in the external test package api_test (not api) on purpose:
// pkg/plugin imports pkg/api, so an internal api test importing pkg/plugin
// would form an import cycle. The external test package compiles after both,
// so it can wire the REAL plugin interpolator behind the /interpolate handler
// and exercise the route<->real-pipeline seam that stubInterpolator cannot.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/plugin/pkg/api"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// describeMetaDS is a metadataDS double (structurally satisfies the unexported
// plugin.metadataDS interface — all its methods are exported) that answers the
// DESCRIBE the adHocFilter macro issues with a canned name/type frame.
type describeMetaDS struct{}

func (describeMetaDS) QueryData(_ context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	refID := req.Queries[0].RefID
	name := "column"
	typ := "String"
	frame := &data.Frame{Fields: []*data.Field{
		data.NewField("name", nil, []*string{&name}),
		data.NewField("type", nil, []*string{&typ}),
	}}
	return &backend.QueryDataResponse{
		Responses: map[string]backend.DataResponse{
			refID: {Frames: data.Frames{frame}},
		},
	}, nil
}

func (describeMetaDS) InstanceSettings() backend.DataSourceInstanceSettings {
	return backend.DataSourceInstanceSettings{}
}
func (describeMetaDS) DefaultDatabase() string { return "mydb" }

// TestInterpolateRoute_RealInterpolator drives POST /interpolate through the
// real HdxInterpolator (real Macros registry) with a stubbed metadata layer.
// It proves the route hands the request to the real pipeline and returns the
// SQL the macros actually produced — not a canned stub value.
func TestInterpolateRoute_RealInterpolator(t *testing.T) {
	provider := plugin.NewMetadataProvider(describeMetaDS{})
	interp := plugin.NewHdxInterpolator(provider, plugin.Macros)
	ds := &sqlds.SQLDatasource{Interpolator: interp.Interpolate}

	from := time.Unix(1000, 0).UTC()
	to := time.Unix(2000, 0).UTC()
	body, err := json.Marshal(api.Request[api.QueryData]{Data: api.QueryData{
		RawSql:   "SELECT * FROM foo WHERE $__adHocFilter() AND $__timeFilter(ts)",
		Filters:  []models.AdHocFilter{{Key: "column", Operator: "=", Value: "test"}},
		Range:    api.Range{From: from, To: to},
		Interval: "30s",
	}})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/interpolate", strings.NewReader(string(body)))
	rr := httptest.NewRecorder()
	api.Routes(ds)["/interpolate"](rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	respBody, err := io.ReadAll(rr.Body)
	require.NoError(t, err)
	var resp api.Response[string]
	require.NoError(t, json.Unmarshal(respBody, &resp))

	assert.False(t, resp.Error, "errorMessage: %s", resp.ErrorMessage)
	// The response is the SQL the real macro pipeline produced.
	assert.Contains(t, resp.Data, "column = 'test'")
	assert.Contains(t, resp.Data, "ts >= toDateTime(1000)")
	assert.Contains(t, resp.Data, "ts <= toDateTime(2000)")
	assert.NotContains(t, resp.Data, "$__")
}

// TestInterpolateRoute_RejectsInjectedOperator confirms the route surfaces the
// pipeline's rejection of an injected filter operator as an error response
// rather than returning SQL that carried the injection.
func TestInterpolateRoute_RejectsInjectedOperator(t *testing.T) {
	provider := plugin.NewMetadataProvider(describeMetaDS{})
	interp := plugin.NewHdxInterpolator(provider, plugin.Macros)
	ds := &sqlds.SQLDatasource{Interpolator: interp.Interpolate}

	from := time.Unix(1000, 0).UTC()
	to := time.Unix(2000, 0).UTC()
	body, err := json.Marshal(api.Request[api.QueryData]{Data: api.QueryData{
		RawSql:   "SELECT * FROM foo WHERE $__adHocFilter()",
		Filters:  []models.AdHocFilter{{Key: "column", Operator: "= 'x' OR 1=1 -- ", Value: "x"}},
		Range:    api.Range{From: from, To: to},
		Interval: "30s",
	}})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/interpolate", strings.NewReader(string(body)))
	rr := httptest.NewRecorder()
	api.Routes(ds)["/interpolate"](rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	respBody, err := io.ReadAll(rr.Body)
	require.NoError(t, err)
	var resp api.Response[string]
	require.NoError(t, json.Unmarshal(respBody, &resp))

	assert.True(t, resp.Error, "injected operator must produce an error response")
	assert.NotContains(t, resp.Data, "OR 1=1")
}
