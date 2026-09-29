package fyneui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestTelemetryFormValidation(t *testing.T) {
	for _, tc := range []struct{ sampling, interval string }{{"no", "15"}, {"101", "15"}, {"NaN", "15"}, {"100", "no"}, {"100", "0"}} {
		if _, err := telemetryFormSettings(false, false, "", tc.sampling, tc.interval); err == nil {
			t.Fatal("invalid input accepted", tc)
		}
	}
	s, err := telemetryFormSettings(true, true, " http://localhost:4318 ", "25", "10")
	if err != nil || !s.Debug || !s.Enabled || s.SampleRatio != .25 || s.IntervalSeconds != 10 || s.Endpoint != "http://localhost:4318" {
		t.Fatal(s, err)
	}
}

func TestTelemetrySettingsPage(t *testing.T) {
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer collector.Close()
	for _, mobile := range []bool{false, true} {
		for _, storage := range []string{"ok", "missing", "failed"} {
			t.Run(storage+map[bool]string{true: " mobile", false: " desktop"}[mobile], func(t *testing.T) {
				a := test.NewTempApp(t)
				w := a.NewWindow("Telemetry settings")
				m := telemetry.New()
				s := th.NewObservedSession(config.Default(), nil, m)
				t.Cleanup(func() { s.Close(); <-s.Done(); m.Close(); w.Close() })
				var save func(config.TelemetrySettings) error
				if storage != "missing" {
					save = func(config.TelemetrySettings) error {
						if storage == "failed" {
							return errors.New("disk unavailable")
						}
						return nil
					}
				}
				v := newView(w, s, mobile, Options{Telemetry: m, SaveTelemetry: save})
				dispatched := make(chan func(), 1)
				v.dispatch = func(fn func()) { dispatched <- fn }
				t.Cleanup(v.Close)
				original := w.Content()
				windowCount := len(a.Driver().AllWindows())
				if !mobile && w.MainMenu() == nil {
					t.Fatal("desktop settings menu missing")
				}
				v.showTelemetrySettings()
				page := w.Content()
				v.showTelemetrySettings()
				if w.Content() != page {
					t.Fatal("settings page opened twice")
				}
				if len(a.Driver().AllWindows()) != windowCount {
					t.Fatal("settings opened another window")
				}
				root := w.Content()
				if surface, ok := root.(*mobileSurface); ok {
					root = surface.content
				}
				var saveButton, back *widget.Button
				var entries []*widget.Entry
				var enabled *widget.Check
				walkControls(root, func(o fyne.CanvasObject) {
					switch c := o.(type) {
					case *settingsButton:
						if c.Text == "Save" {
							saveButton = &c.Button
						}
						if c.Text == "Back" {
							back = &c.Button
						}
					case *settingsEntry:
						entries = append(entries, &c.Entry)
					case *settingsCheck:
						if c.Text == "Enable telemetry export" {
							enabled = &c.Check
						}
					}
				})
				if saveButton == nil || back == nil || enabled == nil || len(entries) != 3 {
					t.Fatal("settings controls missing")
				}
				if storage == "missing" {
					if !saveButton.Disabled() {
						t.Fatal("save enabled without persistence")
					}
				} else {
					entries[1].SetText("invalid")
					test.Tap(saveButton)
					if m.Settings().Enabled {
						t.Fatal("invalid settings applied")
					}
					entries[1].SetText("25")
					if mobile {
						entries[0].SetText(collector.URL)
						enabled.SetChecked(true)
					}
					test.Tap(saveButton)
					select {
					case fn := <-dispatched:
						fn()
					case <-time.After(5 * time.Second):
						t.Fatal("save did not finish")
					}
					wantRatio := .25
					if storage == "failed" {
						wantRatio = 1
					}
					if m.Settings().SampleRatio != wantRatio {
						t.Fatal("wrong settings after save")
					}
					if m.Settings().Enabled != (mobile && storage == "ok") {
						t.Fatal("wrong export state after save")
					}
				}
				test.Tap(back)
				if w.Content() != original {
					t.Fatal("back lost throttle view")
				}
				v.Render(th.State{Cab: 3, Throttles: []th.CabState{{Cab: 3}}})
				if err := m.Flush(context.Background()); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func telemetryTestView(t *testing.T, save func(config.TelemetrySettings) error) *View {
	t.Helper()
	a := test.NewTempApp(t)
	w := a.NewWindow("Settings")
	m := telemetry.New()
	s := th.NewObservedSession(config.Default(), nil, m)
	v := newView(w, s, true, Options{Telemetry: m, SaveTelemetry: save})
	t.Cleanup(func() { v.Close(); s.Close(); <-s.Done(); m.Close(); w.Close() })
	return v
}

func settingsControls(v *View) (entries []*settingsEntry, checks []*settingsCheck, buttons []*settingsButton) {
	root := v.Window.Content().(*mobileSurface).content
	walkControls(root, func(o fyne.CanvasObject) {
		switch c := o.(type) {
		case *settingsEntry:
			entries = append(entries, c)
		case *settingsCheck:
			checks = append(checks, c)
		case *settingsButton:
			buttons = append(buttons, c)
		}
	})
	return
}

// Mirror the mobile driver's dispatch: focused controls receive the key before
// the canvas. A canvas-only Back handler would fail these tests.
func mobileBack(v *View) {
	key := &fyne.KeyEvent{Name: mobile.KeyBack}
	canvas := v.Window.Canvas()
	if focused := canvas.Focused(); focused != nil {
		focused.TypedKey(key)
	} else {
		canvas.OnTypedKey()(key)
	}
}

func TestTelemetryMobileBack(t *testing.T) {
	v := telemetryTestView(t, func(config.TelemetrySettings) error { return nil })
	original := v.Window.Content()
	canvas := v.Window.Canvas()
	forwarded := 0
	canvas.SetOnTypedKey(func(*fyne.KeyEvent) { forwarded++ })
	for _, focus := range []string{"none", "entry", "check", "button"} {
		t.Run(focus, func(t *testing.T) {
			v.showTelemetrySettings()
			entries, checks, buttons := settingsControls(v)
			switch focus {
			case "entry":
				canvas.Focus(entries[0])
				entries[0].TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
				mobileBack(v)
				if !v.settingsOpen || canvas.Focused() != nil {
					t.Fatal("first Back must dismiss the field focus, not the page")
				}
			case "check":
				canvas.Focus(checks[0])
				checks[0].TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
			case "button":
				canvas.Focus(buttons[0])
				buttons[0].TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
			}
			mobileBack(v)
			if v.settingsOpen || v.Window.Content() != original || canvas.Focused() != nil {
				t.Fatal("Back did not restore the original view")
			}
			canvas.OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyLeft})
		})
	}
	if forwarded != 4 {
		t.Fatal("previous key handler was not restored", forwarded)
	}
	v.showTelemetrySettings()
	canvas.OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if forwarded != 5 {
		t.Fatal("non-Back keys were not forwarded")
	}
}

func TestTelemetryBackWaitsForSave(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	v := telemetryTestView(t, func(config.TelemetrySettings) error { close(entered); <-release; return nil })
	dispatched := make(chan func(), 1)
	v.dispatch = func(fn func()) { dispatched <- fn }
	v.showTelemetrySettings()
	_, _, buttons := settingsControls(v)
	for _, button := range buttons {
		if button.Text == "Save" {
			test.Tap(button)
		}
	}
	<-entered
	mobileBack(v)
	if !v.settingsOpen {
		t.Error("Back abandoned an unfinished save")
	}
	close(release)
	select {
	case fn := <-dispatched:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("save did not complete")
	}
	mobileBack(v)
	if v.settingsOpen {
		t.Fatal("Back remained disabled after save")
	}
}

func TestTelemetrySaveNeverUpdatesClosedView(t *testing.T) {
	for _, dispatched := range []bool{false, true} {
		v := telemetryTestView(t, func(config.TelemetrySettings) error { return nil })
		v.dispatch = func(fn func()) { v.Close(); fn() }
		if !dispatched {
			v.Close()
			v.dispatch = func(func()) { t.Error("closed worker dispatched a UI update") }
		}
		v.applyTelemetrySettings(config.DefaultTelemetry(), func(error) { t.Error("closed view updated") })
		v.showTelemetrySettings()
		if v.settingsOpen {
			t.Fatal("closed view opened settings")
		}
	}
}

func TestTelemetryMobileOverflow(t *testing.T) {
	v := telemetryTestView(t, nil)
	walkControls(v.Window.Content().(*mobileSurface).content, func(o fyne.CanvasObject) {
		if button, ok := o.(*widget.Button); ok && button.Text == "" && button.Icon != nil {
			test.Tap(button)
		}
	})
	if v.Window.Canvas().Overlays().Top() == nil {
		t.Fatal("overflow menu did not open")
	}
}
