package plugin

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Native block as a cluster writes it over HTTP. The fork reads HTTP blocks
// at revision 0, so there is no block info: the column and row counts, then
// each column's name, type and values.
type nativeColumn struct {
	name, typ string
	values    []byte
}

func nativeBlock(rows int, columns ...nativeColumn) []byte {
	b := binary.AppendUvarint(nil, uint64(len(columns)))
	b = binary.AppendUvarint(b, uint64(rows))
	for _, c := range columns {
		b = appendNativeString(b, c.name)
		b = appendNativeString(b, c.typ)
		b = append(b, c.values...)
	}
	return b
}

func appendNativeString(b []byte, s string) []byte {
	return append(binary.AppendUvarint(b, uint64(len(s))), s...)
}

func stringColumn(name, value string) nativeColumn {
	return nativeColumn{name: name, typ: "String", values: appendNativeString(nil, value)}
}

func numbersBlock(from, to uint64) []byte {
	var values []byte
	for n := from; n < to; n++ {
		values = binary.LittleEndian.AppendUint64(values, n)
	}
	return nativeBlock(int(to-from), nativeColumn{name: "number", typ: "UInt64", values: values})
}

// The answer to the fork's handshake, `SELECT displayName(), version(),
// revision(), timezone()`.
var helloBlock = nativeBlock(1,
	stringColumn("displayName()", "stream-test"),
	stringColumn("version()", "25.8.1.1"),
	nativeColumn{name: "revision()", typ: "UInt32", values: binary.LittleEndian.AppendUint32(nil, 54523)},
	stringColumn("timezone()", "UTC"),
)

// The answer to the fork's ping, `SELECT 1`: it checks the column's name.
var pingBlock = nativeBlock(1, nativeColumn{name: "1", typ: "UInt8", values: []byte{1}})

// clusterAnswering answers the handshake and the ping, and hands a query on
// numbers() to answer.
func clusterAnswering(t *testing.T, answer func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		query := string(raw) + r.URL.RawQuery
		if strings.Contains(query, "numbers(") {
			answer(w)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		if strings.Contains(query, "displayName()") {
			_, _ = w.Write(helloBlock)
			return
		}
		_, _ = w.Write(pingBlock)
	}))
	t.Cleanup(server.Close)
	return server
}

// streamingCluster streams body to the query in one chunk and ends the
// response with end.
func streamingCluster(t *testing.T, body []byte, end func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	return clusterAnswering(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
		end(w)
	})
}

// What a proxy or a query head that gives up mid-answer does: the chunks
// already sent stay sent, and the connection goes away without the chunked
// encoding's terminating chunk.
func dropConnection(w http.ResponseWriter) {
	w.(http.Flusher).Flush()
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		panic(err)
	}
	_ = conn.Close()
}

// A streamed answer carries its status before its rows, so a query that fails
// partway can only say so by how the stream ends. A dropped connection must
// stay an error through the fork and the transport chain, whatever the fork's
// exception detection does with the bytes it captured; only a stream that the
// server ended itself is a complete answer.
func TestACutStreamIsAnError(t *testing.T) {
	first, second := numbersBlock(0, 1000), numbersBlock(1000, 2000)
	cases := map[string]struct {
		body     []byte
		end      func(http.ResponseWriter)
		wantRows int
	}{
		"a stream the server ended is complete": {
			body: append(append([]byte{}, first...), second...), end: func(http.ResponseWriter) {}, wantRows: 2000,
		},
		"a connection dropped between blocks": {body: first, end: dropConnection},
		"a connection dropped inside a block": {
			body: append(append([]byte{}, first...), second[:len(second)/2]...), end: dropConnection,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster := streamingCluster(t, c.body, c.end)
			db, err := NewHydrolix().Connect(context.Background(), instanceSettingsFor(t, cluster.Listener.Addr().String(), false, "http"), nil)
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			rows, err := db.QueryContext(context.Background(), "SELECT number FROM numbers(2000)")
			require.NoError(t, err)
			read := 0
			for rows.Next() {
				read++
			}
			err = rows.Err()
			_ = rows.Close()

			if c.wantRows > 0 {
				require.NoError(t, err)
				assert.Equal(t, c.wantRows, read)
				return
			}
			require.Error(t, err, "%d rows read and the stream reported as complete", read)
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		})
	}
}
