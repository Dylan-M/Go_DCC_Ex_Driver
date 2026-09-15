package protocol_test

import (
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"testing"
)

func TestTrackManagerReplies(t *testing.T) {
	if p.EncodeTrackQuery() != "<=>" {
		t.Fatal("track query")
	}
	for _, frame := range []string{"<= A MAIN>", "<= B PROG>", "<= H DC 123>", "<= C MAIN_INV>"} {
		e, err := p.Parse(frame)
		if err != nil {
			t.Fatal(frame, err)
		}
		v, ok := e.(p.TrackMode)
		if !ok || v.Track == "" || v.Mode == "" || v.RawFrame() != frame {
			t.Fatalf("%s: %+v", frame, e)
		}
	}
	for _, frame := range []string{"<=>", "<= A>", "<= Z MAIN>", "<= AB MAIN>", "<= A main>", "<= A DC -1>", "<= A DC 10294>", "<= A DC 1 extra>"} {
		if _, err := p.Parse(frame); err == nil {
			t.Fatal("accepted", frame)
		}
	}
}
