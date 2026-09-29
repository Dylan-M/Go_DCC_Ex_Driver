package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/startup"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/stations"
	bolt "go.etcd.io/bbolt"
	logpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/protobuf/proto"
)

type lifecycleTestDriver struct {
	fyne.Driver
	queue   chan func()
	stopped chan struct{}
}

func (d *lifecycleTestDriver) DoFromGoroutine(fn func(), wait bool) {
	done := make(chan struct{})
	select {
	case d.queue <- func() { fn(); close(done) }:
	case <-d.stopped:
		return
	}
	if wait {
		select {
		case <-done:
		case <-d.stopped:
		}
	}
}

type lifecycleTestStorage struct {
	fyne.Storage
	root fyne.URI
}

func (s lifecycleTestStorage) RootURI() fyne.URI { return s.root }

type lifecycleTestApp struct {
	fyne.App
	driver *lifecycleTestDriver
	store  fyne.Storage
	window *lifecycleTestWindow
}

func (a *lifecycleTestApp) Driver() fyne.Driver   { return a.driver }
func (a *lifecycleTestApp) Storage() fyne.Storage { return a.store }
func (a *lifecycleTestApp) NewWindow(title string) fyne.Window {
	a.window.Window = a.App.NewWindow(title)
	return a.window
}

type lifecycleTestWindow struct {
	fyne.Window
	onRun     func()
	intercept func()
	closed    bool
}

func (w *lifecycleTestWindow) SetCloseIntercept(fn func()) { w.intercept = fn }
func (w *lifecycleTestWindow) ShowAndRun()                 { w.onRun() }
func (w *lifecycleTestWindow) Close()                      { w.closed = true; w.Window.Close() }

func TestApplicationTelemetryLifecycleAndCorruptSettings(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled", true: "corrupt"}[corrupt], func(t *testing.T) {
			var mu sync.Mutex
			var events []string
			collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if r.URL.Path == "/v1/logs" {
					var report logpb.ExportLogsServiceRequest
					if err := proto.Unmarshal(body, &report); err != nil {
						t.Error(err)
					}
					mu.Lock()
					for _, resource := range report.ResourceLogs {
						for _, scope := range resource.ScopeLogs {
							for _, record := range scope.LogRecords {
								events = append(events, record.EventName)
							}
						}
					}
					mu.Unlock()
				}
				w.Header().Set("Content-Type", "application/x-protobuf")
			}))
			defer collector.Close()
			root := t.TempDir()
			path := filepath.Join(root, "stations.db")
			db, err := stations.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			settings := config.DefaultTelemetry()
			settings.Enabled, settings.Endpoint = true, collector.URL
			if err := db.SaveTelemetry(settings); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if corrupt {
				raw, err := bolt.Open(path, 0600, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := raw.Update(func(tx *bolt.Tx) error {
					return tx.Bucket([]byte("preferences")).Put([]byte("telemetry"), []byte("broken"))
				}); err != nil {
					t.Fatal(err)
				}
				if err := raw.Close(); err != nil {
					t.Fatal(err)
				}
			}
			base := test.NewTempApp(t)
			driver := &lifecycleTestDriver{Driver: base.Driver(), queue: make(chan func()), stopped: make(chan struct{})}
			window := new(lifecycleTestWindow)
			a := &lifecycleTestApp{App: base, driver: driver, window: window, store: lifecycleTestStorage{Storage: base.Storage(), root: storage.NewFileURI(root)}}
			fyne.SetCurrentApp(a)
			window.onRun = func() {
				defer close(driver.stopped)
				life := a.Lifecycle().(interface {
					OnStarted() func()
					OnEnteredForeground() func()
					OnExitedForeground() func()
				})
				life.OnEnteredForeground()()
				life.OnExitedForeground()()
				life.OnStarted()()
				closing := false
				deadline := time.NewTimer(5 * time.Second)
				defer deadline.Stop()
				for !window.closed {
					select {
					case fn := <-driver.queue:
						fn()
						if !closing && (!corrupt || startupConsoleContains(window.Content(), "Telemetry settings unavailable; export disabled")) {
							closing = true
							window.intercept()
							window.intercept() // Repeated close requests must be harmless.
						}
					case <-deadline.C:
						t.Fatal("startup warning or orderly close did not complete")
					}
				}
				if window.intercept != nil {
					t.Fatal("close intercept was not removed")
				}
			}
			if err := runApplication(startup.Options{}, a); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if corrupt {
				if len(events) != 0 {
					t.Fatal("corrupt settings enabled export")
				}
			} else {
				for _, want := range []string{"application.started", "application.foreground", "application.background", "session.stopped", "application.stopped"} {
					if !strings.Contains(strings.Join(events, "\n"), want) {
						t.Error("missing lifecycle event", want)
					}
				}
			}
		})
	}
}

func startupConsoleContains(root fyne.CanvasObject, text string) bool {
	switch o := root.(type) {
	case *container.Split:
		return startupConsoleContains(o.Trailing, text)
	case *fyne.Container:
		for _, child := range o.Objects {
			if startupConsoleContains(child, text) {
				return true
			}
		}
	case *widget.List:
		for i := 0; i < o.Length(); i++ {
			item := o.CreateItem()
			o.UpdateItem(i, item)
			if strings.Contains(item.(*canvas.Text).Text, text) {
				return true
			}
		}
	}
	return false
}
