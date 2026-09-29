package telemetry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"go.opentelemetry.io/otel/attribute"
	olog "go.opentelemetry.io/otel/log"
	logpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

type capture struct {
	mu     sync.Mutex
	bodies map[string][][]byte
}

func collector(t *testing.T) (*httptest.Server, *capture) {
	t.Helper()
	c := &capture{bodies: make(map[string][][]byte)}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("environment credentials leaked")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		c.mu.Lock()
		c.bodies[r.URL.Path] = append(c.bodies[r.URL.Path], body)
		c.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	return s, c
}
func (c *capture) snapshot(path string) [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte(nil), c.bodies[path]...)
}
func configured(t *testing.T, endpoint string, debug bool, ratio float64) *Manager {
	t.Helper()
	m := New()
	s := config.DefaultTelemetry()
	s.Enabled, s.Endpoint, s.Debug, s.SampleRatio = true, endpoint, debug, ratio
	if err := m.Configure(s, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}
func flush(t *testing.T, m *Manager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.Flush(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAllSignalsAndCorrelation(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://unwanted.invalid")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=do-not-export")
	s, capture := collector(t)
	m := configured(t, s.URL+"/otel", true, 1)
	ctx, finish := m.Start(nil, "test.parent", attribute.Int("loco.address", 42))
	child, end := m.Start(ctx, "test.child")
	m.Event(child, "test.event", attribute.Bool("success", true))
	m.Log(child, olog.SeverityError, "Test error")
	m.Traffic(child, "send", "<t 42 8 1>")
	m.Traffic(child, "send", `<+AT+CWJAP="ssid","super-secret">`)
	m.Bytes(child, "send", 11)
	m.Bytes(child, "receive", 0)
	m.Message(child, "send", "t", "success")
	m.Active(1)
	m.Current(350)
	end(errors.New("credential must not appear"))
	finish(nil)
	flush(t, m)
	spans := map[string]string{}
	var parentID, childParent string
	for _, body := range capture.snapshot("/otel/v1/traces") {
		var payload tracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		for _, resource := range payload.ResourceSpans {
			for _, scope := range resource.ScopeSpans {
				for _, span := range scope.Spans {
					spans[span.Name] = string(span.TraceId)
					if span.Name == "test.parent" {
						parentID = string(span.SpanId)
					}
					if span.Name == "test.child" {
						childParent = string(span.ParentSpanId)
						if span.Status.GetCode() != 2 {
							t.Error("missing error status")
						}
					}
				}
			}
		}
		if strings.Contains(string(body), "credential must not appear") {
			t.Fatal("error credential leaked")
		}
	}
	if spans["test.parent"] == "" || spans["test.parent"] != spans["test.child"] || parentID != childParent {
		t.Fatal("broken trace parentage", spans)
	}
	traffic, event, correlated := false, false, false
	for _, body := range capture.snapshot("/otel/v1/logs") {
		if strings.Contains(string(body), "super-secret") {
			t.Fatal("credential leaked")
		}
		var payload logpb.ExportLogsServiceRequest
		if err := proto.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		for _, resource := range payload.ResourceLogs {
			for _, scope := range resource.ScopeLogs {
				for _, log := range scope.LogRecords {
					if log.EventName == "test.event" {
						event = true
						correlated = string(log.TraceId) == spans["test.child"]
					}
					if log.Body.GetStringValue() == "DCC-EX traffic" {
						traffic = true
						if log.SeverityNumber != 5 {
							t.Error("traffic not debug")
						}
					}
				}
			}
		}
	}
	if !traffic || !event || !correlated {
		t.Fatal("missing logs/events/correlation")
	}
	names := map[string]bool{}
	for _, body := range capture.snapshot("/otel/v1/metrics") {
		var payload metricpb.ExportMetricsServiceRequest
		if err := proto.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		for _, resource := range payload.ResourceMetrics {
			for _, scope := range resource.ScopeMetrics {
				for _, metric := range scope.Metrics {
					names[metric.Name] = true
				}
			}
		}
		if strings.Contains(string(body), "loco.address") {
			t.Fatal("unbounded span attribute leaked into metrics")
		}
	}
	for _, name := range []string{"dccex.operations", "dccex.operation.duration", "dccex.traffic.bytes", "dccex.messages", "dccex.events", "dccex.connection.active", "dccex.station.current", "dccex.runtime.heap", "dccex.runtime.goroutines", "dccex.telemetry.export_failures"} {
		if !names[name] {
			t.Error("missing metric", name)
		}
	}
}

func TestDisabledAndDebugFiltering(t *testing.T) {
	s, capture := collector(t)
	for _, m := range []*Manager{nil, New()} {
		ctx, end := m.Start(nil, "disabled")
		m.Event(ctx, "ignored")
		m.Log(ctx, olog.SeverityInfo, "ignored")
		m.Traffic(ctx, "send", "<t 1>")
		m.Bytes(ctx, "send", 4)
		m.Message(ctx, "send", "t", "success")
		m.Active(1)
		end(nil)
		if m.Settings().Enabled {
			t.Fatal("enabled by default")
		}
		flush(t, m)
		m.Close()
	}
	if len(capture.snapshot("/v1/logs")) != 0 {
		t.Fatal("disabled exported")
	}
	m := configured(t, s.URL, false, 0)
	ctx, end := m.Start(nil, "unsampled")
	m.Traffic(ctx, "send", "<t 42 7 1>")
	m.Log(ctx, olog.SeverityDebug, "hidden debug")
	m.Message(ctx, "send", "t", "success")
	end(nil)
	flush(t, m)
	if len(capture.snapshot("/v1/traces")) != 0 {
		t.Fatal("zero sampling exported traces")
	}
	for _, b := range capture.snapshot("/v1/logs") {
		if strings.Contains(string(b), "DCC-EX traffic") || strings.Contains(string(b), "hidden debug") {
			t.Fatal("debug data exported at info")
		}
	}
	if len(capture.snapshot("/v1/metrics")) == 0 {
		t.Fatal("sampling suppressed metrics")
	}
}

func TestConfigureFailuresAndConcurrentUse(t *testing.T) {
	s, _ := collector(t)
	m := configured(t, s.URL, false, 1)
	before := m.Settings()
	bad := before
	bad.Version = 99
	if m.Configure(bad, nil) == nil {
		t.Fatal("invalid settings accepted")
	}
	failure := errors.New("disk unavailable")
	if err := m.Configure(before, func(config.TelemetrySettings) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if m.Settings() != before {
		t.Fatal("failed save changed settings")
	}
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				ctx, end := m.Start(nil, "concurrent")
				m.Event(ctx, "worker")
				end(nil)
			}
		}()
	}
	if err := m.Configure(config.DefaultTelemetry(), nil); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	if m.load() != nil {
		t.Fatal("disable retained pipeline")
	}
	if err := m.Configure(config.DefaultTelemetry(), func(config.TelemetrySettings) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	m.Close()
	m.Close()
	if m.Configure(before, nil) == nil {
		t.Fatal("closed manager restarted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestExportFailureIsolation(t *testing.T) {
	var failures atomic.Int64
	for _, status := range []int{200, 503, 0} {
		transport := exportTransport{roundTripFunc(func(*http.Request) (*http.Response, error) {
			if status == 0 {
				return nil, errors.New("password=secret")
			}
			return &http.Response{StatusCode: status}, nil
		}), &failures}
		_, err := transport.RoundTrip(&http.Request{})
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Fatal("transport error leaked credentials")
		}
	}
	if failures.Load() != 2 {
		t.Fatal("incorrect failure count")
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	m := configured(t, server.URL, false, 1)
	ctx, end := m.Start(nil, "blocked_export")
	m.Event(ctx, "test")
	end(nil)
	flushDone := make(chan error, 1)
	flushContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() { flushDone <- m.Flush(flushContext) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("export did not start")
	}
	controlDone := make(chan struct{})
	go func() { _, finish := m.Start(nil, "control"); finish(nil); close(controlDone) }()
	select {
	case <-controlDone:
	case <-time.After(time.Second):
		t.Fatal("export blocked control instrumentation")
	}
	if err := <-flushDone; err == nil {
		t.Fatal("failed export reported success")
	}
}

func TestTrafficClassificationAndRedaction(t *testing.T) {
	for _, frame := range []string{"<t 42 100 1>", "<F 42 0 1>", "<W 29 6>"} {
		if Redact(frame) != frame {
			t.Fatal("ordinary traffic redacted")
		}
	}
	for _, frame := range []string{"<+ wifi data>", "< + wifi data>", `<C WIFI "wifi" "hunter2">`, "<C\tWIFI network hunter2>", "AT+CWJAP=password", "Authorization: secret", "token=123", "PASSWORD=123", "secret=123"} {
		if Redact(frame) == frame {
			t.Fatal("credential not redacted")
		}
	}
	for _, frame := range []string{"", "<arbitrary-name>", "<not-a-command>"} {
		if CommandKind(frame) != "unknown" {
			t.Fatal("unbounded command label")
		}
	}
	if CommandKind("<t 42 100 1>") != "t" {
		t.Fatal("wrong command")
	}
}
