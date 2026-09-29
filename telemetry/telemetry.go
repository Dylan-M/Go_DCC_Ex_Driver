// Package telemetry owns app-local OpenTelemetry providers. It never installs
// global providers and never changes the command-station wire protocol.
package telemetry

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	olog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const scope = "github.com/Dylan-M/Go_DCC_Ex_Driver"

type pipeline struct {
	traces                              *sdktrace.TracerProvider
	metrics                             *sdkmetric.MeterProvider
	logs                                *sdklog.LoggerProvider
	tracer                              trace.Tracer
	logger                              olog.Logger
	operations, bytes, messages, events metric.Int64Counter
	duration                            metric.Float64Histogram
	debug                               bool
	transport                           *http.Transport
}

// Manager is shared by the UI, session, transport and storage. Configure and
// Close run outside the control loop; producers never wait for HTTP export.
// A nil Manager is a no-op, keeping library users and existing tests opt-in.
type Manager struct {
	instanceID     string
	version        string
	sdk            sdkFactory
	current        atomic.Pointer[pipeline]
	mu             sync.Mutex
	closed         bool
	settings       config.TelemetrySettings
	active         atomic.Int64
	exportFailures atomic.Int64
	currentMA      atomic.Int64
	hasCurrent     atomic.Bool
}

// New uses the packaged application version when supplied by the entry point.
func New(version ...string) *Manager {
	v := "development"
	if len(version) > 0 && version[0] != "" {
		v = version[0]
	}
	return &Manager{settings: config.DefaultTelemetry(), instanceID: rand.Text(), version: v, sdk: defaultSDK()}
}

// sdkFactory keeps exporter construction and its failure cleanup testable without
// changing global OpenTelemetry providers or making collector requests in tests.
type sdkFactory struct {
	traces  func(context.Context, ...otlptracehttp.Option) (sdktrace.SpanExporter, error)
	metrics func(context.Context, ...otlpmetrichttp.Option) (sdkmetric.Exporter, error)
	logs    func(context.Context, ...otlploghttp.Option) (sdklog.Exporter, error)
	meter   func(*sdkmetric.MeterProvider) metric.Meter
}

func defaultSDK() sdkFactory {
	return sdkFactory{
		traces: func(ctx context.Context, options ...otlptracehttp.Option) (sdktrace.SpanExporter, error) {
			return otlptracehttp.New(ctx, options...)
		},
		metrics: func(ctx context.Context, options ...otlpmetrichttp.Option) (sdkmetric.Exporter, error) {
			return otlpmetrichttp.New(ctx, options...)
		},
		logs: func(ctx context.Context, options ...otlploghttp.Option) (sdklog.Exporter, error) {
			return otlploghttp.New(ctx, options...)
		},
		meter: func(p *sdkmetric.MeterProvider) metric.Meter { return p.Meter(scope) },
	}
}

func (m *Manager) Settings() config.TelemetrySettings {
	if m == nil {
		return config.DefaultTelemetry()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// Configure persists validated settings before starting any exporter. A failed
// save never sends data to the proposed destination. Setup failure rolls back
// storage and leaves the old pipeline active.
func (m *Manager) Configure(s config.TelemetrySettings, save func(config.TelemetrySettings) error) error {
	if err := s.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("telemetry is closed")
	}
	if save != nil {
		if err := save(s); err != nil {
			return err
		}
	}
	var next *pipeline
	var err error
	if s.Enabled {
		next, err = m.build(s)
		if err != nil {
			if save != nil {
				err = errors.Join(err, save(m.settings))
			}
			return err
		}
	}
	old := m.current.Swap(next)
	m.settings = s
	if old != nil {
		old.shutdown()
	}
	m.Event(context.Background(), "telemetry.configured")
	return nil
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if old := m.current.Swap(nil); old != nil {
		old.shutdown()
	}
}

// Flush is only for lifecycle/diagnostic workers, never a train-control callback.
func (m *Manager) Flush(ctx context.Context) error {
	p := m.load()
	if p == nil {
		return nil
	}
	return errors.Join(p.traces.ForceFlush(ctx), p.metrics.ForceFlush(ctx), p.logs.ForceFlush(ctx))
}

func (p *pipeline) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// A shared deadline bounds flushing. Export failure must not prevent exit.
	_ = p.traces.Shutdown(ctx)
	_ = p.metrics.Shutdown(ctx)
	_ = p.logs.Shutdown(ctx)
	if p.transport != nil {
		p.transport.CloseIdleConnections()
	}
}

type exportTransport struct {
	base     http.RoundTripper
	failures *atomic.Int64
}

func (t exportTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		t.failures.Add(1)
		return nil, errors.New("telemetry collector request failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.failures.Add(1)
	}
	return resp, nil
}

func (m *Manager) build(s config.TelemetrySettings) (*pipeline, error) {
	// Explicit URLs, headers and transport prevent OTEL_* variables from
	// overriding the user's destination or silently supplying credentials.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	httpClient := &http.Client{Transport: exportTransport{transport, &m.exportFailures}, Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	base := strings.TrimRight(s.Endpoint, "/")
	ctx := context.Background()
	t, err := m.sdk.traces(ctx, otlptracehttp.WithEndpointURL(base+"/v1/traces"), otlptracehttp.WithHeaders(map[string]string{}), otlptracehttp.WithHTTPClient(httpClient), otlptracehttp.WithTimeout(2*time.Second), otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}))
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errors.New("could not create trace exporter")
	}
	me, err := m.sdk.metrics(ctx, otlpmetrichttp.WithEndpointURL(base+"/v1/metrics"), otlpmetrichttp.WithHeaders(map[string]string{}), otlpmetrichttp.WithHTTPClient(httpClient), otlpmetrichttp.WithTimeout(2*time.Second), otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{Enabled: false}))
	if err != nil {
		_ = t.Shutdown(ctx)
		transport.CloseIdleConnections()
		return nil, errors.New("could not create metric exporter")
	}
	l, err := m.sdk.logs(ctx, otlploghttp.WithEndpointURL(base+"/v1/logs"), otlploghttp.WithHeaders(map[string]string{}), otlploghttp.WithHTTPClient(httpClient), otlploghttp.WithTimeout(2*time.Second), otlploghttp.WithRetry(otlploghttp.RetryConfig{Enabled: false}))
	if err != nil {
		_ = t.Shutdown(ctx)
		_ = me.Shutdown(ctx)
		transport.CloseIdleConnections()
		return nil, errors.New("could not create log exporter")
	}
	r := resource.NewSchemaless(attribute.String("service.name", "go-dcc-ex-driver"), attribute.String("service.instance.id", m.instanceID), attribute.String("service.version", m.version), attribute.String("os.type", runtime.GOOS), attribute.String("host.arch", runtime.GOARCH))
	p := &pipeline{debug: s.Debug, transport: transport}
	p.traces = sdktrace.NewTracerProvider(sdktrace.WithResource(r), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(s.SampleRatio))), sdktrace.WithBatcher(t, sdktrace.WithMaxQueueSize(1024), sdktrace.WithMaxExportBatchSize(128), sdktrace.WithBatchTimeout(time.Second), sdktrace.WithExportTimeout(2*time.Second)))
	p.metrics = sdkmetric.NewMeterProvider(sdkmetric.WithResource(r), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me, sdkmetric.WithInterval(time.Duration(s.IntervalSeconds)*time.Second), sdkmetric.WithTimeout(2*time.Second))))
	p.logs = sdklog.NewLoggerProvider(sdklog.WithResource(r), sdklog.WithProcessor(sdklog.NewBatchProcessor(l, sdklog.WithMaxQueueSize(1024), sdklog.WithExportMaxBatchSize(128), sdklog.WithExportInterval(time.Second), sdklog.WithExportTimeout(2*time.Second))))
	meter := m.sdk.meter(p.metrics)
	if err := p.instruments(meter); err != nil {
		p.shutdown()
		return nil, err
	}
	_, err = meter.Int64ObservableGauge("dccex.connection.active", metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error { o.Observe(m.active.Load()); return nil }))
	if err != nil {
		p.shutdown()
		return nil, err
	}
	_, err = meter.Int64ObservableCounter("dccex.telemetry.export_failures", metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error { o.Observe(m.exportFailures.Load()); return nil }))
	if err != nil {
		p.shutdown()
		return nil, err
	}
	_, err = meter.Int64ObservableGauge("dccex.runtime.goroutines", metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(int64(runtime.NumGoroutine()))
		return nil
	}))
	if err != nil {
		p.shutdown()
		return nil, err
	}
	_, err = meter.Int64ObservableGauge("dccex.runtime.heap", metric.WithUnit("By"), metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		o.Observe(int64(stats.HeapAlloc))
		return nil
	}))
	if err != nil {
		p.shutdown()
		return nil, err
	}
	_, err = meter.Int64ObservableGauge("dccex.station.current", metric.WithUnit("mA"), metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
		if m.active.Load() > 0 && m.hasCurrent.Load() {
			o.Observe(m.currentMA.Load())
		}
		return nil
	}))
	if err != nil {
		p.shutdown()
		return nil, err
	}
	return p, nil
}

func (p *pipeline) instruments(meter metric.Meter) error {
	p.tracer = p.traces.Tracer(scope)
	p.logger = p.logs.Logger(scope)
	var err error
	if p.operations, err = meter.Int64Counter("dccex.operations"); err != nil {
		return err
	}
	if p.bytes, err = meter.Int64Counter("dccex.traffic.bytes", metric.WithUnit("By")); err != nil {
		return err
	}
	if p.messages, err = meter.Int64Counter("dccex.messages"); err != nil {
		return err
	}
	if p.events, err = meter.Int64Counter("dccex.events"); err != nil {
		return err
	}
	p.duration, err = meter.Float64Histogram("dccex.operation.duration", metric.WithUnit("s"))
	return err
}

func (m *Manager) load() *pipeline {
	if m == nil {
		return nil
	}
	return m.current.Load()
}

// Start measures client-side operations, including failures. Span attributes
// may carry connection/cab details; metric dimensions stay bounded.
func (m *Manager) Start(ctx context.Context, operation string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	if ctx == nil {
		ctx = context.Background()
	}
	p := m.load()
	if p == nil {
		return ctx, func(error) {}
	}
	ctx, span := p.tracer.Start(ctx, operation, trace.WithAttributes(attrs...))
	start := time.Now()
	return ctx, func(err error) {
		outcome := "success"
		if err != nil {
			outcome = "error"
			span.SetStatus(codes.Error, "operation failed")
			span.RecordError(errors.New("operation failed"))
		}
		labels := metric.WithAttributes(attribute.String("operation", operation), attribute.String("outcome", outcome))
		p.operations.Add(ctx, 1, labels)
		p.duration.Record(ctx, time.Since(start).Seconds(), labels)
		span.End()
	}
}

func (m *Manager) Event(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if p := m.load(); p != nil {
		p.events.Add(ctx, 1, metric.WithAttributes(attribute.String("event.name", name)))
		trace.SpanFromContext(ctx).AddEvent(name, trace.WithAttributes(attrs...))
		m.log(ctx, olog.SeverityInfo, name, name, attrs...)
	}
}

func (m *Manager) Log(ctx context.Context, level olog.Severity, body string, attrs ...attribute.KeyValue) {
	m.log(ctx, level, body, "", attrs...)
}

func (m *Manager) log(ctx context.Context, level olog.Severity, body, event string, attrs ...attribute.KeyValue) {
	p := m.load()
	if p == nil || (level < olog.SeverityInfo && !p.debug) {
		return
	}
	var record olog.Record
	record.SetTimestamp(time.Now())
	record.SetSeverity(level)
	record.SetBody(attribute.StringValue(body))
	record.SetEventName(event)
	record.AddAttributes(attrs...)
	p.logger.Emit(ctx, record)
}

func (m *Manager) Active(delta int64) {
	if m != nil {
		if m.active.Add(delta) <= 0 {
			m.hasCurrent.Store(false)
		}
	}
}

// Current records existing station reports; telemetry never enables polling.
func (m *Manager) Current(value int) {
	if m != nil {
		m.currentMA.Store(int64(value))
		m.hasCurrent.Store(true)
	}
}

func (m *Manager) Bytes(ctx context.Context, direction string, n int) {
	if p := m.load(); p != nil && n > 0 {
		p.bytes.Add(ctx, int64(n), metric.WithAttributes(attribute.String("direction", direction)))
	}
}

func (m *Manager) Message(ctx context.Context, direction, kind, outcome string) {
	if p := m.load(); p != nil {
		p.messages.Add(ctx, 1, metric.WithAttributes(attribute.String("direction", direction), attribute.String("command", kind), attribute.String("outcome", outcome)))
	}
}

// Traffic belongs exclusively at debug level. Credential-bearing pass-through
// commands are redacted; identifiers and ordinary DCC traffic are retained.
func (m *Manager) Traffic(ctx context.Context, direction, frame string) {
	if p := m.load(); p == nil || !p.debug {
		return
	}
	m.Log(ctx, olog.SeverityDebug, "DCC-EX traffic", attribute.String("direction", direction), attribute.String("frame", Redact(frame)))
}

func Redact(frame string) string {
	if credentialTraffic.MatchString(frame) {
		return "[credential-bearing traffic redacted]"
	}
	return frame
}

var credentialTraffic = regexp.MustCompile(`(?i)(<\s*\+|AT\+|<\s*C\s+WIFI\b|\b(password|authorization|token|secret)\s*[:=])`)

// CommandKind bounds metric cardinality even for raw commands.
func CommandKind(frame string) string {
	fields := strings.Fields(strings.Trim(frame, "<> \r\n\t"))
	if len(fields) == 0 {
		return "unknown"
	}
	if strings.HasPrefix(fields[0], "i") {
		return "i"
	}
	if fields[0] == "p0" || fields[0] == "p1" || fields[0] == "p2" {
		return "p"
	}
	switch fields[0] {
	case "t", "f", "F", "!", "0", "1", "s", "c", "=", "R", "W", "w", "r", "b", "B", "D", "J", "T", "l", "p", "a", "v", "X", "C", "H", "h", "jR", "jT", "jA", "jC", "JR", "JT", "JA", "JC":
		return fields[0]
	default:
		return "unknown"
	}
}
