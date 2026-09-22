# Go_DCC_Ex_Driver

A native desktop throttle for DCC-EX model railway command stations, written in
Go with Fyne. Based on [RB211/DCC_Ex_Driver](https://github.com/RB211/DCC_Ex_Driver),
it connects directly over TCP or USB serial using the DCC-EX native protocol.

## Features

- Multiple locomotive throttles, each with independent speed, direction and F0–F28 controls.
- A sliding Rev/Fwd selector, individual stop controls and emergency stop for all locomotives.
- Independent Main and Prog power controls, All On/Off, and current monitoring.
- Named, saved TCP and serial connections that persist between launches.
- Programming-track address/CV reads and writes, plus a CV29 bit editor.
- Programming on Main (POM), with its own locomotive-address and CV/value inputs.
- A protocol console for diagnostics and manual commands.

## Download and run

Versioned builds are published on the [Releases page](https://github.com/Dylan-M/Go_DCC_Ex_Driver/releases).
Choose the package matching your operating system and processor:

| Platform | Architectures | Package |
| --- | --- | --- |
| Windows | x64 (`amd64`), ARM64 | ZIP containing `dccex-driver.exe` |
| macOS | Intel (`amd64`), Apple Silicon (`arm64`) | ZIP containing `Go_DCC_Ex_Driver.app` |
| Linux | x64 (`amd64`), ARM64 | tar.gz containing `dccex-driver` |
| Android alpha testing | ARM64 | Debug-signed APK for sideloading |

Extract the archive before launching. On macOS, move the app to Applications if
desired. On Linux, run `./dccex-driver`; packages are built on Ubuntu 24.04 and
require compatible system libraries, including OpenGL and X11. A working
graphics driver is required on all platforms.

Packages are not publisher-signed or notarized, so Windows and macOS may display
security warnings. Only run downloads you trust. Each release includes
`SHA256SUMS` for checking archive integrity. Alpha releases include an Android
ARM64 APK marked `debug`; it is for testing only, not publisher-authenticated.
Android APKs are also available from successful Android PR checks. Proper
release signing is planned before beta. See [Android development](ANDROID.md).

## Connect to a command station

In **Connection**, choose TCP and enter the hostname/IP and port, or choose Serial
and select the device and baud rate. Click **Connect**. The button changes to
**Disconnect** while connected.

You can also provide TCP settings at launch:

```bash
dccex-driver --host localhost --port 2560
```

Explicit `--host` or `--port` triggers one automatic connection attempt. The
omitted setting uses its default: `192.168.4.1` and port `2560`. Without either
argument, connection is manual. Failed attempts appear in the console and can
be retried with Connect. Command-line settings do not overwrite saved stations.

### Saved stations

Use **Save…** to name the current settings, the **Station** dropdown to load a
saved profile, and **Delete…** to remove one. Replacing or deleting a saved
profile requires confirmation.

Selecting a profile only updates the form; it does not interrupt an active
connection. After deletion, the form returns to the active connection's settings.
If disconnected, the editable fields remain unchanged. Profiles are stored
locally in the application's private storage directory.

### Power and current

Power controls and current draw are in **Connection**. Main is green when On and
red when Off; Prog is blue when On and orange when Off. The text in parentheses
always describes the reported state. Unknown, mixed and overload states have
explicit labels rather than an On/Off color.

All On and All Off are fixed-color action buttons. State changes are displayed
after the command station reports them.

Closing or disconnecting the app leaves track power and locomotive operation
unchanged for other operators. Use the power and stop controls explicitly when
you want to shut down the layout or stop trains; exiting is not an emergency stop.

## Run locomotives

In **Run**, use **+ Throttle** to open another locomotive address. Each tab keeps
its own speed, direction and function state. Stop a locomotive before closing
or reassigning its throttle; at least one throttle stays open. Open tabs, their
order and the selected locomotive are saved automatically between launches.
Reassigning an address keeps the tab in its original position.

Use **Setup…** on a throttle to name its tab. Names update and save as you type;
leave the name blank to display `Loco <address>`. Names support up to 80 Unicode
characters and remain with the tab when its address changes.
Setup also gives each F0–F28 button its own label, independently for every tab.
Blank labels use the function number. Labels save automatically and do not
change which decoder function a button controls.
Long button labels are shortened with an ellipsis to keep the window usable;
the full label remains available in Setup and to accessibility tools.
Uncheck **Show** beside an unused function to hide its button on that tab.
Showing it again restores its label and mode. Hidden functions still track
station state and are included in **All Functions Off**. Hiding a button does
not turn that function off; release a held momentary button before hiding it.

You can also Ctrl-click a locomotive tab title to edit its name in place,
without switching the active locomotive. Ordinary clicks select immediately,
without waiting for a possible double-click.
Press Enter or move focus away to save; Escape cancels. On Android, long-hold
the tab title instead. Setup remains available, and function buttons keep their
normal operating gestures.

For tab-switch diagnostics, set `DCCEX_DEBUG_TAB_TIMING=1` before launching and
redirect standard error to a log file. Each desktop tab click emits JSON lines
with a gesture ID, locomotive address, elapsed milliseconds, time since the
previous stage. Logging is off by default.
The stages cover first mouse-down, release, tap dispatch, tab layout, queued
controller selection, controller execution, and the first matching state update
applied to the UI. A Ctrl-click rename ends its gesture trace separately.

`state_render_complete` means that the UI update and layout calls returned, not
that the GPU or monitor presented the frame. Timing begins when Fyne delivers
mouse-down; it cannot include an earlier OS event-queue delay. The gap after
controller execution includes persistence, snapshot delivery, and UI queue time.
It does not prove that a fresh command-station reply has arrived. Diagnostic
writes add a little overhead; avoid slow output destinations when comparing runs.

Only the tab layout is restored, never speed, direction, function states or
track power. Connecting queries the station for each restored locomotive's
current state; restoring tabs does not start trains or automatically connect.
Saved connections, tab layout and per-tab function-mode preferences are stored
in one bbolt database, `stations.db`, in the application's private storage
directory. Older JSON settings files are not imported or modified.
On first launch, the default is locomotive 3. If saved layout data
is unreadable or unsupported, the console reports the problem, the default tab
opens, and that record is left untouched for the session. Save failures are also
reported in the console; a later layout change attempts another save.

Tap either side of the purple direction selector, or drag it to Rev/Fwd. The
speed slider runs from 0 to 126, the normal speed values used in 128-step mode;
stop and emergency-stop encodings account for the other values.

Function buttons support momentary and toggle behavior. In **Setup…**, check
**Toggle** for a latched function; unchecked functions act only while held.
Right-clicking a function also changes its mode. Modes are saved independently
per tab, and toggle buttons show `↕`. New tabs default to F3 toggle and all other
functions momentary. Legacy shared mode settings are not imported.
Track power and emergency stop remain shared across throttles.

## Programming

The **Programming** tab separates programming-track operations from POM.

- **Programming Track**: read/write locomotive addresses and CVs, or use the
  CV29 bit editor. Place only the intended locomotive on the programming track.
- **POM**: open the **On Main** sub-tab and enter the target locomotive address,
  CV and value. The address is independent of every Run throttle and starts
  blank. The locomotive does not need an open throttle tab. POM uses addressed
  CV writes without readback or decoder acknowledgement.

## Validation and limitations

Automated tests cover the protocol, connection handling, throttle state, saved
stations and Fyne UI interactions. Integration tests also run actual DCC-EX
firmware on an AVR emulator.

The emulator does not reproduce electrical track output or a physical decoder.
Physical-hardware testing, decoder programming verification and mobile runtime
validation remain outstanding. Test carefully before using the app on a layout.

## Build from source

Use the Go version specified in [go.mod](go.mod), a C compiler, and your platform's
[Fyne graphics development dependencies](https://docs.fyne.io/started/quick/).

```bash
go build -o dccex-driver ./cmd/dccex-driver
go test -tags ci ./...
go vet -tags ci ./...
```

On Windows, name the output `dccex-driver.exe`. The `ci` tag selects Fyne's
headless test driver; it is not used for application builds.

The code separates transport and protocol (`dccex`), controller/session logic
(`throttle`), saved stations (`stations`), startup options (`startup`) and the
native interface (`ui/fyne`).

Further documentation:

- [Saved-station storage](stations/README.md)
- [Firmware simulator and integration tests](integration/emulator/README.md)
- [Building and publishing releases](RELEASING.md)
- [Android builds and emulator testing](ANDROID.md)
- [DCC-EX protocol reference](https://dcc-ex.com/reference/software/command-summary-consolidated.html)
