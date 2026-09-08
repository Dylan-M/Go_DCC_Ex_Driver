package protocol

const MaxFrameSize = 4096 // Includes both brackets.

type FrameResult struct {
	Frame string
	Err   error
}

// Framer extracts complete frames. The zero value is ready for use by one
// goroutine. Garbage outside frames is ignored. A new '<' resynchronizes the
// stream; oversized frames are discarded until the next '<'.
type Framer struct{ buf []byte }

func (f *Framer) Feed(data []byte) []FrameResult {
	var out []FrameResult
	for _, b := range data {
		if b == '<' {
			if len(f.buf) > 0 {
				out = append(out, FrameResult{Err: ErrMalformed})
			}
			if f.buf == nil {
				f.buf = make([]byte, 0, MaxFrameSize)
			}
			f.buf = append(f.buf[:0], b)
			continue
		}
		if len(f.buf) == 0 {
			continue
		}
		if len(f.buf) == MaxFrameSize {
			out = append(out, FrameResult{Err: ErrFrameTooLarge})
			f.buf = f.buf[:0]
			continue
		}
		f.buf = append(f.buf, b)
		if b == '>' {
			out = append(out, FrameResult{Frame: string(f.buf)})
			f.buf = f.buf[:0]
		}
	}
	return out
}
func (f *Framer) Pending() string { return string(f.buf) }
func (f *Framer) Reset()          { f.buf = f.buf[:0] }
func (f *Framer) End() error {
	defer f.Reset()
	if len(f.buf) > 0 {
		return ErrTruncated
	}
	return nil
}
