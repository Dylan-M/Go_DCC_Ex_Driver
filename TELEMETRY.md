# OpenTelemetry

The application exports metrics, events, logs, and traces through the
OpenTelemetry Go SDK. Export is disabled by default. No exporters or periodic
export workers are started while disabled, and no collector is contacted.

## Configuration

Desktop: open **Settings** from the menu. Android: choose **Settings** from the
app-bar overflow menu. Android uses a settings page in the existing window.
The **Back** button returns to the throttle without disconnecting.
Android system Back also returns to the throttle. When a text field is focused,
Back first dismisses its focus and keyboard. Navigation waits for an in-progress
save to finish. Leaving without saving discards edits.

![Mobile telemetry settings with export disabled by default](docs/screenshots/mobile-telemetry.png)

This is a generated view of the mobile settings page, without Android system
bars or keyboard. No collector is contacted to generate documentation images.

- **Enable telemetry export** enables all signals.
- **Collector OTLP/HTTP base URL** is your collector address, for example
  `http://192.168.1.20:4318`. A path prefix is supported. The app appends
  `/v1/metrics`, `/v1/logs`, and `/v1/traces` and sends OTLP protobuf payloads.
- **Debug logs** enables raw command/response frames and detailed diagnostics.
- **Trace sampling (%)** accepts 0–100. It defaults to 100 when enabled.
  Sampling affects traces, not metrics or independently exported log/event records.
- **Metric export interval** accepts 5–300 seconds and defaults to 15 seconds.

Use HTTPS outside a trusted network. Certificate validation is not disabled.
Collector URLs must not contain credentials or query parameters. This version
does not provide an authentication-header editor; use a trusted local collector
or gateway when the upstream telemetry service requires authentication.
Exporter endpoints and headers are supplied by the app, not inherited from
`OTEL_EXPORTER_OTLP_*` environment variables.

Settings are stored in `stations.db`, in the application's private storage
directory. Save applies changes without restarting or reconnecting to a station.
A failed save leaves the current configuration active. Unsupported or damaged
saved telemetry settings disable export and are not overwritten automatically.

## Signals

Metrics use bounded operation names, command families, directions and outcomes.
Raw payloads, locomotive addresses, hostnames and arbitrary error strings are not
metric labels. Detailed identifiers may appear on spans and log records.

- `dccex.operations`: completed operations, by operation and success/error outcome.
- `dccex.operation.duration`: client-side duration histogram in seconds.
- `dccex.traffic.bytes`: actual bytes read/written, by direction, including framing
  and newlines. Received bytes include malformed or incomplete traffic.
- `dccex.messages`: sent commands and decoded replies, by direction, command family
  and outcome. Parse failures and truncated frames are counted separately.
- `dccex.events`: structured events, by stable event name.
- `dccex.connection.active`: current open station connections.
- `dccex.station.current`: last reported current in mA while connected and known.
  Instrumentation never enables polling; Engineer mode still does not poll.
- `dccex.runtime.goroutines` and `dccex.runtime.heap`: process runtime diagnostics.
- `dccex.telemetry.export_failures`: failed collector HTTP requests.

Events include connection attempts/open/close/failure, queue rejection, throttle
actions, station power changes, programming results, settings access and app
lifecycle. Events are named OpenTelemetry log records and also become span events
when a current span exists. This supplies the E in MELT without a separate wire
protocol or a separate event exporter.

Logs include lifecycle and operational diagnostics. Command and response frames
are debug-only; sending a command is not logged as proof of firmware execution.
Wi-Fi credential configuration, AT pass-through commands and explicit credential
fields are redacted. Unknown firmware extensions that introduce new credential
formats require corresponding redaction rules before being used with debug logs.

Traces cover queued controller actions, named throttle operations, connection
setup and lifetime, command encoding/dispatch paths, transport writes, frame
parsing, received-state processing, bbolt operations, and UI rendering. Delayed
speed writes retain their originating action's trace context. Error details are
not copied blindly into spans because transport or parser errors can contain
credentials or raw input.
Connection-lifetime outcomes retain terminal read/write failures and write
timeouts, even when closing the transport succeeds. Loco action spans and events
identify the action's target, not whichever tab happens to be selected. A tab
reassignment records both the new address and `loco.previous_address`.

## Interpretation and boundaries

Stock DCC-EX has no general per-request trace-context field. Incoming frames
therefore begin independent traces; they are not guessed to be children of the
latest outgoing command. This also handles unsolicited broadcasts and updates
caused by other connected clients.

Write duration is not command-station execution time or a round-trip measurement.
The app does not currently publish a general request/response latency metric:
matching by message shape alone can misattribute broadcasts or late replies.
Successful transmission does not prove a locomotive or decoder acted on it.

Firmware instrumentation, capability discovery, and cross-device trace-context
propagation are separate work. App instrumentation works with unmodified firmware.

## Failure behavior

Trace and log export uses bounded queues with non-blocking producers. When a
collector is slow or unavailable, telemetry may be dropped; train control does
not wait for export. Collector HTTP requests time out after two seconds and do
not retry indefinitely. Reconfiguration and shutdown flush pending data under
a shared three-second deadline. There is no telemetry spool on disk.

Saving enabled settings confirms configuration, not collector connectivity.
Metrics restart their aggregation when the telemetry pipeline is replaced.
An operation crossing reconfiguration can lose its old pipeline's final span;
reconfiguration must not hold up train control to preserve telemetry.

Tests use a local OTLP receiver to verify all three signal payloads, trace
parentage, credential redaction, debug filtering, metric sampling independence,
byte counts, persistence, and export-failure isolation. Fyne tests exercise the
desktop and mobile settings page, including focused and unfocused Android Back
dispatch, pending saves, and shutdown. Lifecycle tests verify startup warnings,
foreground/background events, and orderly completion. Real-device timing remains
a separate check.
