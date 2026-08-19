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
	"UUID":     "61f0c404-5cb3-11e7-907b-a6006ad3dba0",
	"IPv4":     "1.2.3.4",
	"IPv6":     "2001:db8::1",
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
				require.NoError(t, batch.Close())
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

// TestIPv6RenderingMatchesServer proves the IPv6 renderer agrees with ClickHouse
// itself rather than with a string this test hardcoded: it selects the column
// twice — once raw, once through the server's own toString() — and asserts the
// converted frame value equals the server's rendering.
//
// The value is an IPv4-mapped address, which is how Hydrolix stores IPv4 in its
// `ip` type, and the case where a value-driven formatter would disagree
// (net.IP.String() would give "1.2.3.4"). Runs over both protocols because the
// two take different paths to driver.Value.
func (s *ConvertersTestSuite) TestIPv6RenderingMatchesServer() {
	t := s.T()
	for name, port := range map[string]uint16{"native": s.ChContainer.NativePort, "http": s.ChContainer.HttpPort} {
		t.Run(name, func(t *testing.T) {
			settings := s.DatasourceSettings(name, port)
			db, err := s.HdxPlugin.Connect(s.Ctx, settings, json.RawMessage{})
			require.NoError(t, err)

			_, err = db.ExecContext(s.Ctx, "drop table if exists ip6_render_test")
			require.NoError(t, err)
			_, err = db.ExecContext(s.Ctx,
				"create table ip6_render_test (v IPv6, n Nullable(IPv6)) engine = MergeTree() order by tuple()")
			require.NoError(t, err)
			_, err = db.ExecContext(s.Ctx,
				"insert into ip6_render_test values ('::ffff:1.2.3.4', '::ffff:1.2.3.4')")
			require.NoError(t, err)

			// server_text is a String column, so it bypasses the IP converter
			// entirely and carries ClickHouse's own formatting.
			rows, err := db.Query("select v, n, toString(v) as server_text from ip6_render_test")
			require.NoError(t, err)

			frame, err := sqlutil.FrameFromRows(rows, 1, converters.Converters...)
			require.NoError(t, err)
			require.Equal(t, 3, len(frame.Fields))

			serverText := frame.Fields[2].At(0)
			assert.Equal(t, "::ffff:1.2.3.4", serverText,
				"sanity: ClickHouse should render the mapped address padded")

			assert.Equal(t, serverText, frame.Fields[0].At(0),
				"IPv6 column should render exactly as ClickHouse renders it")

			concrete, ok := frame.Fields[1].ConcreteAt(0)
			assert.True(t, ok)
			assert.Equal(t, serverText, concrete,
				"Nullable(IPv6) column should render exactly as ClickHouse renders it")

			_, err = db.ExecContext(s.Ctx, "drop table if exists ip6_render_test")
			require.NoError(t, err)
		})
	}
}
