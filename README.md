# Go_DCC_Ex_Driver

Functionally equivalent Go/Fyne port of [RB211/DCC_Ex_Driver](https://github.com/RB211/DCC_Ex_Driver).

## Status

**In progress.** This is a bounded implementation slice focusing on the native DCC-EX protocol layer, independent of the GUI. The `dccex/protocol` package provides frame extraction, message parsing, and command encoding for the DCC-EX native command protocol.

## What it does

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

- Go 1.22+ (standard library only for this slice)

## Development

```bash
# Initialize the module (already done by CI)
go mod init github.com/Dylan-M/Go_DCC_Ex_Driver

# Run tests
go test ./...

# Vet for issues
go vet ./...
```

## Protocol semantics preserved from Python implementation

- Inbound station updates never echo back as new outbound throttle commands
- Speed values manipulated while disconnected are not transmitted after reconnection
- Queued speed updates are discarded when their intent becomes obsolete (cab change, explicit stop, e-stop, inbound authoritative state, disconnect)
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
