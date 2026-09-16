package throttle_test

import (
	"reflect"
	"testing"
)

func TestPOMExplicitAddressIsIndependentOfThrottles(t *testing.T) {
	c, s := setup(t)
	if err := c.AddCab(7); err != nil {
		t.Fatal(err)
	}
	before := c.Snapshot()
	s.commands = nil
	if err := c.POM(42, 29, 6); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.commands, []string{"<w 42 29 6>"}) {
		t.Fatal("POM used the Run address", s.commands)
	}
	after := c.Snapshot()
	if before.Cab != after.Cab || !reflect.DeepEqual(before.Throttles, after.Throttles) {
		t.Fatal("POM changed the Run throttles")
	}
	for _, args := range [][3]int{{0, 29, 6}, {10294, 29, 6}, {42, 0, 6}, {42, 1025, 6}, {42, 29, -1}, {42, 29, 256}} {
		s.commands = nil
		if c.POM(args[0], args[1], args[2]) == nil || len(s.commands) != 0 {
			t.Fatal("invalid POM request sent", args, s.commands)
		}
	}
}
