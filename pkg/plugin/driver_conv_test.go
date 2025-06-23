package plugin_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/hydrolix/plugin/pkg/converters"
	"github.com/hydrolix/plugin/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConvertersTestSuite struct {
	suite.Suite
	DsTestSuite
}

func (s *ConvertersTestSuite) SetupSuite() {
	s.DsTestSuite.SetupSuite()
	s.HdxPlugin = plugin.NewHydrolix()
}

func (s *ConvertersTestSuite) TearDownSuite() {
	s.DsTestSuite.TearDownSuite()
}

func TestConvertersTestSuite(t *testing.T) {
	suite.Run(t, new(ConvertersTestSuite))
}

var dt, _ = time.Parse(time.DateTime, "2025-02-11 01:01:01")
var testData = map[string]interface{}{
	"UInt8":    uint8(255),
	"UInt16":   uint16(255),
	"UInt32":   uint32(255),
	"UInt64":   uint64(255),
	"Int8":     int8(127),
	"Int16":    int16(255),
	"Int32":    int32(255),
	"Int64":    int64(255),
	"Bool":     true,
	"DateTime": dt,
	"String":   "1234567890",
}

func (s *ConvertersTestSuite) TestConverters() {
	t := s.T()
	for name, port := range map[string]uint16{"native": s.ChContainer.NativePort, "http": s.ChContainer.HttpPort} {
		settings := s.DatasourceSettings(name, port)
		for kind, val := range testData {
			t.Run(fmt.Sprintf("using %s for %s", name, kind), func(t *testing.T) {
				db, err := s.HdxPlugin.Connect(s.Ctx, settings, json.RawMessage{})

				require.NoError(t, err)
				_, err = db.ExecContext(s.Ctx, "drop table if exists conv_test ")
				require.NoError(t, err)
				_, err = db.ExecContext(s.Ctx, fmt.Sprintf("create table conv_test (valcol %s, nilcol Nullable(%s)) engine = MergeTree() order by tuple() ", kind, kind))
				require.NoError(t, err)

				tx, err := db.BeginTx(s.Ctx, nil)
				require.NoError(t, err)
				batch, err := tx.PrepareContext(s.Ctx, "insert into conv_test ")
				require.NoError(t, err)
				_, err = batch.ExecContext(s.Ctx, val, val)
				require.NoError(t, err)
				_, err = batch.ExecContext(s.Ctx, val, nil)
				require.NoError(t, err)
				batch.Close()
				require.NoError(t, tx.Commit())

				res, err := db.Query("select * from conv_test")
				require.NoError(t, err)

				frame, err := sqlutil.FrameFromRows(res, 2, converters.Converters...)
				require.NoError(t, err)

				assert.Equal(t, 2, len(frame.Fields))

				assert.Equal(t, val, frame.Fields[0].At(0))
				assert.Equal(t, val, frame.Fields[0].At(1))

				cval, _ := frame.Fields[1].ConcreteAt(0)
				assert.Equal(t, val, cval)
				assert.Nil(t, frame.Fields[1].At(1))

				_, err = db.ExecContext(s.Ctx, "drop table if exists conv_test ")
				require.NoError(t, err)
			})
		}
	}

}
