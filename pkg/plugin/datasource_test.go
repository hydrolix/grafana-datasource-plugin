package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
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

// forwarder relays TCP to the ClickHouse container, so that a test can take
// the cluster away from an instance that has already connected to it.
type forwarder struct {
	listener net.Listener
	mu       sync.Mutex
	conns    []net.Conn
}

func newForwarder(t *testing.T, target string) *forwarder {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := &forwarder{listener: listener}
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			upstream, err := net.Dial("tcp", target)
			if err != nil {
				_ = client.Close()
				continue
			}
			f.mu.Lock()
			f.conns = append(f.conns, client, upstream)
			f.mu.Unlock()
			go func() { _, _ = io.Copy(upstream, client) }()
			go func() { _, _ = io.Copy(client, upstream) }()
		}
	}()
	t.Cleanup(f.Close)
	return f
}

func (f *forwarder) Port() uint16 {
	return uint16(f.listener.Addr().(*net.TCPAddr).Port)
}

// Close stops listening and drops every relayed connection, the pooled ones
// included: what was a cluster is now a closed port.
func (f *forwarder) Close() {
	_ = f.listener.Close()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.conns {
		_ = c.Close()
	}
	f.conns = nil
}

// A cluster that stops answering after the instance was built fails in sqlds's
// own health check, not in Connect: sqlds pings the connection it holds and
// answers with the ping's error as text. That text names the address like any
// other, so HdxSqlDatasource.CheckHealth replaces it.
func (s *DatasourceTestSuite) TestHealthCheckOfALostClusterNamesNoAddress() {
	for protocol, port := range map[string]uint16{"native": s.ChContainer.NativePort, "http": s.ChContainer.HttpPort} {
		s.T().Run(protocol, func(t *testing.T) {
			relay := newForwarder(t, net.JoinHostPort(s.ChContainer.Hostname, strconv.Itoa(int(port))))
			instance, err := plugin.NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
				UID: "lost-cluster-" + protocol,
				JSONData: []byte(fmt.Sprintf(`{"host": "127.0.0.1", "port": %d, "protocol": "%s", "username": "%s", "secure": false}`,
					relay.Port(), protocol, s.ChContainer.Username)),
				DecryptedSecureJSONData: map[string]string{"password": s.ChContainer.Password},
			})
			require.NoError(t, err)
			ds, ok := instance.(backend.CheckHealthHandler)
			require.True(t, ok)

			res, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
			require.NoError(t, err)
			require.Equal(t, backend.HealthStatusOk, res.Status, res.Message)

			relay.Close()

			res, err = ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
			require.NoError(t, err)
			assert.Equal(t, backend.HealthStatusError, res.Status)
			assert.Equal(t, "Connection refused", res.Message)
			for _, detail := range []string{"127.0.0.1", strconv.Itoa(int(relay.Port())), "dial tcp"} {
				assert.NotContains(t, res.Message, detail)
			}
		})
	}
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
