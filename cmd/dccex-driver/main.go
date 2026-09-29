package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/startup"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	fyneui "github.com/Dylan-M/Go_DCC_Ex_Driver/ui/fyne"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string) error {
	options, err := startup.Parse(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	return runApplication(options, app.NewWithID("com.github.Dylan-M.Go_DCC_Ex_Driver"))
}

func runApplication(options startup.Options, a fyne.App) error {
	observer := telemetry.New(a.Metadata().Version)
	defer observer.Close()
	var saveTelemetry func(config.TelemetrySettings) error
	var telemetryErr error
	settings := config.Default()
	var tabErr error
	tabPersistence := throttle.TabPersistence{Initial: config.DefaultThrottles()}
	var saved stations.Repository
	dbPath, dbErr := stationDatabasePath(a.Storage().RootURI())
	if dbErr == nil {
		var db *stations.Store
		db, dbErr = stations.Open(dbPath, observer)
		if dbErr == nil {
			saved = db
			defer db.Close()
			tabPersistence, tabErr = loadTabPersistence(db)
			var savedTelemetry config.TelemetrySettings
			savedTelemetry, telemetryErr = db.LoadTelemetry()
			if telemetryErr == nil {
				saveTelemetry = db.SaveTelemetry
				telemetryErr = observer.Configure(savedTelemetry, nil)
			}
		}
	}
	window := a.NewWindow("DCC-EX Native Throttle")
	observer.Event(context.Background(), "application.started")
	defer observer.Event(context.Background(), "application.stopped")
	session := throttle.NewObservedSession(settings, nil, observer, tabPersistence)
	view := fyneui.New(window, session, fyneui.Options{Host: options.Host, Port: options.Port, Stations: saved, PowerThrottle: options.PowerThrottle, Telemetry: observer, SaveTelemetry: saveTelemetry})
	defer view.Close()
	if telemetryErr != nil {
		session.Post(func(c *throttle.Controller) error {
			c.Log("err", "Telemetry settings unavailable; export disabled: "+telemetryErr.Error())
			return nil
		})
	}
	if tabErr != nil {
		session.Post(func(c *throttle.Controller) error {
			c.Log("err", "Saved throttles unavailable; changes will not be saved this session: "+tabErr.Error())
			return nil
		})
	}
	if dbErr != nil {
		session.Post(func(c *throttle.Controller) error {
			c.Log("err", "Local database unavailable; stations, tabs and function settings will not be saved: "+dbErr.Error())
			return nil
		})
	}
	if err := connectOnLaunch(options, session.Connect); err != nil {
		session.Post(func(c *throttle.Controller) error {
			c.Log("err", "Startup connection: "+err.Error())
			return nil
		})
	}
	a.Lifecycle().SetOnStarted(stateRenderer(session.Updates(), view.Render, fyne.DoAndWait))
	a.Lifecycle().SetOnEnteredForeground(func() { observer.Event(context.Background(), "application.foreground") })
	a.Lifecycle().SetOnExitedForeground(func() {
		observer.Event(context.Background(), "application.background")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			// Export failures are counted by the telemetry transport. They do not
			// affect the connection or need an interrupting UI dialog.
			_ = observer.Flush(ctx)
		}()
	})
	closing := false
	window.SetCloseIntercept(func() {
		if closing {
			return
		}
		closing = true
		view.Close()
		session.Close()
		go func() { <-session.Done(); fyne.Do(func() { window.SetCloseIntercept(nil); window.Close() }) }()
	})
	window.ShowAndRun()
	session.Close()
	<-session.Done()
	return nil
}

// Session.Connect queues a single asynchronous attempt. Failure is reported by
// the existing session/console path, leaving manual connection available.
func connectOnLaunch(options startup.Options, connect func(throttle.Connection) error) error {
	if !options.AutoConnect {
		return nil
	}
	return connect(throttle.Connection{Host: options.Host, Port: options.Port})
}

func stationDatabasePath(root fyne.URI) (string, error) {
	return appStoragePath(root, "stations.db")
}

func loadTabPersistence(db *stations.Store) (throttle.TabPersistence, error) {
	persistence := throttle.TabPersistence{Initial: config.DefaultThrottles()}
	settings, err := db.LoadThrottles()
	if err != nil {
		// Do not replace damaged or newer settings with fallback defaults.
		return persistence, err
	}
	persistence.Initial = settings
	persistence.Save = db.SaveThrottles
	return persistence, nil
}

func appStoragePath(root fyne.URI, name string) (string, error) {
	if root == nil || root.Scheme() != "file" {
		return "", errors.New("app storage must be a local directory")
	}
	path := root.Path()
	if runtime.GOOS == "windows" && len(path) > 3 && path[0] == '/' && path[2] == ':' {
		path = strings.TrimPrefix(path, "/")
	}
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		return "", errors.New("app storage path must be absolute")
	}
	return filepath.Join(path, name), nil
}
