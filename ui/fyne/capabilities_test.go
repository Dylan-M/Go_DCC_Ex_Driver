package fyneui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func walkControls(root fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(root)
	switch o := root.(type) {
	case *fyne.Container:
		for _, child := range o.Objects {
			walkControls(child, visit)
		}
	case *container.Scroll:
		walkControls(o.Content, visit)
	case *container.ThemeOverride:
		walkControls(o.Content, visit)
	}
}

func TestThrottleCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		mobile, requested, power bool
	}{
		{"desktop Engineer", false, false, false},
		{"desktop Power", false, true, true},
		{"Android Engineer", true, false, false},
		{"Android ignores Power option", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := test.NewTempApp(t)
			w := a.NewWindow(tc.name)
			s := th.NewSession(config.Default(), nil)
			t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
			v := newView(w, s, tc.mobile, Options{PowerThrottle: tc.requested})
			if err := s.RenameCab(3, "Ready"); err != nil {
				t.Fatal(err)
			}
			renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "Ready" })
			if v.last.Poll != tc.power || v.powerThrottle != tc.power {
				t.Fatal("mode or current polling does not match capabilities")
			}
			wantTabs := 2
			if tc.power {
				wantTabs = 3
			}
			if len(v.tabs.Items) != wantTabs || v.tabs.Items[0].Text != "Connection" || v.tabs.Items[1].Text != "Run" {
				t.Fatal("unexpected tabs")
			}
			if (v.mainPower != nil) != tc.power || (v.programmingTabs != nil) != tc.power || (v.current != nil) != tc.power {
				t.Fatal("administrative controls constructed in the wrong mode")
			}
			// The connection form and all locomotive controls remain available.
			if v.connect == nil || v.savedStations == nil || v.speed == nil || v.direction == nil || v.setupButton == nil || len(v.functions) != 29 {
				t.Fatal("Engineer controls missing")
			}
			entries, sends := 0, 0
			root := w.Content()
			if surface, ok := root.(*mobileSurface); ok {
				root = surface.content
			}
			walkControls(root.(*container.Split).Trailing, func(object fyne.CanvasObject) {
				switch o := object.(type) {
				case *widget.Entry:
					entries++
				case *widget.Button:
					if o.Text == "Send" {
						sends++
					}
				}
			})
			wantInputs := 0
			if tc.power {
				wantInputs = 1
			}
			if entries != wantInputs || sends != wantInputs {
				t.Fatal("raw command entry must be Power-only", entries, sends)
			}
			// Unsolicited station state must render safely even without power or
			// programming widgets, and must never reveal them in Engineer mode.
			state := v.last
			state.Connected = true
			state.MainPower, state.ProgPower = p.On, p.Off
			state.ProgramAddress, state.ProgramValue, state.ProgramResult = "42", "6", "Complete"
			state.CV29, state.CV29Known = 6, true
			state.HasCurrent, state.CurrentMA, state.TripMA = true, 200, 2000
			state.Logs = []th.LogEntry{{Kind: "rx", Text: "<p1 MAIN>"}}
			v.Render(state)
			if len(v.tabs.Items) != wantTabs || v.logs[0] != state.Logs[0] {
				t.Fatal("station update changed capabilities or lost diagnostics")
			}
			if tc.power {
				if v.tabs.Items[2].Text != "Programming" || v.progAddress.Text != "42" || v.progValue.Text != "6" || v.cv29Label.Text != "CV29 = 6" || v.currentBar.Value != 200 {
					t.Fatal("Power mode did not render station state")
				}
				state.TripMA, state.MaxMA = 0, 3000
				v.Render(state)
				state.MaxMA = 0
				v.Render(state)
				state.Overload = true
				v.Render(state)
				if v.current.Text != "OVERLOAD" {
					t.Fatal("overload not displayed")
				}
				// Keep coverage of CV29 actions now that mobile is Engineer-only.
				tapped := 0
				walkControls(v.programmingTabs.Items[0].Content, func(object fyne.CanvasObject) {
					if b, ok := object.(*widget.Button); ok && (b.Text == "Read CV29" || b.Text == "Write CV29") {
						test.Tap(b)
						tapped++
					}
				})
				if tapped != 2 {
					t.Fatal("missing CV29 actions")
				}
				walkControls(v.tabs.Items[0].Content, func(object fyne.CanvasObject) {
					if poll, ok := object.(*widget.Check); ok && poll.Text == "Poll current" {
						poll.SetChecked(false)
						v.rendering = true
						poll.SetChecked(true) // Rendering must not submit an intent.
						v.rendering = false
					}
				})
				renderUntil(t, v, s, func(state th.State) bool { return !state.Poll })
			}
		})
	}
}
