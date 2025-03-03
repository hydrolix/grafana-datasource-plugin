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
	"strconv"
)

type DsTestSuite struct {
	ChContainer *ClickhouseContainer
	HdxPlugin   sqlds.Driver
	Ctx         context.Context
}

func (s *DsTestSuite) SetupSuite() {
	s.Ctx = context.Background()
	disable, err := strconv.ParseBool(os.Getenv("CLICKHOUSE_DISABLE_CONTAINER"))
	if err == nil && disable {
		s.ChContainer = &ClickhouseContainer{
			Hostname:   os.Getenv("CLICKHOUSE_HOSTNAME"),
			NativePort: 9000,
			HttpPort:   8123,
		}
	} else {
		chContainer, err := CreateClickhouseContainer(s.Ctx, "default", "")
		if err != nil {
			log.Fatal(err)
		}
		s.ChContainer = chContainer
	}
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
