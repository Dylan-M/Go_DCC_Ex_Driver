package protocol_test

import (
	"fmt"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"reflect"
	"strings"
	"testing"
)

func TestParseKnownMessages(t *testing.T) {
	cases := []struct {
		frame string
		want  p.Event
	}{
		{"<l 3 -1 129 4294967295>", p.LocoState{Cab: 3, Reg: -1, SpeedByte: 129, Speed: 0, Direction: 1, Emergency: true, FunctionMask: 4294967295}},
		{"<l 10293 2 255 1>", p.LocoState{Cab: 10293, Reg: 2, SpeedByte: 255, Speed: 126, Direction: 1, FunctionMask: 1}},
		{"<p0 MAIN>", p.TrackPower{State: p.Off, Track: "MAIN"}},
		{"<p1>", p.TrackPower{State: p.On, Track: "ALL"}},
		{"<p2 PROG>", p.TrackPower{State: p.Overload, Track: "PROG"}},
		{"<p1 JOIN>", p.TrackPower{State: p.On, Track: "JOIN"}},
		{"<c 12>", p.CurrentInfo{CurrentMA: 12}},
		{"<c 15 400 600>", p.CurrentInfo{CurrentMA: 15, MaxMA: 400, TripMA: 600, HasLimits: true}},
		{"<c CurrentMAIN 0 C Milli 0 1497 1 1497>", p.CurrentInfo{CurrentMA: 0, MaxMA: 1497, TripMA: 1497, HasLimits: true}},
		{"<c \"CurrentMAIN\" 15 C \"Milli\" \"0\" 400 \"1\" 600>", p.CurrentInfo{CurrentMA: 15, MaxMA: 400, TripMA: 600, HasLimits: true}},
		{"<c \"CurrentMAIN\" 0 C \"Milli\" \"0\" 0 \"1\" 0>", p.CurrentInfo{HasLimits: true}},
		{"<v 29 -1>", p.CVResult{Operation: p.Read, CV: 29, Value: -1}},
		{"<r 1024 255>", p.CVResult{Operation: p.Write, CV: 1024, Value: 255}},
		{"<r 29 -1>", p.CVResult{Operation: p.Write, CV: 29, Value: -1}},
		{"<r 300>", p.AddressResult{Operation: p.Read, Address: 300}},
		{"<r -1>", p.AddressResult{Operation: p.Read, Address: -1}},
		{"<w 10293>", p.AddressResult{Operation: p.Write, Address: 10293}},
		{"<w -1>", p.AddressResult{Operation: p.Write, Address: -1}},
		{"<iDCC-EX V-5.0 / ESP32>", p.VersionInfo{Text: "DCC-EX V-5.0 / ESP32"}},
		{"<X>", p.Unknown{}},
		{"<jR 3 4>", p.Unknown{}},
	}
	for _, tt := range cases {
		t.Run(tt.frame, func(t *testing.T) {
			got, err := p.Parse(tt.frame)
			if err != nil {
				t.Fatal(err)
			}
			// Populate only the raw envelope in the independent expected payload.
			v := reflect.New(reflect.TypeOf(tt.want))
			v.Elem().Set(reflect.ValueOf(tt.want))
			v.Elem().FieldByName("Message").Set(reflect.ValueOf(p.Message{Raw: tt.frame}))
			if !reflect.DeepEqual(got, v.Elem().Interface()) {
				t.Fatalf("got %#v want %#v", got, v.Elem().Interface())
			}
			if got.RawFrame() != tt.frame {
				t.Fatal("raw frame lost")
			}
		})
	}
}
func TestSpeedByteExhaustive(t *testing.T) {
	// Golden DCC speed anchors, then all 256 bytes against the defined formula.
	for raw := 0; raw < 256; raw++ {
		e, err := p.Parse(fmt.Sprintf("<l 3 0 %d 0>", raw))
		if err != nil {
			t.Fatal(err)
		}
		s := e.(p.LocoState)
		expected := raw%128 - 1
		if expected < 0 {
			expected = 0
		}
		if s.Speed != expected || s.Direction != raw/128 || s.Emergency != (raw%128 == 1) {
			t.Fatalf("raw %d: %+v", raw, s)
		}
	}
}
func TestParseWhitespace(t *testing.T) {
	e, err := p.Parse("<  l\t3  -1\n130\r0   >")
	if err != nil {
		t.Fatal(err)
	}
	s := e.(p.LocoState)
	if s.Cab != 3 || s.Speed != 1 || s.Direction != 1 {
		t.Fatalf("%+v", s)
	}
}
func TestRejectMalformedKnownMessages(t *testing.T) {
	frames := []string{"", "<>", "< >", "l 3 0 0 0", "<p1", "p1>", "<<p1>>", "<p1><p0>", "<p1\x00>", "<\xff>",
		"<l>", "<l 0 0 0 0>", "<l -1 0 0 0>", "<l 10294 0 0 0>", "<l 3 -2 0 0>", "<l 3 0 256 0>", "<l 3 0 -1 0>", "<l 3 0 0 -1>", "<l 3 0 0 4294967296>", "<l 3 0 0 0 extra>", "<l +3 0 0 0>", "<l x 0 0 0>", "<l 3 0 0 999999999999999999999999999>",
		"<p0 MAIN PROG>", "<p1 \"MAIN\">", "<v 29>", "<v 0 1>", "<v 1025 1>", "<v 29 -2>", "<v 29 256>", "<v 29 x>", "<v - 0>", "<v 29 1 extra>", "<r>", "<r 0>", "<r -2>", "<w 3 4>", "<w 10294>",
		"<c>", "<c 1 2>", "<c 1 bad 3>", "<c \"CurrentMAIN\" bad C \"Milli\" \"0\" 500 \"1\" 400>",
		"<c CurrentMAIN\" 0 C Milli 0 1497 1 1497>", "<c CurrentMAIN 0 C Milli 2 1497 1 1497>",
		"<c \"CurrentMAIN\" 1 C \"Milli\" \"0\" bad \"1\" 400>", "<c \"CurrentMAIN\" 1 C \"Milli\" \"0\" 500 \"1\" bad>",
		"<c \"CurrentMAIN\" 1 X \"Milli\" \"0\" 500 \"1\" 400>",
		"<" + strings.Repeat("x", p.MaxFrameSize) + ">"}
	for _, frame := range frames {
		t.Run(frame, func(t *testing.T) {
			e, err := p.Parse(frame)
			if err == nil || e != nil {
				t.Fatalf("accepted: %#v err=%v", e, err)
			}
		})
	}
}
func TestResultSuccess(t *testing.T) {
	for _, op := range []string{"v", "r"} {
		for _, value := range []int{-1, 0, 255} {
			e, err := p.Parse(fmt.Sprintf("<%s 29 %d>", op, value))
			if err != nil {
				t.Fatal(err)
			}
			if e.(p.CVResult).Success() != (value >= 0) {
				t.Fatal("CV success")
			}
		}
	}
	for _, op := range []string{"r", "w"} {
		for _, value := range []int{-1, 3, 10293} {
			e, err := p.Parse(fmt.Sprintf("<%s %d>", op, value))
			if err != nil {
				t.Fatal(err)
			}
			if e.(p.AddressResult).Success() != (value > 0) {
				t.Fatal("address success")
			}
		}
	}
}
