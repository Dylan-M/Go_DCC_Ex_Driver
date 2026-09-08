// Package protocol implements the DCC-EX native messages used by DCC_Ex_Driver.
// It has no dependency on a GUI, transport, or application state.
package protocol

// Result contains one event or error. A bad frame does not prevent subsequent
// valid frames in the same chunk from being delivered.
type Result struct {
	Event Event
	Err   error
}

// Decoder combines framing with typed parsing. The zero value is ready for
// use. Each connection owns a decoder; it must not be used concurrently.
type Decoder struct{ framer Framer }

func (d *Decoder) Feed(data []byte) []Result {
	var out []Result
	for _, f := range d.framer.Feed(data) {
		if f.Err != nil {
			out = append(out, Result{Err: f.Err})
			continue
		}
		e, err := Parse(f.Frame)
		out = append(out, Result{Event: e, Err: err})
	}
	return out
}
func (d *Decoder) Pending() string { return d.framer.Pending() }
func (d *Decoder) Reset()          { d.framer.Reset() }
func (d *Decoder) End() error      { return d.framer.End() }
