package throttle_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestPerLocoModesAndHeldRelease(t *testing.T) {
	c := th.New(config.Default().Toggle)
	station := &sender{}
	if err := c.Attach(station, "test"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetToggle(0, true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetToggle(3, false); err != nil {
		t.Fatal(err)
	}
	if c.SetToggle(-1, true) == nil || c.SetToggle(29, true) == nil {
		t.Fatal("invalid mode index")
	}
	if err := c.AddCab(7); err != nil {
		t.Fatal(err)
	}
	if c.Snapshot().Toggle[0] || !c.Snapshot().Toggle[3] {
		t.Fatal("new cab inherited another cab's edits")
	}
	station.commands = nil
	// A background tab uses its own mode, never the currently focused tab's.
	for _, pressed := range []bool{true, false, true, false} {
		if err := c.WithCab(3, func(c *th.Controller) error { return c.Function(0, pressed) }); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(station.commands, []string{"<F 3 0 1>", "<F 3 0 0>"}) {
		t.Fatal(station.commands)
	}
	if c.Snapshot().Cab != 7 || c.Snapshot().Toggle[0] {
		t.Fatal("focus or mode leaked")
	}
	station.commands = nil
	if err := c.Function(0, true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetToggle(0, true); err != nil {
		t.Fatal(err)
	}
	if err := c.Function(0, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(station.commands, []string{"<F 7 0 1>", "<F 7 0 0>"}) {
		t.Fatal("held release lost after mode change", station.commands)
	}
	if err := c.SelectCab(9); err != nil {
		t.Fatal(err)
	}
	if c.Snapshot().Toggle[0] || !c.Snapshot().Toggle[3] {
		t.Fatal("SelectCab default modes")
	}
}

func TestPerLocoModesSaveAndRestore(t *testing.T) {
	db, err := stations.Open(filepath.Join(t.TempDir(), "stations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: db.SaveThrottles})
	if err := s.Post(func(c *th.Controller) error {
		if err := c.SetToggle(3, false); err != nil {
			return err
		}
		if err := c.AddCab(7); err != nil {
			return err
		}
		if err := c.SetToggle(28, true); err != nil {
			return err
		}
		return c.ReplaceCab(7, 42)
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
	if saved.Tabs[0].Toggle == nil || *saved.Tabs[0].Toggle != [29]bool{} {
		t.Fatal("all-momentary preference lost", saved)
	}
	r := tabSession(t, th.TabPersistence{Initial: saved})
	state := sessionSnapshot(t, r)
	if state.Throttles[0].Toggle != [29]bool{} || !state.Throttles[1].Toggle[28] || state.Throttles[1].Cab != 42 {
		t.Fatal(state)
	}
	// Mutation of caller-owned persistence data must not affect the session.
	(*saved.Tabs[1].Toggle)[28] = false
	if !sessionSnapshot(t, r).Throttles[1].Toggle[28] {
		t.Fatal("restored data aliases caller")
	}
}
