package fyneui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func setupView(t *testing.T) (*View, *th.Session) {
	t.Helper()
	a := test.NewTempApp(t)
	w := a.NewWindow("setup")
	s := th.NewSession(config.Default(), nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
	v := New(w, s)
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 1 })
	return v, s
}

func renderUntil(t *testing.T, v *View, s *th.Session, match func(th.State) bool) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case state := <-s.Updates():
			v.Render(state)
			if match(state) {
				return
			}
		case <-timer.C:
			t.Fatal("state timeout")
		}
	}
}

func TestLocoSetupNames(t *testing.T) {
	v, s := setupView(t)
	panel := v.panels[3]
	test.Tap(panel.setupButton)
	first := panel.setup
	panel.showSetup()
	if panel.setup != first {
		t.Fatal("duplicate setup dialog")
	}
	panel.name.SetText("Étoile")
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "Étoile" })
	if panel.tab.Text != "Étoile" {
		t.Fatal(panel.tab.Text)
	}
	panel.name.SetText("bad\nname")
	if panel.name.Validate() == nil {
		t.Fatal("invalid name accepted")
	}
	panel.name.SetText("")
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "" })
	if panel.tab.Text != "Loco 3" {
		t.Fatal(panel.tab.Text)
	}
	first.Hide()
	if panel.setup != nil {
		t.Fatal("dialog retained after closing")
	}
	panel.showSetup()
	if err := s.Post(func(c *th.Controller) error { return c.AddCab(7) }); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 2 })
	if err := s.Post(func(c *th.Controller) error { return c.RemoveCab(3) }); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 1 })
	if panel.setup != nil {
		t.Fatal("orphaned setup dialog")
	}
}
