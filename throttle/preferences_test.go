package throttle_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func sessionSnapshot(t *testing.T, s *th.Session) th.State {
	t.Helper()
	result := make(chan th.State, 1)
	if err := s.Post(func(c *th.Controller) error { result <- c.Snapshot(); return nil }); err != nil {
		t.Fatal(err)
	}
	select {
	case state := <-result:
		return state
	case <-time.After(5 * time.Second):
		t.Fatal("session did not process intents")
	}
	return th.State{}
}

func tabSession(t *testing.T, tabs th.TabPersistence) *th.Session {
	t.Helper()
	s := th.NewSession(config.Default(), nil, nil, tabs)
	t.Cleanup(func() { s.Close(); <-s.Done() })
	return s
}

func cabAddresses(state th.State) []int {
	var addresses []int
	for _, cab := range state.Throttles {
		addresses = append(addresses, cab.Cab)
	}
	return addresses
}

func TestTabChangesSurviveRestartWithoutLiveState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "throttles.json")
	saves := 0
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: func(settings config.ThrottleSettings) error {
		saves++
		return config.SaveThrottles(path, settings)
	}})
	for _, intent := range []func(*th.Controller) error{
		func(c *th.Controller) error { return c.AddCab(42) },
		func(c *th.Controller) error { return c.AddCab(7) },
		func(c *th.Controller) error { return c.ReplaceCab(3, 99) },
		func(c *th.Controller) error { return c.RemoveCab(42) },
		func(c *th.Controller) error { return c.FocusCab(7) },
	} {
		if err := s.Post(intent); err != nil {
			t.Fatal(err)
		}
	}
	state := sessionSnapshot(t, s)
	if saves != 5 || !reflect.DeepEqual(cabAddresses(state), []int{99, 7}) || state.Cab != 7 {
		t.Fatal(saves, state)
	}
	// Invalid mutations and operating controls must not write layout preferences.
	for _, intent := range []func(*th.Controller) error{
		func(c *th.Controller) error { return c.AddCab(7) },
		func(c *th.Controller) error { return c.ReplaceCab(99, 7) },
		func(c *th.Controller) error { return c.AddCab(0) },
		func(c *th.Controller) error { return c.MoveSpeed(50) },
		func(c *th.Controller) error { c.SetPoll(false); return nil },
	} {
		if err := s.Post(intent); err != nil {
			t.Fatal(err)
		}
	}
	sessionSnapshot(t, s)
	if saves != 5 {
		t.Fatal("operating controls or rejected edit saved layout", saves)
	}
	s.Close()
	<-s.Done()
	saved, err := config.LoadThrottles(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := tabSession(t, th.TabPersistence{Initial: saved})
	state = sessionSnapshot(t, restarted)
	if !reflect.DeepEqual(cabAddresses(state), []int{99, 7}) || state.Cab != 7 || state.Connected {
		t.Fatal(state)
	}
	for _, cab := range state.Throttles {
		if cab.Speed != 0 || cab.Direction != 1 || cab.Functions != [29]bool{} {
			t.Fatal("restored live operating state", cab)
		}
	}
	station := &sender{}
	if err := restarted.Post(func(c *th.Controller) error {
		if err := c.Attach(station, "test"); err != nil {
			return err
		}
		return c.Tick(time.Now().Add(time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	sessionSnapshot(t, restarted)
	if !reflect.DeepEqual(station.commands, []string{"<=>", "<s>", "<t 99>", "<t 7>"}) {
		t.Fatal("restore sent operating commands", station.commands)
	}
}

func TestTabPersistenceFailureIsVisibleAndRetryable(t *testing.T) {
	saves := 0
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: func(settings config.ThrottleSettings) error {
		saves++
		if saves == 1 {
			return errors.New("disk full")
		}
		if len(settings.Tabs) != 2 || settings.Selected != 3 {
			return errors.New("wrong retry data")
		}
		return nil
	}})
	s.Post(func(c *th.Controller) error { return c.AddCab(7) })
	state := sessionSnapshot(t, s)
	if len(state.Throttles) != 2 || !strings.Contains(state.Logs[len(state.Logs)-1].Text, "Could not save throttle tabs: disk full") {
		t.Fatal(state)
	}
	s.Post(func(c *th.Controller) error { return c.FocusCab(3) })
	state = sessionSnapshot(t, s)
	if saves != 2 || state.Cab != 3 {
		t.Fatal(saves, state)
	}
}

func TestBadRestorationCannotOverwriteSettings(t *testing.T) {
	called := false
	s := tabSession(t, th.TabPersistence{Initial: config.ThrottleSettings{Version: 99}, Save: func(config.ThrottleSettings) error { called = true; return nil }})
	state := sessionSnapshot(t, s)
	if !reflect.DeepEqual(cabAddresses(state), []int{3}) || len(state.Logs) == 0 {
		t.Fatal(state)
	}
	s.Post(func(c *th.Controller) error { return c.AddCab(7) })
	sessionSnapshot(t, s)
	if called {
		t.Fatal("invalid input was overwritten")
	}
}

func TestSaveLayoutAfterStateQueryFailure(t *testing.T) {
	var saved config.ThrottleSettings
	s := tabSession(t, th.TabPersistence{Initial: config.DefaultThrottles(), Save: func(value config.ThrottleSettings) error { saved = value; return nil }})
	s.Post(func(c *th.Controller) error {
		station := &sender{}
		if err := c.Attach(station, "test"); err != nil {
			return err
		}
		station.fail = true
		return c.ReplaceCab(3, 42)
	})
	state := sessionSnapshot(t, s)
	if !reflect.DeepEqual(cabAddresses(state), []int{42}) || state.Connected || saved.Selected != 42 || len(saved.Tabs) != 1 {
		t.Fatal("query failure lost layout change", state, saved)
	}
}
