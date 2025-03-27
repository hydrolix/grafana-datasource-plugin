package plugin_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v4"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/hydrolix/plugin/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

type DatasourceTestSuite struct {
	suite.Suite
	testhelpers.DsTestSuite
}

func (s *DatasourceTestSuite) SetupSuite() {
	s.DsTestSuite.SetupSuite()
	s.HdxPlugin = &plugin.Hydrolix{}
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
		case *sqlds.SQLDatasource:
			_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{},
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
		case *sqlds.SQLDatasource:
			_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{},
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
