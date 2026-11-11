package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/config"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DatasourceTestSuite struct {
	suite.Suite
	DsTestSuite
}

func (s *DatasourceTestSuite) SetupSuite() {
	s.DsTestSuite.SetupSuite()
	s.HdxPlugin = plugin.NewHydrolix()
}

func (s *DatasourceTestSuite) TearDownSuite() {
	s.DsTestSuite.TearDownSuite()
}

func TestDatasourceTestSuite(t *testing.T) {
	suite.Run(t, new(DatasourceTestSuite))
}

func (s *DatasourceTestSuite) TestNewDatasourceCreation() {
	t := s.T()
	t.Run("create http datasource", func(t *testing.T) {
		settings := backend.DataSourceInstanceSettings{
			Name: "test-hydrolix-http-datasource",
			JSONData: []byte(fmt.Sprintf(`{
			"host": "%s","port": %d,"protocol": "http",
			"username": "%s", "password": "%s", 
			"secure": false, "path": "/query", "skipTlsVerify": true
		}`, s.ChContainer.Hostname, s.ChContainer.HttpPort, s.ChContainer.Username, s.ChContainer.Password)),
		}
		_, err := plugin.NewDatasource(context.Background(), settings)
		assert.NoError(t, err)
	})

	t.Run("create native datasource", func(t *testing.T) {
		settings := backend.DataSourceInstanceSettings{
			Name: "test-hydrolix-native-datasource",
			JSONData: []byte(fmt.Sprintf(`{
			"host": "%s","port": %d,"protocol": "native",
			"username": "%s", "password": "%s", 
			"secure": false, "path": "/query", "skipTlsVerify": true
		}`, s.ChContainer.Hostname, s.ChContainer.NativePort, s.ChContainer.Username, s.ChContainer.Password)),
		}
		_, err := plugin.NewDatasource(context.Background(), settings)
		assert.NoError(t, err)
	})
}

func (s *DatasourceTestSuite) TestDatasourceRunQuery() {
	t := s.T()
	t.Run("run query for http datasource", func(t *testing.T) {
		settings := backend.DataSourceInstanceSettings{
			Name: "test-hydrolix-http-datasource",
			JSONData: []byte(fmt.Sprintf(`{
			"host": "%s","port": %d,"protocol": "http",
			"username": "%s", "password": "%s", 
			"secure": false, "path": "/query", "skipTlsVerify": true
		}`, s.ChContainer.Hostname, s.ChContainer.HttpPort, s.ChContainer.Username, s.ChContainer.Password)),
		}
		db, err := plugin.NewDatasource(context.Background(), settings)
		assert.NoError(t, err)

		switch ds := db.(type) {
		case *plugin.HdxSqlDatasource:
			_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
				Queries: []backend.DataQuery{
					backend.DataQuery{
						JSON: json.RawMessage(`{"rawSQL":"select now()", "Meta":{"TimeZone":"CDT"}, "Round":"2s", "RefID":"A"}`),
					},
				},
			})
			assert.NoError(t, err)
		default:
			t.Fatal("wrong sql datasource")
		}
	})

	t.Run("run query for native datasource", func(t *testing.T) {
		settings := backend.DataSourceInstanceSettings{
			Name: "test-hydrolix-native-datasource",
			JSONData: []byte(fmt.Sprintf(`{
			"host": "%s","port": %d,"protocol": "native",
			"username": "%s", "password": "%s", 
			"secure": false, "path": "/query", "skipTlsVerify": true
		}`, s.ChContainer.Hostname, s.ChContainer.NativePort, s.ChContainer.Username, s.ChContainer.Password)),
		}
		db, err := plugin.NewDatasource(context.Background(), settings)
		assert.NoError(t, err)

		switch ds := db.(type) {
		case *plugin.HdxSqlDatasource:
			_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
				Queries: []backend.DataQuery{
					backend.DataQuery{
						JSON: json.RawMessage(`{"rawSQL":"select now()", "Meta":{"TimeZone":"CDT"}, "Round":"2s", "RefID":"A"}`),
					},
				},
			})
			assert.NoError(t, err)
		default:
			t.Fatal("wrong sql datasource")
		}
	})
}

// TestDatasourceResponseLimit runs a query whose answer is larger than
// Grafana's dataproxy.response_limit against a real cluster, over HTTP, where
// the limit is enforced on the transport (transportFor). The SDK puts the
// limit on every handler's context; here the context is built the same way.
func (s *DatasourceTestSuite) TestDatasourceResponseLimit() {
	t := s.T()
	settingsFor := func(protocol string, port uint16) backend.DataSourceInstanceSettings {
		return backend.DataSourceInstanceSettings{
			UID:  "limited-" + protocol,
			Name: "test-hydrolix-" + protocol + "-datasource",
			Type: "hydrolix-hydrolix-datasource",
			JSONData: []byte(fmt.Sprintf(`{
			"host": "%s","port": %d,"protocol": "%s",
			"username": "%s", "password": "%s",
			"secure": false, "path": "/query", "skipTlsVerify": true
		}`, s.ChContainer.Hostname, port, protocol, s.ChContainer.Username, s.ChContainer.Password)),
		}
	}
	// A megabyte and more of UInt64s, well past a 64 KiB limit.
	const wide = `{"rawSQL":"select number from numbers(200000)", "Meta":{"TimeZone":"UTC"}, "RefID":"A"}`
	limited := config.WithGrafanaConfig(context.Background(), config.NewGrafanaCfg(map[string]string{
		config.ResponseLimit: fmt.Sprint(64 << 10),
	}))
	query := func(t *testing.T, ctx context.Context, settings backend.DataSourceInstanceSettings) error {
		t.Helper()
		db, err := plugin.NewDatasource(context.Background(), settings)
		require.NoError(t, err)
		ds, ok := db.(*plugin.HdxSqlDatasource)
		require.True(t, ok, "wrong sql datasource")
		resp, err := ds.QueryData(ctx, &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
			Queries:       []backend.DataQuery{{RefID: "A", JSON: json.RawMessage(wide)}},
		})
		require.NoError(t, err)
		return resp.Responses["A"].Error
	}

	t.Run("http: without a limit the answer is read whole", func(t *testing.T) {
		require.NoError(t, query(t, context.Background(), settingsFor("http", s.ChContainer.HttpPort)))
	})

	t.Run("http: an answer over Grafana's limit fails the query", func(t *testing.T) {
		err := query(t, limited, settingsFor("http", s.ChContainer.HttpPort))
		require.Error(t, err)
		assert.True(t, errors.Is(err, httpclient.ErrResponseBodyTooLarge), "got %v", err)
		assert.True(t, backend.IsDownstreamError(err))
		assert.Contains(t, err.Error(), "exceeds Grafana's response limit")
	})

	t.Run("native: the limit is an HTTP protection and does not apply", func(t *testing.T) {
		require.NoError(t, query(t, limited, settingsFor("native", s.ChContainer.NativePort)))
	})
}
