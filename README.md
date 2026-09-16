# Go_DCC_Ex_Driver

Functionally equivalent Go/Fyne port of [RB211/DCC_Ex_Driver](https://github.com/RB211/DCC_Ex_Driver).

## Status

**In progress.** Native Go/Fyne throttle, TCP/serial transport, protocol and controller layers are implemented, with unit, headless UI and actual-firmware emulator tests. Saved station profiles and host/port startup arguments are available. Physical hardware and full mobile app validation remain outstanding; this is not a release-ready application.

## Connections and saved stations

Launch with `dccex-driver --host localhost --port 2560` to prefill the TCP fields,
then click **Connect**. Use the **Station** dropdown to load a saved connection,
**Save…** to save current settings, or **Delete…** to remove a profile. Profiles
use bbolt in the platform's private app storage; loading or passing arguments
does not automatically connect or overwrite saved data.

See [saved stations and platform scope](stations/README.md) and
[firmware integration tests](integration/emulator/README.md).

## What it does

### Tabbed controls

The main tabs are **Connection**, **Run**, and **Programming**. Run contains
independent locomotive tabs: use **+ Throttle** to add an address, and a tab's
close control to remove it. Stop a locomotive before closing or reassigning its
throttle; at least one throttle stays open. Open throttle tabs are session-only.

Each throttle maintains its own speed, direction, and function state. Switching
tabs does not cancel another throttle's queued speed command. Direction uses a
full-width sliding Rev/Fwd selector: tap either side or drag and release.
Track power, emergency stop, and momentary/toggle function preferences are shared.
Program on Main explicitly shows the selected Run locomotive as its target.

This port reimplements the Python-based DCC-EX throttle client in Go using Fyne as the UI toolkit. It speaks the **DCC-EX native command protocol** directly to an EX-CommandStation over TCP or USB serial — no phone apps, no JMRI, no WiThrottle bridge, no subscriptions.

### Protocol capabilities (this slice)

- **Frame extraction**: Incremental parsing of angle-bracket delimited frames from arbitrary byte streams, handling partial frames, multiple frames per read, garbage bytes, and bounded runaway buffers.
- **Message parsing**: Typed parsing for all inbound DCC-EX native messages currently handled by the Python application:
  - Locomotive state (`<l cab reg speedByte functMap>`)
  - Track power status (`<p0>`, `<p1>`, `<p1 MAIN>`, `<p1 PROG>`, `<p2>` overload)
  - Track current replies (`<c "CurrentMAIN" ...>`)
  - CV read results (`<v cv value>`)
  - Address/CV write acknowledgements (`<r cv address/value>`, `<r address>`)
  - Version/status banner (`<iDCC-EX V-...>`)
- **Command encoding**: Construction of all outbound commands used by the Python application:
  - Status/version request (`<s>`)
  - Track power control (`<1>` / `<0>`, `<1 MAIN>` / `<0 MAIN>`, `<1 PROG>` / `<0 PROG>`)
  - Locomotive state request (`<t cab>`)
  - Throttle updates (`<t cab speed dir>`)
  - Function toggles (`<F cab func state>`)
  - Emergency stop (`<!>`)
  - Current query (`<c>`)
  - Address read/write on programming track (`<R>`, `<W addr>`)
  - CV read/write on programming track (`<R cv>`, `<W cv val>`)
  - Program on Main (`<w cab cv val>`)
- **Validation**: Explicit range validation where values have documented bounds:
  - Speed: 0-126 (step value 0/1 = stop/e-stop; 2-127 maps to speed 1-126)
  - Direction: 0 (reverse), 1 (forward)
  - Functions: F0-F28 exposed (protocol supports up to F68, but FUNCTMap is 32-bit so practical ceiling is F31)
  - CV values: 0-255, addresses: 1-10293

### Architecture (this slice)

```text
dccex/protocol/    frame extraction, message parsing, command encoding
```

This protocol package is **GUI-independent** and **transport-neutral**. It does not depend on Fyne, serial, or TCP implementations. The UI layer submits user intents and renders state/events from the station; it does not own protocol truth.

## Requirements

- Go 1.27+, with dependencies pinned in `go.mod`.
- Native Fyne desktop builds require a working C compiler and the platform's graphics development dependencies. Headless tests use the `ci` build tag.

## Development

```bash
# Run unit and headless UI tests
go test -tags ci ./...

# Vet for issues
go vet -tags ci ./...
```

## Protocol semantics preserved from Python implementation

- Inbound station updates never echo back as new outbound throttle commands
- Speed values manipulated while disconnected are not transmitted after reconnection
- Queued speed updates are discarded when their intent becomes obsolete (locomotive reassignment, explicit stop, e-stop, inbound authoritative state, disconnect), not when navigating between open throttle tabs
- Displayed speed/direction/latched functions don't claim success when writes fail
- Direction/stop controls restore prior displayed state on send failure
- TCP EOF vs serial timeout handled distinctly; serial timeout is not a disconnect
- Network/serial reads don't mutate UI state directly
- Overload indication latched by `<p2>`, cleared only by later `<p0>`/`<p1>` (not by current query reply)
- CV29 bits 6-7 preserved from last read to prevent clobbering during read-modify-write

## Testing strategy

This slice includes:

1. **Table-driven unit tests** for every parsed message and generated command
2. **Edge cases**: malformed input, numeric boundaries, fragmented/concatenated frames
3. **Garbage handling**: random bytes interspersed with valid frames
4. **Oversized input**: runaway buffer testing at the 4KB limit
5. **Speed-byte decoding**: stop/e-stop representation verification
6. **Quoted current replies**: parsing by value rather than position
7. **Opcode disambiguation**: `<r>` with 1 vs 2 arguments distinguished

## Reference and compatibility

- **Upstream Python source**: https://github.com/RB211/DCC_Ex_Driver
- **Protocol reference**: https://dcc-ex.com/reference/software/command-summary-consolidated.html
- **Offline protocol copy**: Documents/dccex-native-protocol.md

The Go implementation is functionally equivalent to the Python version (v0.2.0 baseline) for all observable behavior required by the source application. This does not mean line-for-line translation: Go structure and idioms are used wherever they preserve the required external behavior.

## Notes

- This repository is an **in-progress port** under supervised implementation.
- The protocol package lives here; UI implementations (Fyne for desktop, future Android) would depend on it separately.
- See the upstream repository's translation brief in `Documents/Homelab/docs/translations/dcc-ex-driver-go.md` for architecture decisions and compatibility contract.
- No license has been agreed upon with the upstream author yet; do not claim copyright or distribute publicly until that is resolved.

---

*Porting by Dylan Myers, supervised implementation workflow.*
