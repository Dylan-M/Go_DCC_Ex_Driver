package fyneui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
)

func (v *View) installSettingsAccess() {
	if v.telemetry == nil {
		return
	}
	settings := fyne.NewMenuItem("Settings", v.showTelemetrySettings)
	if !v.mobile {
		v.Window.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("Settings", settings)))
		return
	}
	// Android uses an app-bar overflow menu and an in-window settings page,
	// never a second desktop window.
	var overflow *widget.Button
	overflow = widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), func() {
		widget.ShowPopUpMenuAtPosition(fyne.NewMenu("", settings), v.Window.Canvas(), fyne.CurrentApp().Driver().AbsolutePositionForObject(overflow).Add(fyne.NewPos(0, overflow.Size().Height)))
	})
	bar := container.NewBorder(nil, nil, widget.NewLabel("DCC-EX"), overflow)
	v.Window.SetContent(newMobileSurface(container.NewBorder(bar, nil, nil, nil, v.Window.Content())))
}

func (v *View) showTelemetrySettings() {
	if v.settingsOpen || v.closed.Load() {
		return
	}
	v.settingsOpen = true
	v.telemetry.Event(context.Background(), "ui.settings_opened")
	previous := v.Window.Content()
	canvas := v.Window.Canvas()
	previousKey := canvas.OnTypedKey()
	canvas.Unfocus()
	var saving bool
	leave := func() {
		if saving {
			return
		}
		canvas.Unfocus()
		canvas.SetOnTypedKey(previousKey)
		v.settingsOpen = false
		v.Window.SetContent(previous)
	}
	if v.mobile {
		canvas.SetOnTypedKey(func(e *fyne.KeyEvent) {
			if e.Name == mobile.KeyBack {
				leave()
			} else if previousKey != nil {
				previousKey(e)
			}
		})
	}
	saved := v.telemetry.Settings()
	enabled := newSettingsCheck("Enable telemetry export", leave)
	enabled.SetChecked(saved.Enabled)
	endpoint := newSettingsEntry(saved.Endpoint, canvas.Unfocus)
	endpoint.SetPlaceHolder("http://collector-host:4318")
	debug := newSettingsCheck("Debug logs (includes command traffic)", leave)
	debug.SetChecked(saved.Debug)
	sampling := newSettingsEntry(strconv.FormatFloat(saved.SampleRatio*100, 'f', -1, 64), canvas.Unfocus)
	interval := newSettingsEntry(strconv.Itoa(saved.IntervalSeconds), canvas.Unfocus)
	status := wrappedLabel("Changes apply immediately after saving. Metrics are not trace-sampled. No firmware changes are required.")
	back := newSettingsButton("Back", leave, leave)
	back.SetIcon(theme.NavigateBackIcon())
	var apply *settingsButton
	apply = newSettingsButton("Save", leave, func() {
		next, err := telemetryFormSettings(enabled.Checked, debug.Checked, endpoint.Text, sampling.Text, interval.Text)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		apply.Disable()
		back.Disable()
		saving = true
		go v.applyTelemetrySettings(next, func(err error) {
			apply.Enable()
			back.Enable()
			saving = false
			if err != nil {
				status.SetText("Could not save telemetry settings: " + err.Error())
				return
			}
			status.SetText("Settings saved. Export is disabled.")
			if next.Enabled {
				status.SetText("Settings saved. Export is enabled; this does not confirm collector connectivity.")
			}
		})
	})
	if v.saveTelemetry == nil {
		apply.Disable()
		status.SetText("Settings storage is unavailable. Telemetry settings cannot be changed this session.")
	}
	body := container.NewVBox(wrappedLabel("OpenTelemetry — metrics, events, logs and traces"), enabled,
		widget.NewLabel("Collector OTLP/HTTP base URL"), endpoint,
		wrappedLabel("Use HTTPS outside a trusted local network. Credentials must not be included in the URL. Configure collector authentication at a local gateway."),
		debug, widget.NewLabel("Trace sampling (%)"), sampling,
		widget.NewLabel("Metric export interval (seconds)"), interval, status, buttonRows(apply))
	page := container.NewBorder(container.NewBorder(nil, nil, back, nil, widget.NewLabel("Settings")), nil, nil, nil, container.NewVScroll(body))
	if v.mobile {
		v.Window.SetContent(newMobileSurface(page))
	} else {
		v.Window.SetContent(page)
	}
}

func (v *View) applyTelemetrySettings(next config.TelemetrySettings, complete func(error)) {
	err := v.telemetry.Configure(next, v.saveTelemetry)
	if v.closed.Load() {
		return
	}
	v.dispatch(func() {
		if !v.closed.Load() {
			complete(err)
		}
	})
}

// The mobile driver delivers Back to the focused widget instead of the canvas.
// Text fields first release focus (and the keyboard); a subsequent Back leaves
// the page. Other controls navigate directly, subject to the pending-save guard.
type settingsEntry struct {
	widget.Entry
	onBack func()
}

func newSettingsEntry(text string, back func()) *settingsEntry {
	e := &settingsEntry{onBack: back}
	e.ExtendBaseWidget(e)
	e.SetText(text)
	return e
}

func (e *settingsEntry) TypedKey(key *fyne.KeyEvent) {
	settingsKey(key, e.onBack, e.Entry.TypedKey)
}

type settingsCheck struct {
	widget.Check
	onBack func()
}

func newSettingsCheck(text string, back func()) *settingsCheck {
	c := &settingsCheck{onBack: back}
	c.Text = text
	c.ExtendBaseWidget(c)
	return c
}

func (c *settingsCheck) TypedKey(key *fyne.KeyEvent) {
	settingsKey(key, c.onBack, c.Check.TypedKey)
}

type settingsButton struct {
	widget.Button
	onBack func()
}

func newSettingsButton(text string, back, tap func()) *settingsButton {
	b := &settingsButton{onBack: back}
	b.Text, b.OnTapped = text, tap
	b.ExtendBaseWidget(b)
	return b
}

func (b *settingsButton) TypedKey(key *fyne.KeyEvent) {
	settingsKey(key, b.onBack, b.Button.TypedKey)
}

func settingsKey(key *fyne.KeyEvent, back func(), otherwise func(*fyne.KeyEvent)) {
	if key.Name == mobile.KeyBack {
		back()
		return
	}
	otherwise(key)
}

// Close prevents pending settings workers from updating a closed application.
func (v *View) Close() { v.closed.Store(true) }

func telemetryFormSettings(enabled, debug bool, endpoint, sampling, interval string) (config.TelemetrySettings, error) {
	s := config.DefaultTelemetry()
	s.Enabled, s.Debug, s.Endpoint = enabled, debug, strings.TrimSpace(endpoint)
	ratio, err := strconv.ParseFloat(strings.TrimSpace(sampling), 64)
	if err != nil {
		return s, fmt.Errorf("trace sampling must be a number from 0 to 100")
	}
	s.SampleRatio = ratio / 100
	s.IntervalSeconds, err = strconv.Atoi(strings.TrimSpace(interval))
	if err != nil {
		return s, fmt.Errorf("metric interval must be a whole number from 5 to 300")
	}
	return s, s.Validate()
}
