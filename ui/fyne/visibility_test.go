package fyneui

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestSetupFunctionVisibility(t *testing.T) {
	v, s := setupView(t)
	panel := v.panels[3]
	test.Tap(panel.setupButton)
	panel.name.SetText("Branch line")
	panel.labels[0].SetText("Headlight")
	panel.shown[0].SetChecked(false)
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Hidden[0] })
	if panel.functionCells[0].Visible() || panel.functionColumns != 10 {
		t.Fatal("hidden label affected layout")
	}
	panel.shown[0].SetChecked(true)
	renderUntil(t, v, s, func(state th.State) bool { return !state.Throttles[0].Hidden[0] })
	if !panel.functionCells[0].Visible() || panel.functions[0].Text != "Headlight" || panel.functionColumns != 6 {
		t.Fatal("show lost label")
	}
	if err := s.Post(func(c *th.Controller) error {
		for n := 0; n < 29; n++ {
			if err := c.SetFunctionHidden(3, n, true); err != nil {
				return err
			}
		}
		return c.AddCab(7)
	}); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 2 })
	if !panel.noFunctions.Visible() || panel.shown[0].Checked || !v.panels[7].functionCells[0].Visible() {
		t.Fatal("visibility leaked or empty hint missing")
	}
	for n := range panel.functions {
		if panel.functionCells[n].Visible() {
			t.Fatalf("F%d still visible", n)
		}
	}
	panel.shown[0].SetChecked(true)
	panel.shown[3].SetChecked(true)
	panel.labels[3].SetText("Whistle")
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Labels[3] == "Whistle" })
	if panel.noFunctions.Visible() {
		t.Fatal("empty hint not cleared")
	}
	v.tabs.SelectIndex(1)
	v.runTabs.Select(panel.tab)
	v.Window.Resize(fyne.NewSize(1050, 840))
	v.Window.Show()
	if path := os.Getenv("DCCEX_SETUP_SCREENSHOT"); path != "" {
		captureSetup(t, v, path)
	}
	panel.setup.Hide()
	if path := os.Getenv("DCCEX_RUN_SCREENSHOT"); path != "" {
		captureSetup(t, v, path)
	}
}

func captureSetup(t *testing.T, v *View, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, v.Window.Canvas().Capture()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
