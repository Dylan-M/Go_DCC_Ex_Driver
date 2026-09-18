package main

import (
	"bytes"
	"errors"
	"fyne.io/fyne/v2/storage"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/startup"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	bolt "go.etcd.io/bbolt"
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

func TestInvalidDatabaseLayoutDisablesSaving(t *testing.T) {
	for _, contents := range []string{"broken", `{"version":99,"tabs":[{"address":42}],"selected":42}`} {
		t.Run(contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stations.db")
			db, err := stations.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			db.Close()
			raw, err := bolt.Open(path, 0600, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := raw.Update(func(tx *bolt.Tx) error {
				bucket, err := tx.CreateBucketIfNotExists([]byte("preferences"))
				if err != nil {
					return err
				}
				return bucket.Put([]byte("throttles"), []byte(contents))
			}); err != nil {
				t.Fatal(err)
			}
			raw.Close()
			db, err = stations.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			persistence, err := loadTabPersistence(db)
			if err == nil || persistence.Save != nil || !reflect.DeepEqual(persistence.Initial, config.DefaultThrottles()) {
				t.Fatal("invalid database record enabled writes", err)
			}
		})
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

func TestTabStorageUsesDatabaseAndIgnoresLegacyFiles(t *testing.T) {
	root := t.TempDir()
	db, err := stations.Open(filepath.Join(root, "stations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := filepath.Join(root, "dccex-throttle.json")
	legacyBytes := []byte(`{"locos":[{"address":42,"name":"Python loco"}]}`)
	if err := os.WriteFile(legacy, legacyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "throttles.json")
	contents := `{"version":99,"tabs":[{"address":42}],"selected":42}`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	persistence, err := loadTabPersistence(db)
	if err != nil || persistence.Save == nil || !reflect.DeepEqual(persistence.Initial, config.DefaultThrottles()) {
		t.Fatal(persistence, err)
	}
	want := config.ThrottleSettings{Version: 1, Tabs: []config.ThrottleTab{{Address: 42}}, Selected: 42}
	if err := persistence.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(got, legacyBytes) {
		t.Fatal("legacy configuration changed", err)
	}
	got, err = os.ReadFile(path)
	if err != nil || string(got) != contents {
		t.Fatal("old layout changed", err)
	}
	persistence, err = loadTabPersistence(db)
	if err != nil || !reflect.DeepEqual(persistence.Initial, want) {
		t.Fatal("database layout not restored", persistence, err)
	}
	db.Close()
	persistence, err = loadTabPersistence(db)
	if err == nil || persistence.Save != nil || !reflect.DeepEqual(persistence.Initial, config.DefaultThrottles()) {
		t.Fatal("unavailable database enabled writes", err)
	}
}
