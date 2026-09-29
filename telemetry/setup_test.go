package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

type trackedTraceExporter struct {
	sdktrace.SpanExporter
	closed *atomic.Int64
}

func (e trackedTraceExporter) Shutdown(ctx context.Context) error {
	e.closed.Add(1)
	return e.SpanExporter.Shutdown(ctx)
}

type trackedMetricExporter struct {
	sdkmetric.Exporter
	closed *atomic.Int64
}

func (e trackedMetricExporter) Shutdown(ctx context.Context) error {
	e.closed.Add(1)
	return e.Exporter.Shutdown(ctx)
}

func TestExporterSetupFailureRollsBackAndCleansUp(t *testing.T) {
	for _, stage := range []string{"traces", "metrics", "logs"} {
		t.Run(stage, func(t *testing.T) {
			server, _ := collector(t)
			m := configured(t, server.URL, false, 1)
			old, before := m.current.Load(), m.Settings()
			next := before
			next.Debug = true
			var tracesClosed, metricsClosed atomic.Int64
			constructors := defaultSDK()
			failure := errors.New("secret=must-not-leak")
			m.sdk.traces = func(ctx context.Context, options ...otlptracehttp.Option) (sdktrace.SpanExporter, error) {
				if stage == "traces" {
					return nil, failure
				}
				e, err := constructors.traces(ctx, options...)
				return trackedTraceExporter{e, &tracesClosed}, err
			}
			m.sdk.metrics = func(ctx context.Context, options ...otlpmetrichttp.Option) (sdkmetric.Exporter, error) {
				if stage == "metrics" {
					return nil, failure
				}
				e, err := constructors.metrics(ctx, options...)
				return trackedMetricExporter{e, &metricsClosed}, err
			}
			m.sdk.logs = func(context.Context, ...otlploghttp.Option) (sdklog.Exporter, error) { return nil, failure }
			var saves []config.TelemetrySettings
			rollbackErr := errors.New("storage rollback failed")
			err := m.Configure(next, func(s config.TelemetrySettings) error {
				saves = append(saves, s)
				if len(saves) == 2 {
					return rollbackErr
				}
				return nil
			})
			if err == nil || !errors.Is(err, rollbackErr) || strings.Contains(err.Error(), "must-not-leak") {
				t.Fatalf("unexpected setup/rollback error: %v", err)
			}
			if !reflect.DeepEqual(saves, []config.TelemetrySettings{next, before}) || m.Settings() != before || m.current.Load() != old {
				t.Fatal("failed setup replaced the live pipeline or did not attempt storage rollback")
			}
			wantTraces, wantMetrics := int64(0), int64(0)
			if stage != "traces" {
				wantTraces = 1
			}
			if stage == "logs" {
				wantMetrics = 1
			}
			if tracesClosed.Load() != wantTraces || metricsClosed.Load() != wantMetrics {
				t.Fatal("partially constructed exporters were not shut down")
			}
			if err := m.Configure(next, nil); err == nil {
				t.Fatal("setup failure without persistence reported success")
			}
		})
	}
}

type failingMeter struct {
	metric.Meter
	fail string
}

func (m failingMeter) Int64Counter(name string, options ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	if name == m.fail {
		return nil, errors.New("counter construction failed")
	}
	return m.Meter.Int64Counter(name, options...)
}
func (m failingMeter) Float64Histogram(name string, options ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	if name == m.fail {
		return nil, errors.New("histogram construction failed")
	}
	return m.Meter.Float64Histogram(name, options...)
}
func (m failingMeter) Int64ObservableGauge(name string, options ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	if name == m.fail {
		return nil, errors.New("gauge construction failed")
	}
	return m.Meter.Int64ObservableGauge(name, options...)
}
func (m failingMeter) Int64ObservableCounter(name string, options ...metric.Int64ObservableCounterOption) (metric.Int64ObservableCounter, error) {
	if name == m.fail {
		return nil, errors.New("observable counter construction failed")
	}
	return m.Meter.Int64ObservableCounter(name, options...)
}

func TestInstrumentSetupFailures(t *testing.T) {
	for _, name := range []string{
		"dccex.operations", "dccex.traffic.bytes", "dccex.messages", "dccex.events", "dccex.operation.duration",
		"dccex.connection.active", "dccex.telemetry.export_failures", "dccex.runtime.goroutines", "dccex.runtime.heap", "dccex.station.current",
	} {
		t.Run(name, func(t *testing.T) {
			server, _ := collector(t)
			m := New()
			defer m.Close()
			m.sdk.meter = func(p *sdkmetric.MeterProvider) metric.Meter { return failingMeter{p.Meter(scope), name} }
			s := config.DefaultTelemetry()
			s.Enabled, s.Endpoint = true, server.URL
			if err := m.Configure(s, nil); err == nil {
				t.Fatal("instrument setup failure reported success")
			}
			if m.current.Load() != nil || m.Settings().Enabled {
				t.Fatal("partial pipeline became active")
			}
		})
	}
}

func TestResourceIdentityAndCurrentReset(t *testing.T) {
	server, captured := collector(t)
	m := New("0.0.1-alpha.3")
	defer m.Close()
	if New().version != "development" || New("").version != "development" || New().instanceID == m.instanceID {
		t.Fatal("incorrect default version or reused instance ID")
	}
	s := config.DefaultTelemetry()
	s.Enabled, s.Endpoint = true, server.URL
	for i := 0; i < 2; i++ {
		if err := m.Configure(s, nil); err != nil {
			t.Fatal(err)
		}
		_, end := m.Start(nil, "resource.test")
		end(nil)
		flush(t, m)
	}
	for _, body := range captured.snapshot("/v1/traces") {
		var payload tracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		for _, resource := range payload.ResourceSpans {
			values := map[string]string{}
			for _, attr := range resource.Resource.Attributes {
				values[attr.Key] = attr.Value.GetStringValue()
			}
			if values["service.version"] != "0.0.1-alpha.3" || values["service.instance.id"] != m.instanceID {
				t.Fatal("incorrect resource metadata", values)
			}
		}
	}
	m.Active(1)
	m.Current(500)
	m.Active(-1)
	if m.hasCurrent.Load() || m.active.Load() != 0 {
		t.Fatal("station current survived disconnect")
	}
	for frame, want := range map[string]string{"<iDCC-EX V-5>": "i", "<p0>": "p", "<p1 MAIN>": "p", "<p2 PROG>": "p"} {
		if got := CommandKind(frame); got != want {
			t.Fatalf("CommandKind(%q) = %q, want %q", frame, got, want)
		}
	}
}

func TestExportDoesNotFollowRedirect(t *testing.T) {
	var redirected atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	m := configured(t, server.URL, false, 1)
	_, end := m.Start(nil, "test.redirect")
	end(nil)
	if err := m.Flush(context.Background()); err == nil {
		t.Fatal("redirect reported as successful export")
	}
	if redirected.Load() != 0 || m.exportFailures.Load() == 0 {
		t.Fatal("redirect was followed or not counted")
	}
}
