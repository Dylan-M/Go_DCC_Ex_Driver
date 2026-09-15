package main

import (
	"fyne.io/fyne/v2/storage"
	"path/filepath"
	"testing"
)

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
