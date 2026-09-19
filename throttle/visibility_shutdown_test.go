package throttle_test

import (
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"path/filepath"
	"testing"
)

func TestAllPreferencesPersistWhenClosingImmediately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stations.db")
	db, err := stations.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: db.SaveThrottles})
	entered, release := make(chan struct{}), make(chan struct{})
	if err := s.Post(func(*th.Controller) error { close(entered); <-release; return nil }); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, entered)
	for _, err := range []error{s.RenameCab(3, "Last name"), s.SetFunctionLabel(3, 28, "Last label"), s.SetFunctionToggle(3, 28, true), s.SetFunctionHidden(3, 28, true)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	close(release)
	waitClosed(t, s.Done())
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
	cab := saved.Tabs[0]
	if cab.Name != "Last name" || cab.Labels[28] != "Last label" || cab.Toggle == nil || !cab.Toggle[28] || !cab.Hidden[28] {
		t.Fatal(cab)
	}
}
