package throttle_test

import (
	"path/filepath"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestFunctionLabelsPersistPerTab(t *testing.T) {
	db, err := stations.Open(filepath.Join(t.TempDir(), "stations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: db.SaveThrottles})
	if err := s.Post(func(c *th.Controller) error {
		if err := c.AddCab(7); err != nil {
			return err
		}
		for _, bad := range []struct {
			cab, n int
			text   string
		}{{99, 0, "X"}, {3, -1, "X"}, {3, 29, "X"}, {3, 0, "bad\nlabel"}} {
			if c.SetFunctionLabel(bad.cab, bad.n, bad.text) == nil {
				t.Errorf("accepted %+v", bad)
			}
		}
		if err := c.SetFunctionLabel(3, 0, " Headlight "); err != nil {
			return err
		}
		if err := c.SetFunctionLabel(7, 0, "Cab light"); err != nil {
			return err
		}
		if err := c.SetFunctionLabel(3, 28, "Whistle"); err != nil {
			return err
		}
		return c.ReplaceCab(3, 42)
	}); err != nil {
		t.Fatal(err)
	}
	state := sessionSnapshot(t, s)
	if state.Throttles[0].Labels[0] != "Headlight" || state.Throttles[1].Labels[0] != "Cab light" {
		t.Fatal(state)
	}
	s.Close()
	<-s.Done()
	saved, err := db.LoadThrottles()
	if err != nil {
		t.Fatal(err)
	}
	r := tabSession(t, th.TabPersistence{Initial: saved})
	state = sessionSnapshot(t, r)
	if state.Throttles[0].Labels[28] != "Whistle" || state.Throttles[0].Cab != 42 {
		t.Fatal(state)
	}
	// Snapshots own their label arrays.
	state.Throttles[0].Labels[0] = "changed copy"
	if sessionSnapshot(t, r).Throttles[0].Labels[0] != "Headlight" {
		t.Fatal("snapshot aliases controller")
	}
	if err := r.Post(func(c *th.Controller) error { return c.SetFunctionLabel(42, 28, " ") }); err != nil {
		t.Fatal(err)
	}
	if sessionSnapshot(t, r).Throttles[0].Labels[28] != "" {
		t.Fatal("label not cleared")
	}
	invalid := config.DefaultThrottles()
	invalid.Tabs[0].Labels[28] = "bad\nlabel"
	if err := db.SaveThrottles(invalid); err == nil {
		t.Fatal("invalid saved label")
	}
}
