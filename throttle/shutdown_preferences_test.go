package throttle_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func waitClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("session did not close")
	}
}

func TestCloseFlushesPreferencesButNotOperatingCommands(t *testing.T) {
	for _, failSave := range []bool{false, true} {
		var saved config.ThrottleSettings
		s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: func(value config.ThrottleSettings) error {
			if failSave {
				return errors.New("disk full")
			}
			saved = value
			return nil
		}})
		entered, release := make(chan struct{}), make(chan struct{})
		station := &sender{}
		if err := s.Post(func(c *th.Controller) error {
			if err := c.Attach(station, "test"); err != nil {
				return err
			}
			station.commands = nil
			close(entered)
			<-release
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		waitClosed(t, entered)
		if err := s.RenameCab(3, "First edit"); err != nil {
			t.Fatal(err)
		}
		operated := false
		if err := s.Post(func(c *th.Controller) error { operated = true; return c.Raw("1") }); err != nil {
			t.Fatal(err)
		}
		if err := s.RenameCab(99, "Closed tab"); err != nil {
			t.Fatal(err)
		}
		if err := s.RenameCab(3, "Last edit"); err != nil {
			t.Fatal(err)
		}
		s.Close()
		if err := s.RenameCab(3, "Too late"); err == nil {
			t.Fatal("accepted preference after close")
		}
		close(release)
		waitClosed(t, s.Done())
		if operated || len(station.commands) != 0 {
			t.Fatal("shutdown executed operating command", station.commands)
		}
		var final th.State
		for state := range s.Updates() {
			final = state
		}
		if final.Throttles[0].Name != "Last edit" || final.Connected {
			t.Fatal(final)
		}
		if !failSave && saved.Tabs[0].Name != "Last edit" {
			t.Fatal("last accepted edit lost", saved)
		}
		if failSave && !strings.Contains(final.Logs[len(final.Logs)-1].Text, "disk full") {
			t.Fatal("save failure not reported", final.Logs)
		}
	}
}

func TestCloseAndPreferenceAcceptanceAreAtomic(t *testing.T) {
	for i := 0; i < 100; i++ {
		var saved config.ThrottleSettings
		s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: func(value config.ThrottleSettings) error { saved = value; return nil }})
		start := make(chan struct{})
		var accepted bool
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; accepted = s.RenameCab(3, "Raced edit") == nil }()
		go func() { defer wg.Done(); <-start; s.Close() }()
		close(start)
		wg.Wait()
		waitClosed(t, s.Done())
		if accepted && (len(saved.Tabs) != 1 || saved.Tabs[0].Name != "Raced edit") {
			t.Fatal("accepted preference was discarded", saved)
		}
	}
}

func TestPreferenceQueueFullAndNoStorage(t *testing.T) {
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles()})
	entered, release := make(chan struct{}), make(chan struct{})
	if err := s.Post(func(*th.Controller) error { close(entered); <-release; return nil }); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, entered)
	for i := 0; i < 128; i++ {
		if err := s.RenameCab(3, "Queued edit"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RenameCab(3, "Overflow"); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatal(err)
	}
	s.Close()
	close(release)
	waitClosed(t, s.Done())
	var final th.State
	for state := range s.Updates() {
		final = state
	}
	if final.Throttles[0].Name != "Queued edit" {
		t.Fatal(final)
	}
}
