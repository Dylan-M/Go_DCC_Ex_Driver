package throttle_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestHiddenFunctionsStillTrackAndTurnOff(t *testing.T) {
	c := th.New(config.Default().Toggle)
	station := &sender{}
	if err := c.Attach(station, "test"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddCab(7); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][2]int{{99, 0}, {3, -1}, {3, 29}} {
		if c.SetFunctionHidden(bad[0], bad[1], true) == nil {
			t.Fatal("invalid visibility", bad)
		}
	}
	station.commands = nil
	if err := c.SetFunctionHidden(3, 28, true); err != nil {
		t.Fatal(err)
	}
	if len(station.commands) != 0 {
		t.Fatal("hiding sent commands")
	}
	c.Receive(p.LocoState{Cab: 3, Direction: 1, FunctionMask: 1 << 28})
	state := c.Snapshot()
	if !state.Throttles[0].Functions[28] || !state.Throttles[0].Hidden[28] || state.Throttles[1].Hidden[28] {
		t.Fatal(state)
	}
	if err := c.WithCab(3, func(c *th.Controller) error { return c.AllFunctionsOff() }); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(station.commands, []string{"<F 3 28 0>"}) {
		t.Fatal(station.commands)
	}
	if err := c.Function(0, true); err != nil {
		t.Fatal(err)
	}
	if c.SetFunctionHidden(7, 0, true) == nil || c.Snapshot().Throttles[1].Hidden[0] {
		t.Fatal("hid held function")
	}
	if err := c.Function(0, false); err != nil {
		t.Fatal(err)
	}
	if err := c.SetFunctionHidden(7, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetFunctionHidden(7, 0, false); err != nil {
		t.Fatal(err)
	}
	if c.Snapshot().Throttles[1].Hidden[0] {
		t.Fatal("failed to show function")
	}
}

func TestVisibilityPersistsWithLabelsAndModes(t *testing.T) {
	db, err := stations.Open(filepath.Join(t.TempDir(), "stations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: db.SaveThrottles})
	if err := s.Post(func(c *th.Controller) error {
		if err := c.SetFunctionLabel(3, 28, "Whistle"); err != nil {
			return err
		}
		if err := c.SetToggle(28, true); err != nil {
			return err
		}
		for n := 0; n < 29; n++ {
			if err := c.SetFunctionHidden(3, n, true); err != nil {
				return err
			}
		}
		return c.ReplaceCab(3, 42)
	}); err != nil {
		t.Fatal(err)
	}
	sessionSnapshot(t, s)
	s.Close()
	<-s.Done()
	saved, err := db.LoadThrottles()
	if err != nil {
		t.Fatal(err)
	}
	r := tabSession(t, th.TabPersistence{Initial: saved})
	state := sessionSnapshot(t, r)
	cab := state.Throttles[0]
	if cab.Cab != 42 || cab.Labels[28] != "Whistle" || !cab.Toggle[28] {
		t.Fatal(cab)
	}
	for _, hidden := range cab.Hidden {
		if !hidden {
			t.Fatal("visibility lost")
		}
	}
	if err := r.Post(func(c *th.Controller) error { return c.SetFunctionHidden(42, 28, false) }); err != nil {
		t.Fatal(err)
	}
	cab = sessionSnapshot(t, r).Throttles[0]
	if cab.Hidden[28] || cab.Labels[28] != "Whistle" || !cab.Toggle[28] {
		t.Fatal(cab)
	}
}
