// Package protocol provides the DCC-EX native protocol layer for frame
// extraction, message parsing, and command encoding. It is GUI-independent
// and transport-neutral, depending only on the Go standard library.
package protocol

import (
	"strings"
)

// FrameExtractor accumulates bytes and extracts complete angle-bracket
// delimited frames from an arbitrary byte stream. It handles partial frames,
// multiple frames per read, garbage bytes, and bounded runaway buffers.
// The buffer is bounded at MaxBufferSize; excess data returns overflow error.
type FrameExtractor struct {
	buf []byte // accumulated buffer (bounded at maxBufferSize)
}

// MaxBufferSize specifies the maximum buffer size for FrameExtractor.
// Frames exceeding this limit cause overrun; excess bytes are discarded with an error.
const MaxBufferSize = 4096

// NewFrameExtractor creates a new FrameExtractor ready to receive bytes.
func NewFrameExtractor() *FrameExtractor {
	return &FrameExtractor{
		buf: make([]byte, 0, 1024), // start with reasonable size
	}
}

// overflowOverflowError is returned when the buffer would exceed MaxBufferSize.
var overflowOverflowError = strings.NewReader("buffer overflow")

// Write appends bytes to the internal buffer. Complete frames are extracted
// and returned via ExtractedFrames(). This implements the incremental parse
// pattern where multiple frames may appear in a single read, or a read may
// contain only part of a frame waiting for more data. The buffer is bounded
// at MaxBufferSize (4096 bytes). On overflow: excess bytes beyond current capacity
// plus partial tail are discarded; an error indicates overflow occurred.
// Returns the discarded excess on overflow, nil otherwise.
func (e *FrameExtractor) Write(b []byte) ([]byte, error) {
	const maxBufferSize = MaxBufferSize

	needed := len(e.buf) + len(b)
	if needed > maxBufferSize {
		// Overflow: preserve what we can fit, discard excess
		keep := maxBufferSize
		discard := b[len(e.buf):] // bytes beyond current capacity
		// Fit existing buffer content up to max size
		e.buf = make([]byte, keep)
		copy(e.buf, e.buf[:maxBufferSize])
		// Add partial new data that fits
		if len(discard) > 0 {
			e.buf = append(e.buf, discard[:maxBufferSize-len(e.buf)]...)
		}
		return discard, overflowOverflowError // overflow detected via returned error
	}

	e.buf = append(e.buf, b...)
	return nil, nil
}

// ExtractedFrames returns all complete frames currently buffered, and any
// trailing bytes that do not form a complete frame. The frames are returned
// as byte slices containing the body only (without angle brackets). The buffer
// is reset after extraction of all complete frames; only incomplete trailing
// data remains in the buffer for future writes. Partial frames are retained up to
// MaxBufferSize when overflow occurs and discarded excess bytes are returned on Write().
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
