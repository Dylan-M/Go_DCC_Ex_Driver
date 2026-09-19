package throttle_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestNamesAreLocalAndSurviveRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stations.db")
	db, err := stations.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: db.SaveThrottles})
	station := &sender{}
	if err := s.Post(func(c *th.Controller) error {
		if err := c.AddCab(7); err != nil {
			return err
		}
		if err := c.Attach(station, "test"); err != nil {
			return err
		}
		station.commands = nil
		if err := c.RenameCab(3, "  Étoile 🚂  "); err != nil {
			return err
		}
		if err := c.RenameCab(7, "Freight"); err != nil {
			return err
		}
		if c.RenameCab(99, "missing") == nil {
			t.Error("renamed closed tab")
		}
		if c.RenameCab(3, "bad\nname") == nil {
			t.Error("accepted invalid name")
		}
		c.Receive(p.LocoState{Cab: 3, Speed: 20, Direction: 0, FunctionMask: 4})
		return c.Tick(time.Now())
	}); err != nil {
		t.Fatal(err)
	}
	state := sessionSnapshot(t, s)
	if state.Throttles[0].Name != "Étoile 🚂" || state.Throttles[1].Name != "Freight" || len(station.commands) != 0 {
		t.Fatal(state, station.commands)
	}
	s.Close()
	<-s.Done()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = stations.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	saved, err := db.LoadThrottles()
	if err != nil {
		t.Fatal(err)
	}
	r := tabSession(t, th.TabPersistence{Initial: saved})
	state = sessionSnapshot(t, r)
	if state.Throttles[0].Name != "Étoile 🚂" || state.Throttles[0].Speed != 0 || state.Throttles[0].Functions != [29]bool{} {
		t.Fatal(state)
	}
	if err := r.Post(func(c *th.Controller) error {
		if err := c.ReplaceCab(3, 42); err != nil {
			return err
		}
		return c.RenameCab(7, "  ")
	}); err != nil {
		t.Fatal(err)
	}
	state = sessionSnapshot(t, r)
	if state.Throttles[0].Cab != 42 || state.Throttles[0].Name != "Étoile 🚂" || state.Throttles[1].Name != "" {
		t.Fatal(state)
	}
}
