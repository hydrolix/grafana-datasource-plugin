package plugin

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/pierrec/lz4/v4"
)

// statsPrefix is the marker HDX appends to the response body when
// hdx_query_streaming_result is enabled. The full pattern is
// "\nX-HDX-Query-Stats:{key=val ...}\n".
var statsPrefix = []byte("\nX-HDX-Query-Stats:")

// maxStatsLineSize is the maximum expected size of the trailing stats
// frame (CityHash128 16 + header 9 + LZ4-compressed stats text). We
// hold back this many bytes at the tail of the stream so the backward
// frame walk in findStatsIndex has the entire last frame in view at EOF.
// A typical stats line of 15–20 key=value pairs compresses to
// ~200–500 bytes, so 1KB is a comfortable margin.
const maxStatsLineSize = 1024

// LZ4 ClickHouse compressed-frame header layout:
//
//	[ 0..15] CityHash128 checksum
//	[16   ] compression method (0x82 LZ4 / 0x90 ZSTD / 0x02 NONE)
//	[17..20] compressed_size LE u32 (includes the 9-byte header)
//	[21..24] decompressed_size LE u32
//	[25..  ] payload
const (
	lz4FrameHeaderSize  = 25 // checksum + method + sizes
	lz4FrameMethodLZ4   = 0x82
	lz4FrameMethodZSTD  = 0x90
	lz4FrameMethodNone  = 0x02
	lz4MaxDecompressLen = 64 * 1024
)

// StatsStrippingReader wraps an io.ReadCloser and strips a trailing
// X-HDX-Query-Stats line from the stream. It uses a single flat buffer
// with pointer-based management — zero allocations during Read.
//
// Buffer layout during streaming:
//
//	[consumed | safe to return | trail (512) | free space]
//	^         ^                ^             ^           ^
//	0       start            trail          end       len(buf)
type StatsStrippingReader struct {
	r     io.ReadCloser
	buf   []byte // single flat buffer, allocated once
	start int    // first byte of unread data
	trail int    // boundary of data safe to return to caller
	end   int    // one past last byte of upstream data
	err   error  // terminal error from upstream
}

func NewStatsStrippingReader(r io.ReadCloser) *StatsStrippingReader {
	return &StatsStrippingReader{
		r:   r,
		buf: make([]byte, 32*1024),
	}
}

func (s *StatsStrippingReader) Read(p []byte) (int, error) {
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
		}
	}
}

func (s *StatsStrippingReader) Close() error {
	return s.r.Close()
}

// findStatsIndex returns the index within data where the trailing
// X-HDX-Query-Stats LZ4 frame begins, or len(data) if no such trailer
// is found. The Hydrolix server bug (when hdx_query_streaming_result=1
// and Native+LZ4 are used) appends a spurious LZ4-framed
// "\nX-HDX-Query-Stats:...\n" block as the last frame of the body.
//
// Approach B (see http_streaming_doc.md): walk backward from the end
// of data, looking for a candidate position p where data[p] is a valid
// frame method byte and the LE u32 immediately after equals the byte
// distance from p to the end (i.e. the compressed size matches the
// tail exactly). Then verify by LZ4-decompressing the payload's first
// few bytes and confirming they begin with "\nX-HDX-Query-Stats:". If
// any candidate verifies, return frame_start = p - 16. Otherwise pass
// through (return len(data)) — this also makes the eventual server
// fix a silent no-op for the client.
func findStatsIndex(data []byte) int {
	if len(data) < lz4FrameHeaderSize {
		return len(data)
	}

	// p points at the method byte; we need at least 9 bytes after it
	// for the header (method + compSize + decompSize) and 16 before
	// it for the CityHash128.
	for p := len(data) - 9; p >= 16; p-- {
		method := data[p]
		if method != lz4FrameMethodLZ4 &&
			method != lz4FrameMethodZSTD &&
			method != lz4FrameMethodNone {
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
	if frame[0] != lz4FrameMethodLZ4 {
		return false
	}
	decompSize := binary.LittleEndian.Uint32(frame[5:9])
	if decompSize < uint32(len(statsPrefix)) || decompSize > lz4MaxDecompressLen {
		return false
	}
	dst := make([]byte, decompSize)
	n, err := lz4.UncompressBlock(frame[9:], dst)
	if err != nil || n < len(statsPrefix) {
		return false
	}
	return bytes.HasPrefix(dst[:n], statsPrefix)
}
