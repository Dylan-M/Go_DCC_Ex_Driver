package fyneui

import (
	"fyne.io/fyne/v2/test"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"testing"
)

func TestSetupFunctionLabels(t *testing.T) {
	v, s := setupView(t)
	panel := v.panels[3]
	test.Tap(panel.setupButton)
	panel.labels[0].SetText("Headlight")
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Labels[0] == "Headlight" })
	if panel.functions[0].Text != "Headlight" || panel.functionColumns != 6 {
		t.Fatal("label or layout not updated")
	}
	panel.labels[0].SetText("bad\nlabel")
	if panel.labels[0].Validate() == nil {
		t.Fatal("invalid label accepted")
	}
	if err := s.Post(func(c *th.Controller) error { return c.AddCab(7) }); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 2 })
	if v.panels[7].functions[0].Text != "F0" {
		t.Fatal("label leaked to another tab")
	}
	panel.labels[0].SetText("")
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Labels[0] == "" })
	if panel.functions[0].Text != "F0" || panel.functionColumns != 10 {
		t.Fatal("default not restored")
	}
}
