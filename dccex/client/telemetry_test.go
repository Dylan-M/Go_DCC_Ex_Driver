package client_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	c "github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/client"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/telemetry"
	"go.opentelemetry.io/otel/attribute"
	metricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	spanpb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

type lifetimeConnection struct {
	mode   string
	closed chan struct{}
	once   sync.Once
	closes atomic.Int32
}

func (c *lifetimeConnection) Read([]byte) (int, error) {
	if c.mode == "read" {
		return 0, io.ErrUnexpectedEOF
	}
	<-c.closed
	return 0, io.ErrClosedPipe
}
func (c *lifetimeConnection) Write(data []byte) (int, error) {
	if c.mode == "write" {
		return 0, io.ErrShortWrite
	}
	if c.mode == "timeout" {
		<-c.closed
		return 0, io.ErrClosedPipe
	}
	return len(data), nil
}
func (c *lifetimeConnection) Close() error {
	c.closes.Add(1)
	c.once.Do(func() { close(c.closed) })
	if c.mode == "close" {
		return errors.New("close failed")
	}
	return nil
}

func TestConnectionLifetimeRecordsTerminalCause(t *testing.T) {
	for _, mode := range []string{"read", "write", "timeout", "close", "normal"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var spans []*spanpb.Span
			var operationReports []*metricpb.ExportMetricsServiceRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				defer mu.Unlock()
				if r.URL.Path == "/v1/traces" {
					var report tracepb.ExportTraceServiceRequest
					if err := proto.Unmarshal(body, &report); err != nil {
						t.Error(err)
					}
					for _, resource := range report.ResourceSpans {
						for _, scope := range resource.ScopeSpans {
							spans = append(spans, scope.Spans...)
						}
					}
				}
				if r.URL.Path == "/v1/metrics" {
					report := new(metricpb.ExportMetricsServiceRequest)
					if err := proto.Unmarshal(body, report); err != nil {
						t.Error(err)
					}
					operationReports = append(operationReports, report)
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
			conn := &lifetimeConnection{mode: mode, closed: make(chan struct{})}
			client := c.NewObserved(conn, m, nil)
			if mode == "read" {
				if got := next(t, client.Events()); !errors.Is(got.Err, io.ErrUnexpectedEOF) {
					t.Fatal(got.Err)
				}
			} else if mode == "write" || mode == "timeout" {
				if err := client.Send("<s>"); err == nil {
					t.Fatal("failed transport accepted a write")
				}
			}
			err := client.Close()
			if (err != nil) != (mode == "close") {
				t.Fatal("wrong close error", err)
			}
			for range client.Events() {
			}
			if err := client.Close(); err != nil {
				t.Fatal(err)
			}
			if conn.closes.Load() != 1 {
				t.Fatal("transport closed more than once")
			}
			if err := m.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			count := 0
			for _, span := range spans {
				if span.Name != "connection.lifetime" {
					continue
				}
				count++
				if got := span.Status.GetCode() == spanpb.Status_STATUS_CODE_ERROR; got != (mode != "normal") {
					t.Fatalf("lifetime error=%v for %s", got, mode)
				}
			}
			if count != 1 {
				t.Fatal("lifetime not exported exactly once", count)
			}
			wantOutcome := "error"
			if mode == "normal" {
				wantOutcome = "success"
			}
			found := false
			for _, report := range operationReports {
				for _, resource := range report.ResourceMetrics {
					for _, scope := range resource.ScopeMetrics {
						for _, metric := range scope.Metrics {
							if metric.Name != "dccex.operations" {
								continue
							}
							for _, point := range metric.GetSum().DataPoints {
								attrs := map[string]string{}
								for _, attr := range point.Attributes {
									attrs[attr.Key] = attr.Value.GetStringValue()
								}
								if attrs["operation"] == "connection.lifetime" && attrs["outcome"] == wantOutcome && point.GetAsInt() == 1 {
									found = true
								}
							}
						}
					}
				}
			}
			if !found {
				t.Fatal("missing lifetime outcome metric")
			}
		})
	}
}

func TestObservedWriteTimeout(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	m := telemetry.New()
	defer m.Close()
	client := c.NewObserved(local, m, nil)
	defer client.Close()
	// The real three-second write deadline must close a non-reading peer.
	if err := client.Send("<s>"); err == nil || err.Error() != "command write timed out" {
		t.Fatalf("unexpected timeout result: %v", err)
	}
	if err := client.Send("<s>"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("timed-out connection reused: %v", err)
	}
	select {
	case _, ok := <-client.Events():
		if ok {
			t.Fatal("local shutdown reported a remote failure")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout did not stop the reader")
	}
}

func TestObservedClientPreservesWireAndCountsTraffic(t *testing.T) {
	var mu sync.Mutex
	payloads := map[string][][]byte{}
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		payloads[r.URL.Path] = append(payloads[r.URL.Path], data)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer collector.Close()
	m := telemetry.New()
	defer m.Close()
	settings := config.DefaultTelemetry()
	settings.Enabled, settings.Debug, settings.Endpoint = true, true, collector.URL
	if err := m.Configure(settings, nil); err != nil {
		t.Fatal(err)
	}
	local, remote := net.Pipe()
	client := c.NewObserved(local, m, []attribute.KeyValue{attribute.String("server.address", "test-station")})
	defer client.Close()
	defer remote.Close()
	const command = "<t 42 10 1>"
	const incoming = "<p1><v 29 bad><r 300><unfinished"
	wire := make(chan string, 1)
	go func() {
		buf := make([]byte, len(command)+1)
		_, err := io.ReadFull(remote, buf)
		if err != nil {
			wire <- err.Error()
			return
		}
		wire <- string(buf)
		_, _ = io.WriteString(remote, incoming)
		_ = remote.Close()
	}()
	ctx, finish := m.Start(nil, "test.intent")
	if err := client.SendContext(ctx, command); err != nil {
		t.Fatal(err)
	}
	finish(nil)
	if got := <-wire; got != command+"\n" {
		t.Fatal("telemetry changed wire protocol", got)
	}
	for range 5 {
		next(t, client.Events())
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	counts := map[string]int64{}
	for _, body := range payloads["/v1/metrics"] {
		var report metricpb.ExportMetricsServiceRequest
		if err := proto.Unmarshal(body, &report); err != nil {
			t.Fatal(err)
		}
		for _, resource := range report.ResourceMetrics {
			for _, scope := range resource.ScopeMetrics {
				for _, metric := range scope.Metrics {
					if metric.Name != "dccex.traffic.bytes" {
						continue
					}
					for _, point := range metric.GetSum().DataPoints {
						for _, attr := range point.Attributes {
							if attr.Key == "direction" {
								counts[attr.Value.GetStringValue()] = point.GetAsInt()
							}
						}
					}
				}
			}
		}
	}
	if counts["send"] != int64(len(command)+1) || counts["receive"] != int64(len(incoming)) {
		t.Fatal("wrong wire byte counts", counts)
	}
	parentID, writeParent := "", ""
	for _, body := range payloads["/v1/traces"] {
		var report tracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &report); err != nil {
			t.Fatal(err)
		}
		for _, resource := range report.ResourceSpans {
			for _, scope := range resource.ScopeSpans {
				for _, span := range scope.Spans {
					if span.Name == "test.intent" {
						parentID = string(span.SpanId)
					}
					if span.Name == "transport.write" {
						writeParent = string(span.ParentSpanId)
					}
					if span.Name == "protocol.decode" && len(span.ParentSpanId) != 0 {
						t.Fatal("incoming frame falsely linked to an outgoing command")
					}
				}
			}
		}
	}
	if parentID == "" || parentID != writeParent {
		t.Fatal("write lost intent trace")
	}
	for _, body := range payloads["/v1/logs"] {
		if strings.Contains(string(body), "traceparent") {
			t.Fatal("unexpected protocol propagation")
		}
	}
}
