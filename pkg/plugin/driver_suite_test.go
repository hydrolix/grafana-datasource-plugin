package plugin_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/hydrolix/plugin/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ConnectivityTestSuite struct {
	suite.Suite
	testhelpers.DsTestSuite
}

func (s *ConnectivityTestSuite) SetupSuite() {
	s.DsTestSuite.SetupSuite()
	s.HdxPlugin = &plugin.Hydrolix{}
}

func (s *ConnectivityTestSuite) TearDownSuite() {
	s.DsTestSuite.TearDownSuite()
}

func TestConnectivityTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectivityTestSuite))
}

func (s *ConnectivityTestSuite) TestNativeConnect() {
	// queryTimeoutNumber := 3600
	// queryTimeoutString := "3600"
	t := s.T()

	t.Run("no error when valid datasource settings", func(t *testing.T) {
		settings := s.DatasourceSettings("native", s.ChContainer.NativePort)
		_, err := s.HdxPlugin.Connect(context.Background(), settings, json.RawMessage{})
		assert.NoError(t, err)
	})
	t.Run("error when invalid datasource settings", func(t *testing.T) {
		settings := s.DatasourceSettings("native", s.ChContainer.HttpPort)
		_, err := s.HdxPlugin.Connect(context.Background(), settings, json.RawMessage{})
		assert.Error(t, err)
	})
}

func (s *ConnectivityTestSuite) TestHttpConnect() {
	// queryTimeoutNumber := 3600
	// queryTimeoutString := "3600"
	t := s.T()

	t.Run("no error when valid datasource settings", func(t *testing.T) {
		settings := s.DatasourceSettings("http", s.ChContainer.HttpPort)
		_, err := s.HdxPlugin.Connect(context.Background(), settings, json.RawMessage{})
		assert.NoError(t, err)
	})
	t.Run("error when invalid datasource settings", func(t *testing.T) {
		settings := s.DatasourceSettings("http", s.ChContainer.NativePort)
		_, err := s.HdxPlugin.Connect(context.Background(), settings, json.RawMessage{})
		assert.Error(t, err)
	})
}
