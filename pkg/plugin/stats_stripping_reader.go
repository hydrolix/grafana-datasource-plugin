package plugin

import (
	"bytes"
	"io"
)

// statsPrefix is the marker HDX appends to the response body when
// hdx_query_streaming_result is enabled. The full pattern is
// "\nX-HDX-Query-Stats:{key=val ...}\n".
var statsPrefix = []byte("\nX-HDX-Query-Stats:")

// maxStatsLineSize is the maximum expected size of the stats trailer.
// We hold back this many bytes at the tail of the stream so we can
// inspect them for the stats line before delivering to the caller.
// Must be larger than any stats line Hydrolix can produce; a typical
// line with 15–20 key=value pairs is ~400–600 bytes, but can grow
// with additional fields, so we use a generous margin.
const maxStatsLineSize = 2048

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
// X-HDX-Query-Stats line begins, or len(data) if no trailing stats
// are found. Only matches when the stats line is the last line —
// nothing meaningful follows except an optional trailing newline.
func findStatsIndex(data []byte) int {
	idx := bytes.LastIndex(data, statsPrefix)
	if idx < 0 {
		return len(data)
	}

	// Verify nothing meaningful follows the stats line.
	after := data[idx+len(statsPrefix):]
	nlPos := bytes.IndexByte(after, '\n')
	if nlPos < 0 {
		// No trailing newline — stats line runs to the end.
		return idx
	}
	if nlPos == len(after)-1 {
		// Newline is the last byte — expected pattern.
		return idx
	}
	// Data follows the stats line — not the trailer.
	return len(data)
}
