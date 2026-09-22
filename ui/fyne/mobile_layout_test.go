package fyneui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestMobileLayout(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("Android layout")
	s := th.NewSession(config.Default(), nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
	v := newView(w, s, true)
	renderUntil(t, v, s, func(state th.State) bool { return len(state.Throttles) == 1 })
	w.Show()
	for _, size := range []fyne.Size{fyne.NewSize(360, 800), fyne.NewSize(800, 360)} {
		w.Resize(size)
		for i, tab := range v.tabs.Items {
			v.tabs.SelectIndex(i)
			if min := tab.Content.MinSize(); min.Width > size.Width {
				t.Errorf("tab %s minimum %v exceeds screen %v", tab.Text, min, size)
			}
		}
	}
	w.Resize(fyne.NewSize(360, 800))
	v.tabs.SelectIndex(1)
	panel := v.panels[3]
	if panel.functionColumns != 2 {
		t.Fatal(panel.functionColumns)
	}
	if err := s.SetFunctionLabel(3, 0, "Headlights"); err != nil {
		t.Fatal(err)
	}
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Labels[0] == "Headlights" })
	if panel.functionColumns != 2 {
		t.Fatal("custom label changed mobile columns")
	}
	if panel.functionCells[2].Position().Y <= panel.functionCells[0].Position().Y {
		t.Fatal("third function did not wrap")
	}
	if panel.functionCells[1].Position().X <= panel.functionCells[0].Position().X {
		t.Fatal("second function not beside first")
	}
	test.Tap(panel.setupButton)
	if panel.setup == nil {
		t.Fatal("setup not opened")
	}
	if size := panel.setup.MinSize(); size.Width > 360 {
		t.Fatalf("setup minimum too wide: %v", size)
	}
	panel.name.SetText("Phone loco")
	renderUntil(t, v, s, func(state th.State) bool { return state.Throttles[0].Name == "Phone loco" })
	panel.setup.Hide()
	if len(v.tabs.Items) != 2 || v.programmingTabs != nil || v.mainPower != nil {
		t.Fatal("mobile must expose only Engineer controls")
	}
	if path := os.Getenv("DCCEX_MOBILE_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		encodeErr := png.Encode(file, w.Canvas().Capture())
		closeErr := file.Close()
		if encodeErr != nil || closeErr != nil {
			t.Fatal(encodeErr, closeErr)
		}
	}
}

func TestMobileSavedStationsAreTCPOnly(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("stations")
	db, err := stations.Open(filepath.Join(t.TempDir(), "stations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, p := range []stations.Profile{
		{Name: "TCP", Mode: "TCP", Host: "station.local", Port: 2560},
		{Name: "USB", Mode: "Serial", Device: "COM7", Baud: 115200},
	} {
		if err := db.Save(p, false); err != nil {
			t.Fatal(err)
		}
	}
	s := th.NewSession(config.Default(), nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
	v := newView(w, s, true, Options{Stations: db})
	if len(v.mode.Options) != 1 || v.mode.Options[0] != "TCP" {
		t.Fatal(v.mode.Options)
	}
	if len(v.savedStations.Options) != 1 || v.savedStations.Options[0] != "TCP" {
		t.Fatal(v.savedStations.Options)
	}
	v.savedStations.SetSelected("TCP")
	if v.host.Text != "station.local" {
		t.Fatal(v.host.Text)
	}
	list, err := db.List()
	if err != nil || len(list) != 2 {
		t.Fatal("hidden serial profile modified", list, err)
	}
}

func TestMobileButtonRows(t *testing.T) {
	test.NewTempApp(t)
	one, two, three := widget.NewButton("One", nil), widget.NewButton("Two", nil), widget.NewButton("Three", nil)
	row := buttonRows(one, two, three)
	row.Resize(fyne.NewSize(360, row.MinSize().Height))
	if one.Position().Y != two.Position().Y || three.Position().Y <= one.Position().Y || three.Position().X != one.Position().X {
		t.Fatal("expected exactly two buttons per row")
	}
	if one.Size().Width != three.Size().Width {
		t.Fatal("odd final row expanded")
	}
}
