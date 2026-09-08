package protocol_test

import (
	"errors"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"reflect"
	"strings"
	"testing"
)

func TestFramerChunkBoundaries(t *testing.T) {
	input := "noise<p1 MAIN><l 3 -1 129 4294967295><v 29 -1>junk"
	want := []p.FrameResult{{Frame: "<p1 MAIN>"}, {Frame: "<l 3 -1 129 4294967295>"}, {Frame: "<v 29 -1>"}}
	for cut := 0; cut <= len(input); cut++ {
		var f p.Framer
		got := f.Feed([]byte(input[:cut]))
		got = append(got, f.Feed([]byte(input[cut:]))...)
		if !reflect.DeepEqual(got, want) || f.Pending() != "" {
			t.Fatalf("cut %d: %+v pending %q", cut, got, f.Pending())
		}
	}
	var f p.Framer
	var got []p.FrameResult
	for _, b := range []byte(input) {
		got = append(got, f.Feed([]byte{b})...)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bytewise: %+v", got)
	}
}
func TestFramerLimitsAndRecovery(t *testing.T) {
	valid := "<" + strings.Repeat("x", p.MaxFrameSize-2) + ">"
	for _, cut := range []int{0, 1, p.MaxFrameSize - 1, p.MaxFrameSize} {
		var f p.Framer
		got := f.Feed([]byte(valid[:cut]))
		got = append(got, f.Feed([]byte(valid[cut:]))...)
		if len(got) != 1 || got[0].Frame != valid || got[0].Err != nil {
			t.Fatalf("exact boundary cut %d: %+v", cut, got)
		}
	}
	oversized := "<" + strings.Repeat("x", p.MaxFrameSize) + ">discard<p0>"
	for _, cut := range []int{0, 1, p.MaxFrameSize - 1, p.MaxFrameSize} {
		var f p.Framer
		got := f.Feed([]byte(oversized[:cut]))
		if len(f.Pending()) > p.MaxFrameSize {
			t.Fatal("unbounded pending")
		}
		got = append(got, f.Feed([]byte(oversized[cut:]))...)
		if len(got) != 2 || !errors.Is(got[0].Err, p.ErrFrameTooLarge) || got[1].Frame != "<p0>" || f.Pending() != "" {
			t.Fatalf("oversize cut %d: %+v", cut, got)
		}
	}
	var f p.Framer
	got := f.Feed([]byte(strings.Repeat("<p1>", 10000)))
	if len(got) != 10000 || f.Pending() != "" {
		t.Fatal("large read of small frames lost data")
	}
}
func TestFramerResynchronizesAndOwnsData(t *testing.T) {
	var f p.Framer
	got := f.Feed([]byte("><broken<p1><par"))
	if len(got) != 2 || !errors.Is(got[0].Err, p.ErrMalformed) || got[1].Frame != "<p1>" || f.Pending() != "<par" {
		t.Fatalf("%+v %q", got, f.Pending())
	}
	saved := f.Pending()
	f.Feed([]byte("tial>"))
	if saved != "<par" || got[1].Frame != "<p1>" {
		t.Fatal("previous outputs mutated")
	}
	f.Feed([]byte("<unfinished"))
	if !errors.Is(f.End(), p.ErrTruncated) || f.Pending() != "" {
		t.Fatal("end must report and clear partial")
	}
	if f.End() != nil {
		t.Fatal("empty end")
	}
	f.Feed([]byte("<pending"))
	f.Reset()
	if f.Pending() != "" {
		t.Fatal("reset")
	}
}
func TestDecoderPreservesOrderAcrossErrors(t *testing.T) {
	var d p.Decoder
	got := d.Feed([]byte("<p1><v 29 bad><r 300><X><iDCC-EX V-5.0><p0 MAIN>"))
	if len(got) != 6 {
		t.Fatalf("%+v", got)
	}
	if _, ok := got[0].Event.(p.TrackPower); !ok {
		t.Fatal("power missing")
	}
	if !errors.Is(got[1].Err, p.ErrMalformed) || got[1].Event != nil {
		t.Fatal("missing parse error")
	}
	if e, ok := got[2].Event.(p.AddressResult); !ok || e.Address != 300 {
		t.Fatal("address misclassified")
	}
	if _, ok := got[3].Event.(p.Unknown); !ok {
		t.Fatal("unknown missing")
	}
	if e, ok := got[4].Event.(p.VersionInfo); !ok || e.Text != "DCC-EX V-5.0" {
		t.Fatal("version missing")
	}
	if e := got[5].Event.(p.TrackPower); e.Track != "MAIN" || e.State != p.Off {
		t.Fatal("power off track")
	}
	d.Feed([]byte("<partial"))
	if d.Pending() != "<partial" || !errors.Is(d.End(), p.ErrTruncated) {
		t.Fatal("partial lost")
	}
	d.Feed([]byte("<unfinished"))
	d.Reset()
	if d.Pending() != "" || d.End() != nil {
		t.Fatal("decoder reset")
	}
	got = d.Feed([]byte("<" + strings.Repeat("x", p.MaxFrameSize) + "><p0>"))
	if len(got) != 2 || !errors.Is(got[0].Err, p.ErrFrameTooLarge) || got[1].Err != nil {
		t.Fatal("decoder overflow recovery")
	}
}
