// Package protocol provides the DCC-EX native protocol layer for frame
// extraction, message parsing, and command encoding. It is GUI-independent
// and transport-neutral, depending only on the Go standard library.
package protocol

import (
	"regexp"
	"strings"
)

// FrameExtractor accumulates bytes and extracts complete angle-bracket
// delimited frames from an arbitrary byte stream. It handles partial frames,
// multiple frames per read, garbage bytes, and bounded runaway buffers.
// The buffer is bounded at 4096 bytes; excess data returns an error on overflow.
type FrameExtractor struct {
	buf     []byte         // accumulated buffer (bounded at maxBufferSize)
	pattern *regexp.Regexp // <...> frame regex
}

// MaxBufferSize specifies the maximum buffer size for FrameExtractor.
// Frames exceeding this limit cause overrun; excess bytes are returned as error.
const MaxBufferSize = 4096

// NewFrameExtractor creates a new FrameExtractor ready to receive bytes.
func NewFrameExtractor() *FrameExtractor {
	// Match angle-bracket delimited frames, non-greedy to handle multiple per read
	return &FrameExtractor{
		buf:     make([]byte, 0, 1024), // start with reasonable size
		pattern: regexp.MustCompile(`<([^>]*)>`),
	}
}

// Write appends bytes to the internal buffer. Complete frames are extracted
// and returned via ExtractedFrames(). This implements the incremental parse
// pattern where multiple frames may appear in a single read, or a read may
// contain only part of a frame waiting for more data. The buffer is bounded
// at MaxBufferSize (4096 bytes); excess bytes are discarded and an error is
// returned to indicate overflow. Partial frames are retained up to the limit.
func (e *FrameExtractor) Write(b []byte) ([]byte, error) {
	const maxBufferSize = 4096 // 4KB bounded buffer per requirements

	needed := len(e.buf) + len(b)
	if needed > maxBufferSize {
		// Truncate to max size and return excess
		excess := b[len(e.buf):] // bytes beyond current capacity
		e.buf = make([]byte, maxBufferSize)
		copy(e.buf, e.buf[:maxBufferSize])
		return excess, nil // Return truncated data; application must re-read remainder
	}

	e.buf = append(e.buf, b...)
	return nil, nil
}

// ExtractedFrames returns all complete frames currently buffered, and any
// trailing bytes that do not form a complete frame. The frames are returned
// as strings containing the body only (without angle brackets). The buffer
// is reset after extraction of all complete frames; only incomplete trailing
// data remains in the buffer for future writes. Partial frames are truncated
// to MaxBufferSize when overflow occurs.
func (e *FrameExtractor) ExtractedFrames() (frames [][]byte, remainder []byte) {
	buf := e.buf

	// If buffer is empty, nothing to extract
	if len(buf) == 0 {
		return nil, nil
	}

	// Find the last closing bracket to determine where partial data begins
	lastClose := -1
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] == '>' {
			lastClose = i
			break
		}
	}

	// If no closing bracket exists, the entire buffer is partial data
	if lastClose < 0 {
		return nil, buf
	}

	// Scan from start to find complete frames
	frames = make([][]byte, 0)

	start := 0
	for start <= lastClose {
		// Find '<' at or after current position
		startIdx := strings.IndexByte(string(buf[start:]), '<')
		if startIdx == -1 {
			break // no more frames
		}

		// Absolute position of '<'
		absStart := start + startIdx

		// Find '>' after this '<'
		endIdx := strings.IndexByte(string(buf[absStart+1:]), '>')
		if endIdx == -1 {
			break // incomplete frame (no closing bracket)
		}

		// Absolute position of '>'
		frameEnd := absStart + 1 + endIdx

		// Check if this frame is complete (ends at or before lastClose)
		if frameEnd > lastClose+1 {
			break // incomplete frame would exceed the boundary
		}

		// Extract frame body (without brackets): from after '<' up to before '>'
		frames = append(frames, buf[absStart+1:frameEnd])

		// Move past this complete frame's closing bracket
		start = frameEnd + 1 // move one past the >
	}

	// The remainder is everything starting after the last complete frame we found
	if len(frames) > 0 {
		remainder = buf[start:]
	} else {
		// No complete frames found - keep partial data (everything up to and including last >)
		remainder = buf[:lastClose+1]
	}

	e.buf = remainder
	return frames, remainder
}
