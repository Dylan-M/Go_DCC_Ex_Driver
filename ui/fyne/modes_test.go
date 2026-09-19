package fyneui

import (
	"fyne.io/fyne/v2/test"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"testing"
)

func TestModeEditorTargetsOriginatingTab(t *testing.T) {
	v, s := setupView(t)
	panel := v.panels[3]
	test.Tap(panel.setupButton)
	if !panel.modes[3].Checked {
		t.Fatal("missing initial mode")
	}
	panel.modes[0].SetChecked(true)
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Toggle[0] })
	if panel.functions[0].Text != "F0 ↕" {
		t.Fatal(panel.functions[0].Text)
	}
	if err := s.Post(func(c *th.Controller) error { return c.AddCab(7) }); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 2 })
	if v.panels[7].functions[0].Text != "F0" {
		t.Fatal("mode leaked")
	}
	// Two queued flips must cancel, even before a new snapshot is rendered.
	panel.functions[0].TappedSecondary(nil)
	panel.functions[0].TappedSecondary(nil)
	panel.modes[28].SetChecked(true)
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Toggle[28] })
	if !panel.modes[0].Checked || v.panels[7].functions[28].Text != "F28" {
		t.Fatal("stale toggle or wrong target")
	}
	panel.functions[0].TappedSecondary(nil)
	renderUntil(t, v, s, func(state th.State) bool { return !state.Throttles[0].Toggle[0] })
	if panel.modes[0].Checked || panel.functions[0].Text != "F0" {
		t.Fatal("setup did not follow mode change")
	}
}
