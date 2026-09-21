//go:build ci

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
)

func TestHeadlessApplicationStartupUsesTabDatabase(t *testing.T) {
	// The headless driver returns from ShowAndRun immediately, but Fyne's
	// preference watchers keep running and recreate deleted directories. Run
	// the real startup in a child process, then remove its storage only after
	// process exit has stopped every watcher. This also isolates global app state.
	if os.Getenv("DCCEX_STARTUP_TEST_CHILD") == "1" {
		if err := run(nil); err != nil {
			t.Fatal(err)
		}
		return
	}
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, executable, "-test.run=^TestHeadlessApplicationStartupUsesTabDatabase$", "-test.count=1")
	child.Env = append(os.Environ(), "DCCEX_STARTUP_TEST_CHILD=1", "TMP="+root, "TEMP="+root, "TMPDIR="+root)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("headless startup failed: %v\n%s", err, output)
	}
	// Check the actual database after process exit, not just the child's status.
	path := filepath.Join(root, "fyne-test", "com.github.Dylan-M.Go_DCC_Ex_Driver", "stations.db")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	db, err := stations.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.LoadThrottles(); err != nil {
		t.Fatal(err)
	}
}
