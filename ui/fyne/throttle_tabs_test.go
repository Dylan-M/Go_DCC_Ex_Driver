package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestTabbedThrottles(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("tabs")
	s := th.NewSession(config.Default(), nil, nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
	v := New(w, s)
	if len(v.tabs.Items) != 3 || v.tabs.Items[0].Text != "Connection" || v.tabs.Items[1].Text != "Run" || v.tabs.Items[2].Text != "Programming" {
		t.Fatal("top-level tab layout")
	}
	// Station power and current belong exclusively inside Connection, not
	// in a persistent header above all three tabs.
	split, ok := w.Content().(*container.Split)
	if !ok || split.Leading != v.tabs {
		t.Fatal("unexpected controls outside the tab layout")
	}
	var contains func(fyne.CanvasObject, fyne.CanvasObject) bool
	contains = func(root, target fyne.CanvasObject) bool {
		if root == target {
			return true
		}
		switch obj := root.(type) {
		case *fyne.Container:
			for _, child := range obj.Objects {
				if contains(child, target) {
					return true
				}
			}
		case *container.Scroll:
			return contains(obj.Content, target)
		case *container.ThemeOverride:
			return contains(obj.Content, target)
		}
		return false
	}
	for _, control := range []fyne.CanvasObject{v.allOn, v.allOff, v.mainPower, v.progPower, v.current, v.currentBar} {
		if !contains(v.tabs.Items[0].Content, control) {
			t.Fatal("station control missing from Connection")
		}
		for _, tab := range v.tabs.Items[1:] {
			if contains(tab.Content, control) {
				t.Fatal("station control appears outside Connection")
			}
		}
	}
	wait := func(match func(th.State) bool) {
		t.Helper()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case state := <-s.Updates():
				v.Render(state)
				if match(state) {
					return
				}
			case <-deadline.C:
				t.Fatal("state timeout")
			}
		}
	}
	if err := s.Post(func(c *th.Controller) error { return c.AddCab(7) }); err != nil {
		t.Fatal(err)
	}
	wait(func(s th.State) bool { return len(s.Throttles) == 2 && s.Cab == 7 })
	if len(v.runTabs.Items) != 2 || v.runTabs.Selected() != v.panels[7].tab {
		t.Fatal("second throttle missing")
	}
	// Simulate an intent from tab 3 while tab 7 owns focus.
	v.panels[3].speed.SetValue(25)
	wait(func(s th.State) bool { return s.Throttles[0].Speed == 25 })
	if v.panels[7].speed.Value != 0 || v.panels[3].speed.Value != 25 {
		t.Fatal("speed leaked across panels")
	}
	v.runTabs.Select(v.panels[3].tab)
	wait(func(s th.State) bool { return s.Cab == 3 })
	if v.panels[3].speed.Value != 25 {
		t.Fatal("tab switch reset speed")
	}
	if err := s.Post(func(c *th.Controller) error {
		return c.WithCab(3, func(c *th.Controller) error { return c.MoveSpeed(0) })
	}); err != nil {
		t.Fatal(err)
	}
	wait(func(s th.State) bool { return s.Speed == 0 })
	v.runTabs.CloseIntercept(v.panels[7].tab)
	wait(func(s th.State) bool { return len(s.Throttles) == 1 })
	if len(v.runTabs.Items) != 1 {
		t.Fatal("closed throttle still visible")
	}
	// Optional visual QA artifact from Fyne's real software canvas renderer.
	if path := os.Getenv("DCCEX_UI_SCREENSHOT"); path != "" {
		v.tabs.SelectIndex(1)
		w.Resize(fyne.NewSize(1050, 840))
		w.Show()
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, w.Canvas().Capture()); err != nil {
			file.Close()
			t.Fatal(err)
		}
		file.Close()
	}
}
