//go:build ci

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
)

func TestHeadlessApplicationStartupUsesTabDatabase(t *testing.T) {
	// Fyne's ci driver stores app data under os.TempDir()/fyne-test and its
	// ShowAndRun returns immediately. Isolate that root from every other app.
	root := t.TempDir()
	for _, variable := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(variable, root)
	}
	if err := run(nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "fyne-test", "com.github.Dylan-M.Go_DCC_Ex_Driver", "stations.db")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	db, err := stations.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.LoadThrottles(); err != nil {
		t.Fatal(err)
	}
}
