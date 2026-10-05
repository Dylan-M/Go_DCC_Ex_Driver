package fyneui

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

// TestDocumentationScreenshots renders the real widgets with fixed demo data.
// Ordinary test runs use temporary output; docs and CI opt into a retained path.
// These images illustrate the UI, not physical hardware or emulator validation.
func TestDocumentationScreenshots(t *testing.T) {
	output := os.Getenv("DCCEX_DOC_SCREENSHOTS")
	if output == "" {
		output = t.TempDir()
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	for _, scene := range []struct {
		name          string
		mobile, power bool
		tab           int
	}{
		{name: "desktop-run", tab: 1},
		{name: "desktop-connection", power: true},
		{name: "desktop-programming", power: true, tab: 2},
		{name: "desktop-pom", power: true, tab: 2},
		{name: "desktop-consists", power: true, tab: 3},
		{name: "mobile-run", mobile: true, tab: 1},
		{name: "mobile-telemetry", mobile: true},
	} {
		t.Run(scene.name, func(t *testing.T) {
			a := test.NewTempApp(t)
			test.ApplyTheme(t, theme.LightTheme())
			w := a.NewWindow("DCC-EX Native Throttle")
			observer := telemetry.New()
			s := th.NewObservedSession(config.Default(), func(context.Context, th.Connection) (io.ReadWriteCloser, error) {
				t.Error("documentation must not connect to a command station")
				return nil, errors.New("documentation fixture has no transport")
			}, observer)
			t.Cleanup(func() {
				s.Close()
				<-s.Done()
				w.Close()
				observer.Close()
			})
			db, err := stations.Open(filepath.Join(t.TempDir(), "stations.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := db.Save(stations.Profile{Name: "Club layout", Mode: "TCP", Host: "192.0.2.10", Port: 2560}, false); err != nil {
				t.Fatal(err)
			}
			v := newView(w, s, scene.mobile, Options{PowerThrottle: scene.power, Stations: db, Host: "192.0.2.10", Port: 2560,
				Telemetry: observer, SaveTelemetry: db.SaveTelemetry})
			t.Cleanup(v.Close)
			v.savedStations.SetSelected("Club layout")
			v.Render(documentationState())
			v.tabs.SelectIndex(scene.tab)
			size := fyne.NewSize(1050, 840)
			if scene.mobile {
				size = fyne.NewSize(390, 844)
			}
			w.Resize(size)
			w.Show()
			switch scene.name {
			case "desktop-pom":
				v.programmingTabs.SelectIndex(1)
				v.pomAddress.SetText("42")
				v.pomCV.SetText("3")
				v.pomValue.SetText("24")
			case "mobile-telemetry":
				v.showTelemetrySettings()
			}
			// A focused entry's blinking cursor must not make doc images vary.
			w.Canvas().Unfocus()
			// Wrapped text computes its height after receiving the final width.
			// Refresh the laid-out tree before capturing the software canvas.
			w.Content().Refresh()
			w.Resize(size)
			w.Content().Refresh()
			image := w.Canvas().Capture()
			if image.Bounds().Dx() != int(size.Width) || image.Bounds().Dy() != int(size.Height) {
				t.Fatalf("unexpected screenshot size: %v", image.Bounds())
			}
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, image); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(output, scene.name+".png"), encoded.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func documentationState() th.State {
	freight := th.CabState{Cab: 3, Name: "Local freight", Speed: 38, Direction: 1}
	for n, name := range []string{"Headlight", "Bell", "Horn", "Sound", "Cab light", "Ditch lights", "Brake", "Coupler"} {
		freight.Labels[n] = name
	}
	for n := 8; n < len(freight.Hidden); n++ {
		freight.Hidden[n] = true
	}
	freight.Toggle[0], freight.Toggle[3] = true, true
	freight.Functions[0], freight.Functions[3] = true, true
	yard := freight
	yard.Cab, yard.Name, yard.Speed, yard.Direction = 42, "Yard switcher", 0, 0
	return th.State{
		Throttles: []th.CabState{freight, yard}, Connected: true,
		ActiveConnection: th.Connection{Host: "192.0.2.10", Port: 2560}, Status: "Connected to Club layout (demo)",
		Cab: 3, Speed: freight.Speed, Direction: freight.Direction, Functions: freight.Functions, Toggle: freight.Toggle,
		MainPower: p.On, ProgPower: p.Off, CurrentMA: 350, TripMA: 2000, HasCurrent: true, Poll: true,
		ProgramAddress: "3", ProgramValue: "6", ProgramResult: "CV29 (Configuration) = 6", CV29: 6, CV29Known: true,
		Logs: []th.LogEntry{{Kind: "info", Text: "Documentation preview — fixed demo data; no hardware connected."}},
	}
}
