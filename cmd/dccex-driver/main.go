package main

import (
	"errors"
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/startup"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
	fyneui "github.com/Dylan-M/Go_DCC_Ex_Driver/ui/fyne"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	settings, path, loadErr := config.LoadDesktop()
	a := app.NewWithID("com.github.Dylan-M.Go_DCC_Ex_Driver")
	tabPersistence, tabErr := loadTabPersistence(a.Storage().RootURI())
	var saved stations.Repository
	dbPath, dbErr := stationDatabasePath(a.Storage().RootURI())
	if dbErr == nil {
		var db *stations.Store
		db, dbErr = stations.Open(dbPath)
		if dbErr == nil {
			saved = db
			defer db.Close()
		}
	}
	window := a.NewWindow("DCC-EX Native Throttle")
	session := throttle.NewSession(settings, nil, func(s config.Settings) error { return config.Save(path, s) }, tabPersistence)
	view := fyneui.New(window, session, fyneui.Options{Host: options.Host, Port: options.Port, Stations: saved})
	if tabErr != nil {
		session.Post(func(c *throttle.Controller) error {
			c.Log("err", "Saved throttles unavailable; changes will not be saved this session: "+tabErr.Error())
			return nil
		})
	}
	if dbErr != nil {
		session.Post(func(c *throttle.Controller) error {
			c.Log("err", "Saved stations unavailable: "+dbErr.Error())
			return nil
		})
	}
	if loadErr != nil {
		session.Post(func(c *throttle.Controller) error { c.Log("err", "Configuration: "+loadErr.Error()); return nil })
	}
	if err := connectOnLaunch(options, session.Connect); err != nil {
		session.Post(func(c *throttle.Controller) error {
			c.Log("err", "Startup connection: "+err.Error())
			return nil
		})
	}
	go func() {
		for state := range session.Updates() {
			s := state
			fyne.DoAndWait(func() { view.Render(s) })
		}
	}()
	closing := false
	window.SetCloseIntercept(func() {
		if closing {
			return
		}
		closing = true
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

func loadTabPersistence(root fyne.URI) (throttle.TabPersistence, error) {
	persistence := throttle.TabPersistence{Initial: config.DefaultThrottles()}
	path, err := appStoragePath(root, "throttles.json")
	if err != nil {
		return persistence, err
	}
	settings, err := config.LoadThrottles(path)
	if err != nil {
		// Do not replace damaged or newer settings with fallback defaults.
		return persistence, err
	}
	persistence.Initial = settings
	persistence.Save = func(s config.ThrottleSettings) error { return config.SaveThrottles(path, s) }
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
