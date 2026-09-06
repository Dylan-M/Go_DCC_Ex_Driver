// Package protocol provides the DCC-EX native protocol layer for frame
// extraction, message parsing, and command encoding. It is GUI-independent
// and transport-neutral, depending only on the Go standard library.
package protocol

// Protocol handles complete DCC-EX native protocol frame extraction and
// message parsing. Use NewProtocol() to create an instance for extracting
// and parsing frames from a byte stream.
type Protocol struct {
	extractor *FrameExtractor
}

// NewProtocol creates a new Protocol instance ready to extract frames from
// an incoming byte stream (TCP or serial). The extractor maintains internal
// state: it accumulates bytes, extracts complete frames, and retains partial
// data for the next read.
func NewProtocol() *Protocol {
	return &Protocol{
		extractor: NewFrameExtractor(),
	}
}

// Append adds raw bytes to the protocol stream. The extractor will parse any
// complete frames it can find and retain partial frames for future Appends.
// Multiple frames may appear in a single append; all complete frames are
// extracted at once. When the buffer would overflow (exceed MaxBufferSize),
// excess bytes are discarded and an empty remainder is returned with nil error.
func (p *Protocol) Append(b []byte) (msgs []Message, remainder []byte) {
	p.extractor.Write(b)
	frames, rem := p.extractor.ExtractedFrames()

	msgs = make([]Message, 0, len(frames))
	for _, frame := range frames {
		msg := ParseMessage(frame)
		if msg != nil && msg.Opcode != "" {
			msgs = append(msgs, *msg)
		}
	}

	return msgs, rem
}

// Extract returns all complete messages currently buffered and any partial
// data that doesn't form a complete frame yet. The extractor maintains its
// buffer automatically between calls.
func (p *Protocol) AppendBytes(b []byte) ([]Message, []byte) {
	// For external use: this is equivalent to calling Append.
	return p.Append(b)
}

// Flush returns any remaining partial bytes that haven't formed a complete
// frame yet. Normally you don't need to call this; just append more data.
// The remainder returned by Append already includes partial frames for future
// processing.
func (p *Protocol) GetRemainder() []byte {
	return p.extractor.buf
}

// IsComplete reports whether there is complete frame data ready to extract.
func (p *Protocol) HasFrames() bool {
	lastClose := -1
	buf := p.extractor.buf
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] == '>' {
			lastClose = i
			break
		}
	}
	return lastClose >= 0
}
