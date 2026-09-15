package throttle_test

import (
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"testing"
)

func TestIndependentTrackPower(t *testing.T) {
	c, _ := setup(t)
	for _, step := range []struct {
		frame      string
		main, prog p.PowerState
	}{
		{"<p0>", p.Off, p.Off},
		{"<p1 MAIN>", p.On, p.Off},
		{"<p1 PROG>", p.On, p.On},
		{"<p0 MAIN>", p.Off, p.On},
		{"<p1 JOIN>", p.On, p.On},
		{"<p2 MAIN>", p.Overload, p.On},
		{"<p0 PROG>", p.Overload, p.Off},
		{"<p0>", p.Off, p.Off},
	} {
		receive(t, c, step.frame)
		s := c.Snapshot()
		if s.MainPower != step.main || s.ProgPower != step.prog {
			t.Fatalf("%s: got %s/%s, want %s/%s", step.frame, s.MainPower, s.ProgPower, step.main, step.prog)
		}
	}
	c.Detach("disconnected")
	if s := c.Snapshot(); s.MainPower != "" || s.ProgPower != "" {
		t.Fatal("stale power after disconnect")
	}
}

func TestMappedTrackPowerAndOverload(t *testing.T) {
	c, _ := setup(t)
	// Deliberately not A=MAIN/B=PROG: roles come from firmware, not assumptions.
	for _, frame := range []string{"<= A PROG>", "<= B MAIN>", "<p1 A>", "<p1 B>", "<p1>", "<p1 MAIN>", "<p1 PROG>"} {
		receive(t, c, frame)
	}
	receive(t, c, "<p0 B>")
	if s := c.Snapshot(); s.MainPower != p.Off || s.ProgPower != p.On {
		t.Fatalf("independent off: %+v", s)
	}
	receive(t, c, "<p2 B>")
	receive(t, c, "<p0 A>")
	receive(t, c, "<p0>")
	if s := c.Snapshot(); s.MainPower != p.Overload || s.ProgPower != p.Off || !s.Overload {
		t.Fatalf("fault hidden: %+v", s)
	}
	receive(t, c, "<p0 B>")
	if c.Snapshot().Overload {
		t.Fatal("fault not cleared")
	}
	receive(t, c, "<= C MAIN>")
	receive(t, c, "<p1 C>")
	if c.Snapshot().MainPower != "MIXED" {
		t.Fatal("multiple MAIN outputs not aggregated")
	}
}
