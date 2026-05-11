package plugin

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/pierrec/lz4/v4"
)

// statsPrefix is the marker HDX appends to the response body when
// hdx_query_streaming_result is enabled. The full pattern is
// "\nX-HDX-Query-Stats:{key=val ...}\n".
var statsPrefix = []byte("\nX-HDX-Query-Stats:")

// maxStatsLineSize is the maximum expected size of the trailing stats
// data. For LZ4-framed bodies this is the full ClickHouse frame
// (CityHash128 16 + header 9 + payload); for plain-text bodies it is
// the raw stats line. We hold back this many bytes at the tail of the
// stream so findStatsIndex has the entire last frame/line in view at
// EOF. A typical stats line of 15–20 key=value pairs compresses to
// ~200–500 bytes, so 1KB is a comfortable margin in both modes.
const maxStatsLineSize = 1024

// ClickHouse compressed-frame header layout:
//
//	[ 0..15] CityHash128 checksum
//	[16   ] compression method (0x82 LZ4 / 0x90 ZSTD / 0x02 NONE)
//	[17..20] compressed_size LE u32 (includes the 9-byte header)
//	[21..24] decompressed_size LE u32
//	[25..  ] payload
const (
	chFrameHeaderSize  = 25 // checksum + method + sizes
	chMethodLZ4        = 0x82
	chMethodZSTD       = 0x90
	chMethodNone       = 0x02
	chMaxDecompressLen = 64 * 1024
)

// statsStrippingReader wraps an io.ReadCloser and strips a trailing
// X-HDX-Query-Stats line from the stream. It uses a single flat buffer
// with pointer-based management — zero allocations during Read.
//
// Buffer layout during streaming:
//
//	[consumed | safe to return | trail (512) | free space]
//	^         ^                ^             ^           ^
//	0       start            trail          end       len(buf)
type statsStrippingReader struct {
	r     io.ReadCloser
	buf   []byte // single flat buffer, allocated once
	start int    // first byte of unread data
	trail int    // boundary of data safe to return to caller
	end   int    // one past last byte of upstream data
	err   error  // terminal error from upstream
}

func newStatsStrippingReader(r io.ReadCloser) *statsStrippingReader {
	return &statsStrippingReader{
		r:   r,
		buf: make([]byte, 32*1024),
	}
}

func (s *statsStrippingReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		// Step 2: return safe data.
		if s.trail > s.start {
			n := copy(p, s.buf[s.start:s.trail])
			s.start += n
			return n, nil
		}

		// Step 3: all safe data drained and upstream is done.
		if s.err != nil {
			return 0, s.err
		}

		// Step 4: compact if free space is tight.
		if s.end > len(s.buf)-2*maxStatsLineSize {
			dataLen := s.end - s.start
			copy(s.buf, s.buf[s.start:s.end])
			s.trail -= s.start
			s.end = dataLen
			s.start = 0
		}

		// Step 5: read from upstream.
		n, err := s.r.Read(s.buf[s.end:])
		s.end += n
		s.trail = s.end - maxStatsLineSize
		if s.trail < s.start {
			s.trail = s.start
		}

		if err == io.EOF {
			s.err = err
			s.trail = s.start + findStatsIndex(s.buf[s.start:s.end])
		} else if err != nil {
			s.err = err
			s.trail = s.end
		} else if n == 0 {
			// Upstream returned (0, nil). io.Reader allows this, so
			// surface it to the caller instead of spinning — looping
			// here would peg the goroutine until the request times
			// out at a higher layer.
			return 0, nil
		}
	}
}

func (s *statsStrippingReader) Close() error {
	return s.r.Close()
}

// findStatsIndex returns the index within data where the trailing
// X-HDX-Query-Stats data begins, or len(data) if no trailer is found.
// The Hydrolix server bug (when hdx_query_streaming_result=1) appends
// a spurious "\nX-HDX-Query-Stats:...\n" block as the last frame of
// the body — LZ4-framed when the response is Native+LZ4, plain text
// when the format is non-binary.
//
// Frame walk runs first: a successful LZ4 decompression to the marker
// is a definitive match and avoids the false-positive risk of finding
// the literal prefix inside compressed payload bytes. Plain-text
// search runs as a fallback for non-LZ4 bodies.
func findStatsIndex(data []byte) int {
	if idx := findStatsFrameIndex(data); idx < len(data) {
		log.DefaultLogger.Debug("found lz4 frame of stats info", "pos", idx)
		return idx
	}
	idx := findStatsTextIndex(data)
	log.DefaultLogger.Debug("text frame of stats info or end of data", "pos", idx)
	return idx
}

// findStatsTextIndex searches for a trailing plain-text
// X-HDX-Query-Stats line. Returns len(data) if not found or if data
// follows the line (meaning the prefix is mid-stream content, not the
// terminal trailer).
func findStatsTextIndex(data []byte) int {
	idx := bytes.LastIndex(data, statsPrefix)
	if idx < 0 {
		return len(data)
	}

	after := data[idx+len(statsPrefix):]
	nlPos := bytes.IndexByte(after, '\n')
	if nlPos < 0 {
		// No trailing newline — stats line runs to the end.
		return idx
	}
	if nlPos == len(after)-1 {
		// Newline is the last byte — expected terminal pattern.
		return idx
	}
	// Bytes follow the stats line — not the trailer.
	return len(data)
}

// findStatsFrameIndex walks backward through data looking for a
// ClickHouse LZ4-compressed frame whose payload begins with the
// X-HDX-Query-Stats marker. Returns the frame start (including the
// 16-byte CityHash128 checksum) or len(data) if no match.
//
// Approach B (see http_streaming_doc.md): for each candidate position
// p (the method byte), accept only if data[p] is a valid frame method
// byte and the LE u32 compressed_size at p+1 equals the distance from
// p to the end (i.e. the compressed size matches the tail exactly).
// Verify by LZ4-decompressing the payload and confirming it begins
// with "\nX-HDX-Query-Stats:". On no match, fall through to
// len(data) — this also makes the eventual server fix a silent no-op
// for the client.
func findStatsFrameIndex(data []byte) int {
	if len(data) < chFrameHeaderSize {
		return len(data)
	}

	// p points at the method byte; we need at least 9 bytes after it
	// for the header (method + compSize + decompSize) and 16 before
	// it for the CityHash128.
	for p := len(data) - 9; p >= 16; p-- {
		method := data[p]
		if method != chMethodLZ4 &&
			method != chMethodZSTD &&
			method != chMethodNone {
			continue
		}
		compSize := binary.LittleEndian.Uint32(data[p+1 : p+5])
		if compSize != uint32(len(data)-p) {
			continue
		}
		if !verifyStatsFrame(data[p:]) {
			// Coincidental method byte / size match inside another
			// frame's payload — keep walking.
			continue
		}
		return p - 16
	}
	return len(data)
}

// verifyStatsFrame reports whether the given ClickHouse compressed
// frame (starting at the method byte) decompresses to a payload that
// begins with the X-HDX-Query-Stats marker.
//
// Only LZ4 (method 0x82) is verified — the streaming-mode bug is
// LZ4-specific, and refusing to strip ZSTD/NONE tails is the safe
// conservative behavior.
func verifyStatsFrame(frame []byte) bool {
	if len(frame) < 9 {
		return false
	}
	if frame[0] != chMethodLZ4 {
		return false
	}
	decompSize := binary.LittleEndian.Uint32(frame[5:9])
	if decompSize < uint32(len(statsPrefix)) || decompSize > chMaxDecompressLen {
		return false
	}
	dst := make([]byte, decompSize)
	n, err := lz4.UncompressBlock(frame[9:], dst)
	if err != nil || n < len(statsPrefix) {
		return false
	}
	return bytes.HasPrefix(dst[:n], statsPrefix)
}
