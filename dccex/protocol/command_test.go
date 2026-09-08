package protocol_test

import (
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"testing"
)

func TestCommands(t *testing.T) {
	type build func() (string, error)
	cases := []struct {
		want  string
		build build
	}{
		{"<t 3>", func() (string, error) { return p.EncodeLocoRequest(3) }},
		{"<t 3 -1 1>", func() (string, error) { return p.EncodeThrottle(3, -1, 1) }},
		{"<t 10293 126 0>", func() (string, error) { return p.EncodeThrottle(10293, 126, 0) }},
		{"<t 1 0 1>", func() (string, error) { return p.EncodeThrottle(1, 0, 1) }},
		{"<F 3 0 1>", func() (string, error) { return p.EncodeFunction(3, 0, 1) }},
		{"<F 3 68 0>", func() (string, error) { return p.EncodeFunction(3, 68, 0) }},
		{"<W 10293>", func() (string, error) { return p.EncodeWriteAddress(10293) }},
		{"<R 1024>", func() (string, error) { return p.EncodeReadCV(1024) }},
		{"<W 1 0>", func() (string, error) { return p.EncodeWriteCV(1, 0) }},
		{"<W 1024 255>", func() (string, error) { return p.EncodeWriteCV(1024, 255) }},
		{"<w 3 29 255>", func() (string, error) { return p.EncodeProgramOnMain(3, 29, 255) }},
		{"<1>", func() (string, error) { return p.EncodePower(true, p.All) }},
		{"<0>", func() (string, error) { return p.EncodePower(false, p.All) }},
		{"<1 MAIN>", func() (string, error) { return p.EncodePower(true, p.Main) }},
		{"<0 MAIN>", func() (string, error) { return p.EncodePower(false, p.Main) }},
		{"<1 PROG>", func() (string, error) { return p.EncodePower(true, p.Prog) }},
		{"<0 PROG>", func() (string, error) { return p.EncodePower(false, p.Prog) }},
	}
	for _, tt := range cases {
		got, err := tt.build()
		if err != nil || got != tt.want {
			t.Errorf("want %s got %s err=%v", tt.want, got, err)
		}
	}
	if p.EncodeStatus() != "<s>" || p.EncodeEmergencyStop() != "<!>" || p.EncodeCurrentQuery() != "<c>" || p.EncodeReadAddress() != "<R>" {
		t.Fatal("fixed commands")
	}
}
func reject(t *testing.T, cmd string, err error) {
	t.Helper()
	if err == nil || cmd != "" {
		t.Fatalf("invalid input produced %q err=%v", cmd, err)
	}
}
func TestCommandBounds(t *testing.T) {
	for _, cab := range []int{-1, 0, 10294} {
		c, e := p.EncodeLocoRequest(cab)
		reject(t, c, e)
		c, e = p.EncodeThrottle(cab, 0, 1)
		reject(t, c, e)
		c, e = p.EncodeFunction(cab, 1, 1)
		reject(t, c, e)
		c, e = p.EncodeWriteAddress(cab)
		reject(t, c, e)
		c, e = p.EncodeProgramOnMain(cab, 1, 0)
		reject(t, c, e)
	}
	for _, v := range []int{-2, 127, 256} {
		c, e := p.EncodeThrottle(3, v, 1)
		reject(t, c, e)
	}
	for _, v := range []int{-1, 2} {
		c, e := p.EncodeThrottle(3, 0, v)
		reject(t, c, e)
		c, e = p.EncodeFunction(3, 0, v)
		reject(t, c, e)
	}
	for _, v := range []int{-1, 69} {
		c, e := p.EncodeFunction(3, v, 1)
		reject(t, c, e)
	}
	for _, v := range []int{0, 1025} {
		c, e := p.EncodeReadCV(v)
		reject(t, c, e)
		c, e = p.EncodeWriteCV(v, 0)
		reject(t, c, e)
		c, e = p.EncodeProgramOnMain(3, v, 0)
		reject(t, c, e)
	}
	for _, v := range []int{-1, 256} {
		c, e := p.EncodeWriteCV(1, v)
		reject(t, c, e)
		c, e = p.EncodeProgramOnMain(3, 1, v)
		reject(t, c, e)
	}
	for _, v := range []p.Track{"", "JOIN", "main", "MAIN><1", "MAIN\n"} {
		c, e := p.EncodePower(true, v)
		reject(t, c, e)
	}
}
