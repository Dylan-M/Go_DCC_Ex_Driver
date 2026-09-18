package main

import (
	"bytes"
	"errors"
	"fyne.io/fyne/v2/storage"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/startup"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConnectOnLaunch(t *testing.T) {
	for _, args := range [][]string{nil, {"--host", "localhost", "--port", "50825"}, {"--host", "localhost"}, {"--port", "50825"}} {
		options, err := startup.Parse(args, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		for _, failure := range []error{nil, errors.New("could not queue connection")} {
			calls := 0
			err := connectOnLaunch(options, func(c throttle.Connection) error {
				calls++
				if c.Serial || c.Host != options.Host || c.Port != options.Port {
					t.Fatal("incorrect startup endpoint", c)
				}
				return failure
			})
			if options.AutoConnect {
				if calls != 1 || !errors.Is(err, failure) {
					t.Fatal("expected exactly one connection attempt", calls, err)
				}
			} else if calls != 0 || err != nil {
				t.Fatal("ordinary launch must not connect", calls, err)
			}
		}
	}
}

func TestStationDatabasePath(t *testing.T) {
	root := t.TempDir()
	got, err := stationDatabasePath(storage.NewFileURI(root))
	if err != nil || got != filepath.Join(root, "stations.db") {
		t.Fatal(got, err)
	}
	if _, err := stationDatabasePath(storage.NewURI("https://example.com/files")); err == nil {
		t.Fatal("remote storage accepted")
	}
}

func TestHelpAndInvalidArgumentsDoNotLaunch(t *testing.T) {
	if err := run([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--port", "0"}); err == nil {
		t.Fatal("invalid argument accepted")
	}
}

func TestTabStorageProtectsInvalidAndLegacyFiles(t *testing.T) {
	root := t.TempDir()
	uri := storage.NewFileURI(root)
	legacy := filepath.Join(root, "dccex-throttle.json")
	legacyBytes := []byte(`{"locos":[{"address":42,"name":"Python loco"}]}`)
	if err := os.WriteFile(legacy, legacyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	persistence, err := loadTabPersistence(uri)
	if err != nil || persistence.Save == nil || !reflect.DeepEqual(persistence.Initial, config.DefaultThrottles()) {
		t.Fatal(persistence, err)
	}
	if err := persistence.Save(config.DefaultThrottles()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(got, legacyBytes) {
		t.Fatal("legacy configuration changed", err)
	}
	path := filepath.Join(root, "throttles.json")
	for _, contents := range []string{`broken`, `{"version":99,"tabs":[{"address":42}],"selected":42}`} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		persistence, err = loadTabPersistence(uri)
		if err == nil || persistence.Save != nil || !reflect.DeepEqual(persistence.Initial, config.DefaultThrottles()) {
			t.Fatal("invalid file enabled writes", err)
		}
		got, err = os.ReadFile(path)
		if err != nil || string(got) != contents {
			t.Fatal("invalid file changed", err)
		}
	}
	if _, err := loadTabPersistence(storage.NewURI("https://example.com/files")); err == nil {
		t.Fatal("remote storage accepted")
	}
}
