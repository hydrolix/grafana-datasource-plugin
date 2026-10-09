package plugin

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"testing"

	"github.com/pierrec/lz4/v4"
)

func TestFindStatsIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want int
	}{
		{
			name: "no stats line",
			data: "hello world",
			want: 11,
		},
		{
			name: "stats at end with trailing newline",
			data: "some data\nX-HDX-Query-Stats:exec_time=0 result_rows=1\n",
			want: 9,
		},
		{
			name: "stats at end without trailing newline",
			data: "some data\nX-HDX-Query-Stats:exec_time=0 result_rows=1",
			want: 9,
		},
		{
			name: "stats only",
			data: "\nX-HDX-Query-Stats:exec_time=0\n",
			want: 0,
		},
		{
			name: "data with newlines before stats",
			data: "row1\nrow2\nrow3\nX-HDX-Query-Stats:exec_time=0 head_rows_read=0 peer_rows_read=0\n",
			want: 14,
		},
		{
			name: "empty input",
			data: "",
			want: 0,
		},
		{
			name: "stats-like string in middle is not matched",
			data: "before\nX-HDX-Query-Stats:fake\nafter data here",
			want: len("before\nX-HDX-Query-Stats:fake\nafter data here"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findStatsIndex([]byte(tt.data))
			if got != tt.want {
				t.Errorf("findStatsIndex(%q) = %d, want %d", tt.data, got, tt.want)
			}
		})
	}
}

// smallReader returns data in small chunks to exercise the trailing buffer logic.
type smallReader struct {
	data []byte
	pos  int
	size int
}

func (r *smallReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := r.size
	if n > len(p) {
		n = len(p)
	}
	if r.pos+n > len(r.data) {
		n = len(r.data) - r.pos
	}
	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, io.EOF
	}
	return n, nil
}

func (r *smallReader) Close() error { return nil }

// errorAfterReader returns all data, then a non-EOF error.
type errorAfterReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errorAfterReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

func (r *errorAfterReader) Close() error { return nil }

// errorMidStreamReader returns data in two reads: first read returns
// some data, second read returns more data + error simultaneously.
type errorMidStreamReader struct {
	chunks [][]byte
	errs   []error
	idx    int
}

func (r *errorMidStreamReader) Read(p []byte) (int, error) {
	if r.idx >= len(r.chunks) {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[r.idx])
	err := r.errs[r.idx]
	r.idx++
	return n, err
}

func (r *errorMidStreamReader) Close() error { return nil }

// zeroProgressReader returns (0, nil) on the first call, then EOF.
// Used to verify the reader doesn't spin when upstream stalls without
// reporting an error (allowed by io.Reader contract).
type zeroProgressReader struct {
	called bool
}

func (r *zeroProgressReader) Read(p []byte) (int, error) {
	if !r.called {
		r.called = true
		return 0, nil
	}
	return 0, io.EOF
}

func (r *zeroProgressReader) Close() error { return nil }

func TestStatsStrippingReader(t *testing.T) {
	t.Parallel()

	statsLine := "\nX-HDX-Query-Stats:exec_time=0 head_rows_read=0 peer_rows_read=0 num_partitions=0 num_peers=0 result_rows=1 query_attempts=1 memory_usage=6306448\n"

	tests := []struct {
		name      string
		input     string
		want      string
		chunkSize int
	}{
		{
			name:      "large body with stats, large chunks",
			input:     "lots of clickhouse data here" + statsLine,
			want:      "lots of clickhouse data here",
			chunkSize: 4096,
		},
		{
			name:      "large body with stats, tiny chunks",
			input:     "lots of clickhouse data here" + statsLine,
			want:      "lots of clickhouse data here",
			chunkSize: 10,
		},
		{
			name:      "no stats line",
			input:     "just normal response data",
			want:      "just normal response data",
			chunkSize: 4096,
		},
		{
			name:      "body smaller than trail size with stats",
			input:     "small" + statsLine,
			want:      "small",
			chunkSize: 4096,
		},
		{
			name:      "empty body",
			input:     "",
			want:      "",
			chunkSize: 4096,
		},
		{
			name:      "only stats line",
			input:     statsLine,
			want:      "",
			chunkSize: 4096,
		},
		{
			name:      "body larger than trail with tiny reads",
			input:     string(bytes.Repeat([]byte("x"), 1024)) + statsLine,
			want:      string(bytes.Repeat([]byte("x"), 1024)),
			chunkSize: 7,
		},
		{
			name:      "single byte reads",
			input:     "abc" + statsLine,
			want:      "abc",
			chunkSize: 1,
		},
		{
			name:      "stats without trailing newline",
			input:     "data\nX-HDX-Query-Stats:exec_time=0",
			want:      "data",
			chunkSize: 4096,
		},
		{
			name:      "body exceeds 32KB buffer triggers compaction",
			input:     string(bytes.Repeat([]byte("A"), 40*1024)) + statsLine,
			want:      string(bytes.Repeat([]byte("A"), 40*1024)),
			chunkSize: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newStatsStrippingReader(&smallReader{
				data: []byte(tt.input),
				size: tt.chunkSize,
			})

			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestStatsStrippingReaderWithError(t *testing.T) {
	t.Parallel()

	statsLine := "\nX-HDX-Query-Stats:exec_time=0\n"

	t.Run("non-EOF error returns all data without stripping", func(t *testing.T) {
		t.Parallel()

		input := "good data" + statsLine
		r := newStatsStrippingReader(&errorAfterReader{
			data: []byte(input),
			err:  io.ErrUnexpectedEOF,
		})

		got, err := io.ReadAll(r)
		if err != io.ErrUnexpectedEOF {
			t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
		}
		if string(got) != input {
			t.Errorf("got %q, want %q", string(got), input)
		}
	})

	t.Run("data returned with error is not lost", func(t *testing.T) {
		t.Parallel()

		// Upstream returns data + error in the same Read call
		r := newStatsStrippingReader(&errorMidStreamReader{
			chunks: [][]byte{[]byte("hello"), []byte(" world")},
			errs:   []error{nil, io.ErrUnexpectedEOF},
		})

		got, err := io.ReadAll(r)
		if err != io.ErrUnexpectedEOF {
			t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
		}
		if string(got) != "hello world" {
			t.Errorf("got %q, want %q", string(got), "hello world")
		}
	})
}

// TestStatsStrippingReaderZeroProgress verifies that an upstream
// (0, nil) Read is forwarded to the caller and does not spin in
// place. Without the guard the goroutine would loop until a higher
// layer cancels the request.
func TestStatsStrippingReaderZeroProgress(t *testing.T) {
	t.Parallel()

	r := newStatsStrippingReader(&zeroProgressReader{})

	buf := make([]byte, 16)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("first Read: unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("first Read: got n=%d, want 0", n)
	}

	// Second Read should reach the EOF the upstream returns next.
	n, err = r.Read(buf)
	if err != io.EOF {
		t.Fatalf("second Read: expected io.EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("second Read: got n=%d, want 0", n)
	}
}

func TestStatsStrippingReaderClose(t *testing.T) {
	t.Parallel()

	inner := &smallReader{data: []byte("data"), size: 100}
	r := newStatsStrippingReader(inner)
	if err := r.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func gzipCompress(data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestDecompressThenStripReader(t *testing.T) {
	t.Parallel()

	statsLine := "\nX-HDX-Query-Stats:exec_time=0 head_rows_read=0 peer_rows_read=0 num_partitions=0 num_peers=0 result_rows=1 query_attempts=1 memory_usage=6306448\n"

	tests := []struct {
		name            string
		input           string
		want            string
		contentEncoding string
	}{
		{
			name:            "gzip with stats: decompress then strip",
			input:           "lots of clickhouse data here" + statsLine,
			want:            "lots of clickhouse data here",
			contentEncoding: "gzip",
		},
		{
			name:            "gzip without stats: decompress only",
			input:           "just normal response data",
			want:            "just normal response data",
			contentEncoding: "gzip",
		},
		{
			name:            "gzip empty body",
			input:           "",
			want:            "",
			contentEncoding: "gzip",
		},
		{
			name:            "gzip stats only: decompress then strip all",
			input:           statsLine,
			want:            "",
			contentEncoding: "gzip",
		},
		{
			name:            "non-gzip: plain strip",
			input:           "data" + statsLine,
			want:            "data",
			contentEncoding: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var body io.ReadCloser
			if tt.contentEncoding == "gzip" {
				body = io.NopCloser(bytes.NewReader(gzipCompress([]byte(tt.input))))
			} else {
				body = io.NopCloser(bytes.NewReader([]byte(tt.input)))
			}

			// Correct order: decompress first, then strip from decompressed stream.
			decompressed, err := newDecompressReader(body, tt.contentEncoding)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			r := newStatsStrippingReader(decompressed)
			defer r.Close()

			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("unexpected read error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestDecompressReader(t *testing.T) {
	t.Parallel()

	t.Run("gzip decompresses correctly", func(t *testing.T) {
		t.Parallel()
		input := "hello decompressed world"
		body := io.NopCloser(bytes.NewReader(gzipCompress([]byte(input))))

		r, err := newDecompressReader(body, "gzip")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer r.Close()

		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		if string(got) != input {
			t.Errorf("got %q, want %q", string(got), input)
		}
	})

	t.Run("non-gzip returns body as-is", func(t *testing.T) {
		t.Parallel()
		input := "plain text body"
		body := io.NopCloser(bytes.NewReader([]byte(input)))

		r, err := newDecompressReader(body, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer r.Close()

		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		if string(got) != input {
			t.Errorf("got %q, want %q", string(got), input)
		}
	})

	t.Run("close propagates", func(t *testing.T) {
		t.Parallel()
		body := io.NopCloser(bytes.NewReader(gzipCompress([]byte("data"))))

		r, err := newDecompressReader(body, "gzip")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := r.Close(); err != nil {
			t.Fatalf("unexpected close error: %v", err)
		}
	})

	t.Run("gzip with invalid data returns error", func(t *testing.T) {
		t.Parallel()
		body := io.NopCloser(bytes.NewReader([]byte("not gzip data at all")))

		r, err := newDecompressReader(body, "gzip")
		if err == nil {
			_ = r.Close()
			t.Fatal("expected error for invalid gzip, got nil")
		}
		if r != nil {
			t.Fatalf("expected nil reader on error, got %T", r)
		}
	})
}

// buildCHLZ4Frame builds a ClickHouse LZ4-compressed frame from plain text:
// [16-byte zero checksum] [method=0x82] [compSize LE u32] [decompSize LE u32] [LZ4 payload]
// compSize includes the 9-byte header (method + compSize + decompSize).
func buildCHLZ4Frame(text []byte) []byte {
	hashTable := make([]int, 1<<16) // accepted but ignored by lz4 v4 — kept for signature compatibility
	compressed := make([]byte, lz4.CompressBlockBound(len(text)))
	n, err := lz4.CompressBlock(text, compressed, hashTable)
	if err != nil || n == 0 {
		// Incompressible data — use a larger output buffer.
		compressed = make([]byte, len(text)*2+256)
		n, err = lz4.CompressBlock(text, compressed, hashTable)
		if err != nil {
			panic(err)
		}
		if n == 0 {
			panic("lz4: data incompressible even with larger buffer")
		}
	}
	compressed = compressed[:n]

	// header: method(1) + compSize(4) + decompSize(4) = 9 bytes
	compSize := uint32(9 + len(compressed))
	decompSize := uint32(len(text))

	var buf bytes.Buffer
	buf.Write(make([]byte, 16))                           // CityHash128 checksum (zeroed for tests)
	buf.WriteByte(chMethodLZ4)                            // method
	_ = binary.Write(&buf, binary.LittleEndian, compSize) // compressed size (includes 9-byte header)
	_ = binary.Write(&buf, binary.LittleEndian, decompSize)
	buf.Write(compressed)

	return buf.Bytes()
}

func TestFindStatsFrameIndex(t *testing.T) {
	t.Parallel()

	// Use the full-length stats text so LZ4 can compress it (short
	// texts are incompressible and CompressBlock returns 0).
	statsText := []byte("\nX-HDX-Query-Stats:exec_time=0 head_rows_read=0 peer_rows_read=0 num_partitions=0 num_peers=0 result_rows=1 query_attempts=1 memory_usage=6306448\n")

	tests := []struct {
		name string
		data []byte
		want int // expected return from findStatsFrameIndex
	}{
		{
			name: "LZ4-framed stats at tail",
			data: func() []byte {
				prefix := []byte("some data frames")
				return append(prefix, buildCHLZ4Frame(statsText)...)
			}(),
			want: len("some data frames"),
		},
		{
			name: "LZ4-framed non-stats at tail: no match",
			data: func() []byte {
				prefix := []byte("some data")
				// Use long enough text so LZ4 can compress it.
				nonStats := bytes.Repeat([]byte("this is not stats data "), 10)
				return append(prefix, buildCHLZ4Frame(nonStats)...)
			}(),
			want: -1, // sentinel, replaced below
		},
		{
			name: "data shorter than frame header",
			data: []byte("short"),
			want: -1,
		},
		{
			name: "two LZ4 frames, stats in last",
			data: func() []byte {
				regularData := bytes.Repeat([]byte("regular data block content "), 10)
				frame1 := buildCHLZ4Frame(regularData)
				frame2 := buildCHLZ4Frame(statsText)
				return append(frame1, frame2...)
			}(),
			want: len(buildCHLZ4Frame(bytes.Repeat([]byte("regular data block content "), 10))),
		},
		{
			name: "empty input",
			data: []byte{},
			want: -1,
		},
	}

	for i, tt := range tests {
		// Sentinel -1 means "expect len(data)" (no match).
		if tt.want == -1 {
			tests[i].want = len(tt.data)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findStatsFrameIndex(tt.data)
			if got != tt.want {
				t.Errorf("findStatsFrameIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFindStatsIndexWithLZ4Frame(t *testing.T) {
	t.Parallel()

	statsText := []byte("\nX-HDX-Query-Stats:exec_time=0 head_rows_read=0 peer_rows_read=0 num_partitions=0 num_peers=0 result_rows=1 query_attempts=1 memory_usage=6306448\n")

	t.Run("findStatsIndex finds LZ4-framed stats", func(t *testing.T) {
		t.Parallel()
		prefix := bytes.Repeat([]byte{0xAB}, 100)
		frame := buildCHLZ4Frame(statsText)
		data := append(prefix, frame...)

		got := findStatsIndex(data)
		if got != len(prefix) {
			t.Errorf("findStatsIndex() = %d, want %d", got, len(prefix))
		}
	})

	t.Run("findStatsIndex prefers plain text over LZ4", func(t *testing.T) {
		t.Parallel()
		data := []byte("data\nX-HDX-Query-Stats:exec_time=0\n")
		got := findStatsIndex(data)
		if got != 4 {
			t.Errorf("findStatsIndex() = %d, want 4", got)
		}
	})
}

func TestStatsStrippingReaderWithLZ4Frame(t *testing.T) {
	t.Parallel()

	statsText := []byte("\nX-HDX-Query-Stats:exec_time=0 head_rows_read=0 peer_rows_read=0 num_partitions=0 num_peers=0 result_rows=1 query_attempts=1 memory_usage=6306448\n")
	realData := bytes.Repeat([]byte{0xDD}, 200)
	statsFrame := buildCHLZ4Frame(statsText)
	input := append(realData, statsFrame...)

	r := newStatsStrippingReader(&smallReader{
		data: input,
		size: 4096,
	})

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, realData) {
		t.Errorf("got %d bytes, want %d bytes (real data without stats frame)", len(got), len(realData))
	}
}
