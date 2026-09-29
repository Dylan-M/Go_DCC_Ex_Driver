package throttle

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	"go.opentelemetry.io/otel/trace"
	logpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	spanpb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

type observedCapture struct {
	mu     sync.Mutex
	spans  []*spanpb.Span
	events []string
}

func captureTelemetry(t *testing.T) (*telemetry.Manager, *observedCapture) {
	t.Helper()
	capture := new(observedCapture)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		capture.mu.Lock()
		defer capture.mu.Unlock()
		switch r.URL.Path {
		case "/v1/traces":
			var report tracepb.ExportTraceServiceRequest
			if err := proto.Unmarshal(body, &report); err != nil {
				t.Error(err)
			}
			for _, resource := range report.ResourceSpans {
				for _, scope := range resource.ScopeSpans {
					capture.spans = append(capture.spans, scope.Spans...)
				}
			}
		case "/v1/logs":
			var report logpb.ExportLogsServiceRequest
			if err := proto.Unmarshal(body, &report); err != nil {
				t.Error(err)
			}
			for _, resource := range report.ResourceLogs {
				for _, scope := range resource.ScopeLogs {
					for _, record := range scope.LogRecords {
						capture.events = append(capture.events, record.EventName)
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	t.Cleanup(server.Close)
	m := telemetry.New()
	t.Cleanup(m.Close)
	settings := config.DefaultTelemetry()
	settings.Enabled, settings.Endpoint = true, server.URL
	if err := m.Configure(settings, nil); err != nil {
		t.Fatal(err)
	}
	return m, capture
}

func TestTabTelemetryUsesTargetInsteadOfSelectedCab(t *testing.T) {
	m, capture := captureTelemetry(t)
	c := New([29]bool{})
	if err := c.AddCab(42); err != nil {
		t.Fatal(err)
	}
	if err := c.FocusCab(3); err != nil {
		t.Fatal(err)
	}
	c.telemetry = m
	for _, action := range []func() error{
		func() error { return c.RenameCab(42, "Freight") },
		func() error { return c.SetFunctionLabel(42, 2, "Horn") },
		func() error { return c.SetFunctionHidden(42, 8, true) },
		func() error { return c.WithCab(42, func(c *Controller) error { return c.SetToggle(2, true) }) },
		func() error { return c.RemoveCab(42) },
		func() error { return c.AddCab(42) },
		func() error { return c.FocusCab(3) },
		func() error { return c.ReplaceCab(42, 43) },
	} {
		if err := action(); err != nil {
			t.Fatal(err)
		}
	}
	// POM has an independent destination, including when sending fails offline.
	if err := c.POM(44, 29, 6); err == nil {
		t.Fatal("POM unexpectedly succeeded offline")
	}
	if err := m.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	seen := map[string]bool{}
	for _, span := range capture.spans {
		want := int64(42)
		if span.Name == "throttle.tab_focus" {
			continue
		}
		if span.Name == "throttle.tab_replace" {
			want = 43
		}
		if span.Name == "throttle.pom" {
			want = 44
		}
		if !strings.HasPrefix(span.Name, "throttle.") {
			continue
		}
		seen[span.Name] = true
		got := int64(-1)
		for _, attr := range span.Attributes {
			if attr.Key == "loco.address" {
				got = attr.Value.GetIntValue()
			}
		}
		if got != want {
			t.Errorf("%s target=%d, want %d", span.Name, got, want)
		}
		for _, event := range span.Events {
			if event.Name == "throttle.action" {
				for _, attr := range event.Attributes {
					if attr.Key == "loco.address" && attr.Value.GetIntValue() != want {
						t.Errorf("%s event target is wrong", span.Name)
					}
				}
			}
		}
	}
	for _, name := range []string{"tab_rename", "function_label", "function_visibility", "function_mode", "tab_remove", "tab_add", "tab_replace", "pom"} {
		if !seen["throttle."+name] {
			t.Error("missing action span", name)
		}
	}
}

func TestSessionFailureEventsBelongToLiveSpans(t *testing.T) {
	m, capture := captureTelemetry(t)
	entered, release := make(chan struct{}), make(chan struct{})
	s := NewObservedSession(config.Default(), func(context.Context, Connection) (io.ReadWriteCloser, error) {
		close(entered)
		<-release
		return nil, errors.New("refused")
	}, m)
	if err := s.Connect(Connection{}); err != nil {
		t.Fatal(err)
	}
	<-entered
	for range cap(s.actions) {
		if err := s.Post(func(*Controller) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Post(func(*Controller) error { return nil }); err == nil {
		t.Fatal("full queue accepted an action")
	}
	close(release)
	s.Close()
	<-s.Done()
	// Closing telemetry immediately after Done must still export session.stopped.
	m.Close()
	capture.mu.Lock()
	defer capture.mu.Unlock()
	for _, want := range []struct{ span, event string }{{"controller.action", "action.queue_full"}, {"connection.open", "connection.open_failed"}} {
		found := false
		for _, span := range capture.spans {
			if span.Name == want.span {
				for _, event := range span.Events {
					if event.Name == want.event {
						found = true
					}
				}
			}
		}
		if !found {
			t.Error("event missing from ended span", want)
		}
	}
	if !strings.Contains(strings.Join(capture.events, "\n"), "session.stopped") {
		t.Fatal("completion published before session.stopped")
	}
}

type failingTimerSender struct{}

func (failingTimerSender) Send(string) error { return errors.New("timer write failed") }
func (failingTimerSender) Close() error      { return nil }

func TestSessionReportsTimerFailures(t *testing.T) {
	for _, tick := range []bool{false, true} {
		m, capture := captureTelemetry(t)
		s := NewObservedSession(config.Default(), nil, m)
		if err := s.Post(func(c *Controller) error {
			c.sender = failingTimerSender{}
			c.state.Connected = true
			c.SetPoll(!tick)
			if tick {
				return c.MoveSpeed(12)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		awaitObservedState(t, s, func(state State) bool { return strings.Contains(state.Status, "timer write failed") })
		s.Close()
		<-s.Done()
		if err := m.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		want := "station.poll_failed"
		if tick {
			want = "throttle.tick_failed"
		}
		capture.mu.Lock()
		found := strings.Contains(strings.Join(capture.events, "\n"), want)
		capture.mu.Unlock()
		if !found {
			t.Fatal("timer failure event missing", want)
		}
	}
}

type tracedSender struct{ contexts []context.Context }

func (s *tracedSender) Send(string) error { return nil }
func (s *tracedSender) Close() error      { return nil }
func (s *tracedSender) SendContext(ctx context.Context, _ string) error {
	s.contexts = append(s.contexts, ctx)
	return nil
}

func TestDelayedSpeedRetainsOriginatingTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer server.Close()
	m := telemetry.New()
	defer m.Close()
	settings := config.DefaultTelemetry()
	settings.Enabled, settings.Endpoint = true, server.URL
	if err := m.Configure(settings, nil); err != nil {
		t.Fatal(err)
	}
	c := New([29]bool{})
	c.telemetry = m
	sender := &tracedSender{}
	if err := c.Attach(sender, "test"); err != nil {
		t.Fatal(err)
	}
	parentContext, finish := m.Start(context.Background(), "test.parent")
	defer finish(nil)
	c.operationContext = parentContext
	wantTrace := trace.SpanContextFromContext(c.operationContext).TraceID()
	if err := c.MoveSpeed(12); err != nil {
		t.Fatal(err)
	}
	if err := c.AddCab(42); err != nil {
		t.Fatal(err)
	}
	c.operationContext = context.Background()
	if err := c.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	got := trace.SpanContextFromContext(sender.contexts[len(sender.contexts)-1]).TraceID()
	if !got.IsValid() || got != wantTrace {
		t.Fatal("delayed write lost speed action context")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionTelemetryConcurrentDisableAndShutdown(t *testing.T) {
	m := telemetry.New()
	s := NewObservedSession(config.Default(), nil, m)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_ = s.Post(func(c *Controller) error { return c.MoveSpeed(12) })
			}
		}()
	}
	s.Close()
	wg.Wait()
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("session failed to close")
	}
	m.Close()
	if err := s.Post(func(*Controller) error { return nil }); err == nil {
		t.Fatal("accepted after shutdown")
	}
}

func awaitObservedState(t *testing.T, s *Session, match func(State) bool) State {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case state, ok := <-s.Updates():
			if !ok {
				t.Fatal("session closed before expected state")
			}
			if match(state) {
				return state
			}
		case <-deadline.C:
			t.Fatal("session did not reach expected state")
		}
	}
}

func TestObservedSessionConnectionPaths(t *testing.T) {
	for _, serial := range []bool{false, true} {
		t.Run(map[bool]string{false: "TCP", true: "serial"}[serial], func(t *testing.T) {
			var mu sync.Mutex
			var events []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if r.URL.Path == "/v1/logs" {
					var payload logpb.ExportLogsServiceRequest
					if err := proto.Unmarshal(body, &payload); err != nil {
						t.Error(err)
					}
					mu.Lock()
					for _, resource := range payload.ResourceLogs {
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
			defer server.Close()
			m := telemetry.New()
			defer m.Close()
			settings := config.DefaultTelemetry()
			settings.Enabled, settings.Endpoint = true, server.URL
			if err := m.Configure(settings, nil); err != nil {
				t.Fatal(err)
			}
			local, remote := net.Pipe()
			defer remote.Close()
			readerDone := make(chan error, 1)
			go func() { _, err := io.Copy(io.Discard, remote); readerDone <- err }()
			options := Connection{Serial: serial, Host: "station", Port: 2560, Device: "COM4", Baud: 115200}
			s := NewObservedSession(config.Default(), func(_ context.Context, got Connection) (io.ReadWriteCloser, error) {
				if got != options {
					t.Error("connection settings changed")
				}
				return local, nil
			}, m)
			defer func() { s.Close(); <-s.Done() }()
			if err := s.Connect(options); err != nil {
				t.Fatal(err)
			}
			state := awaitObservedState(t, s, func(state State) bool { return state.Connected })
			if state.ActiveConnection != options {
				t.Fatal("active connection not published")
			}
			if _, err := io.WriteString(remote, "<l 3 0 139 0>"); err != nil {
				t.Fatal(err)
			}
			awaitObservedState(t, s, func(state State) bool { return state.Speed > 0 })
			if err := s.Connect(options); err != nil {
				t.Fatal(err)
			}
			awaitObservedState(t, s, func(state State) bool { return !state.Connected })
			s.Close()
			<-s.Done()
			if err := <-readerDone; err != nil {
				t.Fatal(err)
			}
			if err := m.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, want := range []string{"connection.attempt", "connection.opened", "connection.closed"} {
				if !strings.Contains(strings.Join(events, "\n"), want) {
					t.Fatal("missing connection event", want, events)
				}
			}
		})
	}
}

func TestObservedConnectionFailureAndCancelledOpen(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed", true: "cancelled"}[late], func(t *testing.T) {
			local, remote := net.Pipe()
			defer local.Close()
			defer remote.Close()
			entered, release := make(chan struct{}), make(chan struct{})
			m := telemetry.New()
			defer m.Close()
			s := NewObservedSession(config.Default(), func(context.Context, Connection) (io.ReadWriteCloser, error) {
				close(entered)
				<-release
				if late {
					return local, nil
				}
				return nil, errors.New("connection refused")
			}, m)
			defer func() { s.Close(); <-s.Done() }()
			if err := s.Connect(Connection{}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("open did not start")
			}
			if late {
				s.Close()
			}
			close(release)
			if !late {
				awaitObservedState(t, s, func(state State) bool {
					for _, log := range state.Logs {
						if strings.Contains(log.Text, "connection refused") {
							return true
						}
					}
					return false
				})
			}
			s.Close()
			<-s.Done()
			if late {
				if _, err := remote.Write([]byte("<p1>")); err == nil {
					t.Fatal("late connection not closed")
				}
			}
		})
	}
}
