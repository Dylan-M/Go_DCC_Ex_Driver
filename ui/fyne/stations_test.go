package fyneui

import (
	"errors"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func TestSavedStationsAndStartupFields(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("stations")
	db, err := stations.Open(filepath.Join(t.TempDir(), "stations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p := stations.Profile{Name: "Layout", Mode: "TCP", Host: "station.local", Port: 2560}
	if err := db.Save(p, false); err != nil {
		t.Fatal(err)
	}
	s := th.NewSession(config.Default(), nil, nil)
	t.Cleanup(func() { s.Close(); <-s.Done(); w.Close() })
	v := New(w, s, Options{Host: "localhost", Port: 5555, Stations: db})
	if v.host.Text != "localhost" || v.port.Text != "5555" || v.savedStations.Selected != "" {
		t.Fatal("saved profile overwrote startup overrides")
	}
	v.savedStations.SetSelected("Layout")
	if v.host.Text != "station.local" || v.port.Text != "2560" || v.mode.Selected != "TCP" {
		t.Fatal("profile not loaded")
	}
	if v.last.Connected {
		t.Fatal("loading a station must not auto-connect")
	}
	v.port.SetText("7000")
	updated, err := v.stationProfile("Layout")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.persistStation(updated, false); !errors.Is(err, stations.ErrExists) {
		t.Fatal("duplicate not rejected", err)
	}
	before, _ := db.List()
	if before[0].Port != 2560 {
		t.Fatal("unconfirmed overwrite")
	}
	if err := v.persistStation(updated, true); err != nil {
		t.Fatal(err)
	}
	v.mode.SetSelected("Serial")
	v.devices.SetText("COM7")
	v.baud.SetText("115200")
	serial, err := v.stationProfile("USB")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.persistStation(serial, false); err != nil {
		t.Fatal(err)
	}
	v.savedStations.SetSelected("Layout")
	v.savedStations.SetSelected("USB")
	if v.mode.Selected != "Serial" || v.devices.Text != "COM7" || v.baud.Text != "115200" {
		t.Fatal("serial profile not restored")
	}
	if err := v.removeStation("USB"); err != nil {
		t.Fatal(err)
	}
	if len(v.savedStations.Options) != 1 || v.savedStations.Selected != "" || !v.deleteStation.Disabled() {
		t.Fatal("delete UI stale")
	}
	v.savedStations.SetSelected("Layout")
	if v.port.Text != "7000" {
		t.Fatal("replacement not retained")
	}
	v.port.SetText("0")
	if _, err := v.stationProfile("bad"); err == nil {
		t.Fatal("invalid port accepted")
	}
}
