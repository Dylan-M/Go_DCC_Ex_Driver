package throttle_test

import (
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"reflect"
	"testing"
	"time"
)

func TestMultipleThrottlesKeepCommandsAndStateIndependent(t *testing.T) {
	c, s := setup(t)
	if err := c.AddCab(7); err != nil {
		t.Fatal(err)
	}
	if c.AddCab(7) == nil || c.FocusCab(999) == nil {
		t.Fatal("invalid tab accepted")
	}
	c.WithCab(3, func(c *th.Controller) error { return c.MoveSpeed(25) })
	c.WithCab(7, func(c *th.Controller) error { return c.MoveSpeed(40) })
	if err := c.FocusCab(3); err != nil {
		t.Fatal(err)
	}
	s.commands = nil
	if err := c.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.commands, []string{"<t 3 25 1>", "<t 7 40 1>"}) {
		t.Fatal("misrouted pending speeds", s.commands)
	}
	receive(t, c, "<l 7 0 41 5>")
	state := c.Snapshot()
	if state.Cab != 3 || state.Speed != 25 || len(state.Throttles) != 2 || state.Throttles[1].Direction != 0 || !state.Throttles[1].Functions[2] {
		t.Fatalf("state bleed: %+v", state)
	}
	state.Throttles[0].Speed = 999
	if c.Snapshot().Throttles[0].Speed != 25 {
		t.Fatal("snapshot aliases controller")
	}
	s.commands = nil
	c.WithCab(7, func(c *th.Controller) error { return c.Function(1, true) })
	if !reflect.DeepEqual(s.commands, []string{"<F 7 1 1>"}) {
		t.Fatal("function routed to focused cab", s.commands)
	}
	if c.RemoveCab(7) == nil || c.ReplaceCab(7, 8) == nil {
		t.Fatal("moving throttle discarded")
	}
	c.WithCab(7, func(c *th.Controller) error { return c.MoveSpeed(55) })
	c.Emergency()
	s.commands = nil
	c.Tick(time.Now().Add(time.Second))
	if len(s.commands) != 0 {
		t.Fatal("emergency replayed pending speeds", s.commands)
	}
	for _, cab := range c.Snapshot().Throttles {
		if cab.Speed != 0 {
			t.Fatal("emergency did not cover all cabs")
		}
	}
	if err := c.RemoveCab(7); err != nil {
		t.Fatal(err)
	}
	if c.WithCab(7, func(c *th.Controller) error { return c.MoveSpeed(60) }) == nil {
		t.Fatal("closed throttle intent accepted")
	}
	if c.RemoveCab(3) == nil {
		t.Fatal("last throttle removed")
	}
	if err := c.ReplaceCab(3, 9); err != nil {
		t.Fatal(err)
	}
	if state := c.Snapshot(); state.Cab != 9 || len(state.Throttles) != 1 {
		t.Fatal("reassignment left extra throttle")
	}
}

func TestMultiThrottleDisconnectCancelsEveryQueue(t *testing.T) {
	c, _ := setup(t)
	c.AddCab(7)
	for _, cab := range []int{3, 7} {
		c.WithCab(cab, func(c *th.Controller) error { return c.MoveSpeed(80) })
	}
	c.Detach("lost")
	s := &sender{}
	if err := c.Attach(s, "again"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.commands, []string{"<=>", "<s>", "<t 3>", "<t 7>"}) {
		t.Fatal("reconnect must resubscribe all cabs", s.commands)
	}
	s.commands = nil
	c.Tick(time.Now().Add(time.Second))
	if len(s.commands) != 0 {
		t.Fatal("stale reconnect commands", s.commands)
	}
}

func TestMultiThrottleFailedEmergencyCancelsEveryQueue(t *testing.T) {
	c, s := setup(t)
	if err := c.AddCab(7); err != nil {
		t.Fatal(err)
	}
	for _, cab := range []int{3, 7} {
		if err := c.WithCab(cab, func(c *th.Controller) error { return c.MoveSpeed(80) }); err != nil {
			t.Fatal(err)
		}
	}
	s.fail = true
	if err := c.WithCab(3, func(c *th.Controller) error { return c.Emergency() }); err == nil {
		t.Fatal("emergency failure lost")
	}
	s.fail = false
	s.commands = nil
	if err := c.Tick(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(s.commands) != 0 {
		t.Fatal("failed emergency replayed movement", s.commands)
	}
	for _, cab := range c.Snapshot().Throttles {
		if cab.Speed != 80 {
			t.Fatal("failed emergency falsely reported stopped locomotive")
		}
	}
}
