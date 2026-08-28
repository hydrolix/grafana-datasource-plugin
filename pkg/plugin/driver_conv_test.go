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

// ipv6RenderCases are the addresses TestIPv6RenderingMatchesServer round-trips.
// The set spans every branch the renderer can take:
//
//   - IPv4-mapped, which is how Hydrolix stores IPv4 in its `ip` type, and the
//     case where a value-driven formatter disagrees (net.IP.String() would give
//     "1.2.3.4");
//   - IPv4-compatible, which ClickHouse renders with a dotted-quad tail while
//     netip renders in hex;
//   - low bits set but bytes 12-13 zero, the boundary that must stay hex;
//   - an ordinary address and the two all-zero-ish edges.
//
// No expected strings here on purpose: the server supplies them.
var ipv6RenderCases = []string{
	"2001:db8::1",
	"::ffff:1.2.3.4",
	"::ffff:0.0.0.0",
	"::1.2.3.4",
	"::10.0.0.1",
	"::0.1.0.0",
	"::255.255.255.255",
	"::2",
	"::100",
	"::ffff",
	"::",
	"::1",
}

// TestIPv6RenderingMatchesServer proves the IPv6 renderer agrees with ClickHouse
// itself rather than with a string this test hardcoded: it selects the column
// twice — once raw, once through the server's own toString() — and asserts the
// converted frame value equals the server's rendering, for every case in
// ipv6RenderCases.
//
// This is the test that cannot drift, so every rendering branch belongs here
// rather than only in the hardcoded table in pkg/converters. Runs over both
// protocols because the two take different paths to driver.Value.
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
				"create table ip6_render_test (id UInt8, v IPv6, n Nullable(IPv6)) engine = MergeTree() order by id")
			require.NoError(t, err)
			for i, addr := range ipv6RenderCases {
				_, err = db.ExecContext(s.Ctx,
					fmt.Sprintf("insert into ip6_render_test values (%d, '%s', '%s')", i, addr, addr))
				require.NoError(t, err)
			}

			// server_text is a String column, so it bypasses the IP converter
			// entirely and carries ClickHouse's own formatting.
			rows, err := db.Query("select v, n, toString(v) as server_text from ip6_render_test order by id")
			require.NoError(t, err)

			frame, err := sqlutil.FrameFromRows(rows, int64(len(ipv6RenderCases)), converters.Converters...)
			require.NoError(t, err)
			require.Equal(t, 3, len(frame.Fields))
			require.Equal(t, len(ipv6RenderCases), frame.Rows())

			for i, addr := range ipv6RenderCases {
				serverText := frame.Fields[2].At(i)

				assert.Equal(t, serverText, frame.Fields[0].At(i),
					"IPv6 column holding %s should render exactly as ClickHouse renders it", addr)

				concrete, ok := frame.Fields[1].ConcreteAt(i)
				assert.True(t, ok)
				assert.Equal(t, serverText, concrete,
					"Nullable(IPv6) column holding %s should render exactly as ClickHouse renders it", addr)
			}

			// Sanity: the two forms that motivate type-driven rendering must not
			// have collapsed into the same text on the server side either.
			assert.Equal(t, "::ffff:1.2.3.4", frame.Fields[2].At(1),
				"sanity: ClickHouse should render the mapped address padded")
			assert.Equal(t, "::1.2.3.4", frame.Fields[2].At(3),
				"sanity: ClickHouse should render the IPv4-compatible address dotted-quad")

			_, err = db.ExecContext(s.Ctx, "drop table if exists ip6_render_test")
			require.NoError(t, err)
		})
	}
}
