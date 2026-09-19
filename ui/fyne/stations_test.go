package fyneui

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	p "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/protocol"
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
	s := th.NewSession(config.Default(), nil)
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
	if v.mode.Selected != "Serial" || v.devices.Text != "COM7" || v.baud.Text != "115200" {
		t.Fatal("disconnected deletion should leave editable fields unchanged")
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

func TestDeleteStationRestoresActiveConnection(t *testing.T) {
	for _, active := range []th.Connection{
		{Host: "localhost", Port: 54942},
		{Serial: true, Device: "COM9", Baud: 57600},
	} {
		name := "TCP"
		if active.Serial {
			name = "Serial"
		}
		t.Run(name, func(t *testing.T) {
			a := test.NewTempApp(t)
			w := a.NewWindow("Delete saved connection")
			db, err := stations.Open(filepath.Join(t.TempDir(), "stations.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			dead := stations.Profile{Name: "Dead", Mode: "Serial", Device: "COM2", Baud: 115200}
			if active.Serial {
				dead = stations.Profile{Name: "Dead", Mode: "TCP", Host: "dead.local", Port: 2560}
			}
			if err := db.Save(dead, false); err != nil {
				t.Fatal(err)
			}
			conn, peer := net.Pipe()
			commands := make(chan string, 32)
			done := make(chan struct{})
			go func() {
				defer close(done)
				scan := bufio.NewScanner(peer)
				for scan.Scan() {
					commands <- scan.Text()
				}
			}()
			var opens atomic.Int32
			s := th.NewSession(config.Default(), func(_ context.Context, got th.Connection) (io.ReadWriteCloser, error) {
				opens.Add(1)
				return conn, nil
			})
			t.Cleanup(func() { s.Close(); <-s.Done(); peer.Close(); <-done; w.Close() })
			v := New(w, s, Options{Stations: db})
			if err := s.Post(func(c *th.Controller) error { c.SetPoll(false); return nil }); err != nil {
				t.Fatal(err)
			}
			// Connect directly, as the CLI path does, not through form fields.
			if err := s.Connect(active); err != nil {
				t.Fatal(err)
			}
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			for !v.last.Connected {
				select {
				case state := <-s.Updates():
					v.Render(state)
				case <-deadline.C:
					t.Fatal("connect timeout")
				}
			}
			if v.last.ActiveConnection != active {
				t.Fatal("session lost actual endpoint")
			}
			expect := func(want string) {
				t.Helper()
				select {
				case got := <-commands:
					if got != want {
						t.Fatalf("command %q, want %q", got, want)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("command timeout", want)
				}
			}
			expect("<=>")
			expect("<s>")
			expect("<t 3>")
			v.savedStations.SetSelected("Dead")
			if v.mode.Selected != dead.Mode {
				t.Fatal("dead profile did not load")
			}
			if err := v.removeStation("Dead"); err != nil {
				t.Fatal(err)
			}
			if active.Serial {
				if v.mode.Selected != "Serial" || v.devices.Text != active.Device || v.baud.Text != strconv.Itoa(active.Baud) {
					t.Fatal("serial endpoint not restored")
				}
			} else {
				if v.mode.Selected != "TCP" || v.host.Text != active.Host || v.port.Text != strconv.Itoa(active.Port) {
					t.Fatal("TCP endpoint not restored")
				}
			}
			if v.savedStations.Selected != "" || !v.deleteStation.Disabled() {
				t.Fatal("deleted profile still selected")
			}
			if list, err := db.List(); err != nil || len(list) != 0 {
				t.Fatal("profile not deleted", err)
			}
			// The original transport still works, with no disconnect/reconnect
			// or extra commands caused by profile selection/deletion.
			if err := s.Post(func(c *th.Controller) error { return c.Power(true, p.Main) }); err != nil {
				t.Fatal(err)
			}
			expect("<1 MAIN>")
			if opens.Load() != 1 {
				t.Fatal("delete reopened the connection")
			}
		})
	}
}
