package throttle_test

import (
	"testing"
	"time"
)

func TestExplicitDirectionSelection(t *testing.T) {
	c, sender := setup(t)
	receive(t, c, "<l 3 0 148 0>") // Forward, speed 19.
	now := time.Now()
	if err := c.SetDirection(1, now); err != nil {
		t.Fatal(err)
	}
	if len(sender.commands) != 0 {
		t.Fatal("reselected direction transmitted")
	}
	if err := c.SetDirection(0, now); err != nil {
		t.Fatal(err)
	}
	if c.Snapshot().Direction != 0 || sender.commands[0] != "<t 3 19 0>" {
		t.Fatal("explicit reverse not applied")
	}
	if err := c.SetDirection(0, now); err != nil {
		t.Fatal(err)
	}
	if len(sender.commands) != 1 {
		t.Fatal("duplicate selection transmitted")
	}
	if c.SetDirection(-1, now) == nil || c.SetDirection(2, now) == nil {
		t.Fatal("invalid direction accepted")
	}
	sender.fail = true
	if c.SetDirection(1, now) == nil {
		t.Fatal("send failure hidden")
	}
	if c.Snapshot().Direction != 0 || c.Snapshot().Connected {
		t.Fatal("failed selection changed state")
	}
}
