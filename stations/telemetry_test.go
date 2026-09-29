package stations

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
)

func TestTelemetryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stations.db")
	observer := telemetry.New()
	defer observer.Close()
	db, err := Open(path, observer)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := db.LoadTelemetry()
	if err != nil || initial != config.DefaultTelemetry() {
		t.Fatal(initial, err)
	}
	want := initial
	want.Enabled, want.Debug, want.Endpoint = true, true, "https://collector.example/otel"
	want.SampleRatio = .25
	if err := db.SaveTelemetry(want); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.LoadTelemetry()
	if err != nil || got != want {
		t.Fatal(got, err)
	}
	bad := want
	bad.Version = 2
	if db.SaveTelemetry(bad) == nil {
		t.Fatal("invalid settings saved")
	}
	bad.SampleRatio = math.NaN()
	if db.SaveTelemetry(bad) == nil {
		t.Fatal("unencodable settings saved")
	}
	if got, err := db.LoadTelemetry(); err != nil || got != want {
		t.Fatal("rejected settings replaced the last valid record", got, err)
	}
	for _, data := range []string{`broken`, `{"version":99}`, `{"version":1,"enabled":true,"endpoint":""}`} {
		if err := db.writePreference("telemetry", []byte(data)); err != nil {
			t.Fatal(err)
		}
		got, err := db.LoadTelemetry()
		if err == nil || got != config.DefaultTelemetry() {
			t.Fatal("invalid settings enabled export", got, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.LoadTelemetry(); err == nil {
		t.Fatal("closed read succeeded")
	}
	if err := db.SaveTelemetry(want); err == nil {
		t.Fatal("closed write succeeded")
	}
}
