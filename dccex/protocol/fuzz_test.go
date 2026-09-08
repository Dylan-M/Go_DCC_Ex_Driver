package protocol_test

import (
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"reflect"
	"testing"
)

func FuzzFramer(f *testing.F) {
	for _, s := range []string{"", "<p1>", "garbage<l 3 -1 129 0><r 29 -1>", "<<<p0>", "<unfinished"} {
		f.Add([]byte(s), uint16(3))
	}
	f.Fuzz(func(t *testing.T, data []byte, width uint16) {
		var whole, split p.Framer
		a := whole.Feed(data)
		var b []p.FrameResult
		step := int(width)%256 + 1
		for start := 0; start < len(data); start += step {
			end := start + step
			if end > len(data) {
				end = len(data)
			}
			b = append(b, split.Feed(data[start:end])...)
			if len(split.Pending()) > p.MaxFrameSize {
				t.Fatal("unbounded buffer")
			}
		}
		if !reflect.DeepEqual(a, b) || whole.Pending() != split.Pending() {
			t.Fatal("chunking changes results")
		}
		for _, r := range a {
			if (r.Frame == "") == (r.Err == nil) {
				t.Fatal("invalid result shape")
			}
		}
		recovery := split.Feed([]byte("<p0>"))
		if len(recovery) == 0 || recovery[len(recovery)-1].Frame != "<p0>" {
			t.Fatal("failed to recover")
		}
	})
}
func FuzzParse(f *testing.F) {
	for _, s := range []string{"<p0 MAIN>", "<l 3 -1 255 4294967295>", "<v 29 -1>", "<r 300>", "<iDCC-EX V-5>", "<c \"CurrentMAIN\" 1 C \"Milli\" \"0\" 500 \"1\" 400>", "<>"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		e, err := p.Parse(s)
		if (e == nil) == (err == nil) {
			t.Fatal("must return exactly event or error")
		}
		if e != nil && e.RawFrame() != s {
			t.Fatal("raw frame changed")
		}
	})
}
