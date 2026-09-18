package stations

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
)

func TestPreferencesShareStationDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stations.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	station := Profile{Name: "Layout", Mode: "TCP", Host: "localhost", Port: 2560}
	if err := db.Save(station, false); err != nil {
		t.Fatal(err)
	}
	// Existing station databases have no preferences bucket.
	if got, err := db.LoadThrottles(); err != nil || !reflect.DeepEqual(got, config.DefaultThrottles()) {
		t.Fatal(got, err)
	}
	if got, err := db.LoadSettings(); err != nil || got != config.Default() {
		t.Fatal(got, err)
	}
	layout := config.ThrottleSettings{Version: 1, Tabs: []config.ThrottleTab{{Address: 42}, {Address: 7}}, Selected: 7}
	settings := config.Settings{}
	settings.Toggle[0], settings.Toggle[28] = true, true
	if err := db.SaveThrottles(layout); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveThrottles(config.ThrottleSettings{Version: 99}); err == nil {
		t.Fatal("invalid layout accepted")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := db.LoadThrottles(); err != nil || !reflect.DeepEqual(got, layout) {
		t.Fatal(got, err)
	}
	if got, err := db.LoadSettings(); err != nil || got != settings {
		t.Fatal(got, err)
	}
	if got, err := db.List(); err != nil || !reflect.DeepEqual(got, []Profile{station}) {
		t.Fatal(got, err)
	}
	if err := db.Delete(station.Name); err != nil {
		t.Fatal(err)
	}
	if got, err := db.LoadThrottles(); err != nil || !reflect.DeepEqual(got, layout) {
		t.Fatal(got, err)
	}
}

func TestInvalidPreferencesReturnDefaultsWithoutChangingRecords(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "stations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, contents := range []string{"broken", `{"version":99,"tabs":[{"address":42}],"selected":42}`} {
		if err := db.writePreference("throttles", []byte(contents)); err != nil {
			t.Fatal(err)
		}
		got, err := db.LoadThrottles()
		if err == nil || !reflect.DeepEqual(got, config.DefaultThrottles()) {
			t.Fatal(got, err)
		}
		if err := db.readPreference("throttles", func(data []byte) error {
			if string(data) != contents {
				t.Fatal("invalid record changed")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.writePreference("function-modes", []byte("broken")); err != nil {
		t.Fatal(err)
	}
	if got, err := db.LoadSettings(); err == nil || got != config.Default() {
		t.Fatal(got, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSettings(config.Default()); err == nil {
		t.Fatal("closed database accepted write")
	}
	if err := db.SaveThrottles(config.DefaultThrottles()); err == nil {
		t.Fatal("closed database accepted write")
	}
}
