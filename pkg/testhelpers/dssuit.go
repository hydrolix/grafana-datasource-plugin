package testhelpers

import (
	"context"
	"encoding/json"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v4"
	"github.com/hydrolix/plugin/pkg/models"
	"github.com/testcontainers/testcontainers-go"
	"log"
	"os"
)

// DsTestSuite is base test suite struct with CH container support.
type DsTestSuite struct {
	ChContainer *ClickhouseContainer
	HdxPlugin   sqlds.Driver
	Ctx         context.Context
}

// SetupSuite instantiates CH container if external one isn't provided.
// CLICKHOUSE_HOSTNAME env variable should points to provided CH service then.
func (s *DsTestSuite) SetupSuite() {
	s.Ctx = context.Background()

	if chHost := os.Getenv("CLICKHOUSE_HOSTNAME"); chHost != "" {
		s.ChContainer = &ClickhouseContainer{
			Hostname:   chHost,
			NativePort: 9000,
			HttpPort:   8123,
		}
		return
	}

	chContainer, err := NewClickhouseContainer(s.Ctx, "default", "")
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	s.ChContainer = chContainer
}

func (s *DsTestSuite) TearDownSuite() {
	if s.ChContainer == nil {
		return
	}
	if err := testcontainers.TerminateContainer(s.ChContainer.Container); err != nil {
		log.Printf("failed to terminate CH container: %s", err)
	}
}

func (s *DsTestSuite) DatasourceSettings(protocol string, port uint16) backend.DataSourceInstanceSettings {
	settings, err := json.Marshal(models.PluginSettings{
		Host:     s.ChContainer.Hostname,
		UserName: s.ChContainer.Username,
		Protocol: protocol,
		Port:     port,
	})
	if err != nil {
		panic(err)
	}
	return backend.DataSourceInstanceSettings{
		JSONData: settings,
		DecryptedSecureJSONData: map[string]string{
			"password": s.ChContainer.Password,
		},
	}
}
