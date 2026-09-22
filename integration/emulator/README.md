# Actual firmware integration tests

These tests execute EX-CommandStation machine code using AVR8js, then connect
the production Go TCP transport, client, decoder and throttle session to its
USART0 through a loopback-only byte-stream adapter. The adapter does not parse
DCC-EX commands or fabricate their replies. JavaScript is test infrastructure,
not a runtime dependency of the Go/Fyne application.

## Run locally

Requirements: Go matching `go.mod`, Node.js 24 (CI pins 24.19.0), npm, Git,
Arduino CLI **1.5.1**, and a C compiler for Go's race detector. Internet access
is required for the initial dependency/firmware build. No board, Wokwi account,
subscription, GUI, or cloud AI service is required.

From the repository root, in Git Bash or another POSIX shell:

```sh
(cd integration/emulator && npm ci --ignore-scripts --no-audit --no-fund && npm test)
# If arduino-cli is not on PATH, export ARDUINO_CLI=/absolute/path/to/arduino-cli
node integration/emulator/build.cjs
go test -race -tags firmware -count=1 -timeout=3m ./dccex/... ./throttle ./config ./integration
go vet -tags firmware ./dccex/... ./throttle ./config ./integration
```

The build is isolated under `.work/`. It downloads Arduino AVR core 1.8.6 and
builds target `arduino:avr:mega:cpu=atmega2560`. It does not flash hardware or
modify a global Arduino configuration. A dirty or unexpected firmware checkout
fails the build rather than silently changing the fixture.

Firmware is pinned to `v5.6.1-Prod`, commit
`822a54977263b621541a70a3546d38f387ac7294` of
[EX-CommandStation](https://github.com/DCC-EX/CommandStation-EX).
Tracked upstream source remains unmodified; the checked-in `config.h` supplies
STANDARD_MOTOR_SHIELD configuration with Wi-Fi and Ethernet disabled. Build
provenance records the source commit, tool versions, configuration hash and HEX
hash. AVR8js 0.21.1 is integrity-locked in `package-lock.json`.

`usart.cjs` corrects a receive-flag defect in that AVR8js version: changing
transmit interrupt enables can clear RXC while an unread input byte is pending.
The wrapper preserves the receive flag and interrupt while RX remains enabled;
the original USART still handles byte timing and register reads. Peripheral
regression tests cover transmit interrupts, receive interrupt masking, and
delivery through the Mega receive vector. DCC-EX firmware is not patched, and
the adapter does not retry rejected commands or fabricate successful replies.

`DCCEX_LOG_DIR` can name an absolute directory for persistent JSONL UART logs;
otherwise each test uses a temporary directory and prints its transcript on
failure. `DCCEX_FIRMWARE` optionally selects another absolute HEX path for local
diagnosis, but the runner still requires the pinned startup version. Such an
override is not equivalent to the clean-build provenance check used by CI.

## Coverage and lifecycle

- Real firmware startup and status, cab selection, maximum speed, direction,
  F28 and emergency-stop replies through a client forced to read one byte at a
  time; current reporting without a simulated track load.
- The production throttle session's speed, momentary F0, direction, stop, cab
  selection and power controls, checked against received firmware messages.
- Session shutdown followed by a new connection querying the still-running
  firmware, confirming track power, locomotive speed, direction and functions
  remain unchanged for other operators.
- Detection of a deliberately broken TCP connection.

Each test starts a fresh emulator and erased EEPROM, with one active TCP client
on an ephemeral `127.0.0.1` port. Accepted input drains into the UART even after
TCP closes or resets; the adapter reports closure only after draining and a
short simulated settling period. This verifies delivery of accepted commands
in this adapter, **not** guaranteed delivery over an arbitrary failed
real-world network. Closing the app sends no power-off or stop command.
Cleanup stops the subprocess. Independent test deadlines
and a 120-second emulator lifetime prevent orphan listeners.

The `firmware` build tag is opt-in. Once enabled, missing dependencies fail
tests rather than silently skipping them. The GitHub workflow builds the
firmware afresh, runs these tests with the race detector, and retains only logs
and provenance, not firmware binaries. Normal unit tests remain independent.

## Simulation boundaries

For manual desktop testing, `node integration/emulator/server.cjs --manual`
extends the lifetime to two hours and permits a detached stdin. The first stdout
JSON line reports the loopback TCP port to enter in the app. Stop that process
when finished; the default CI mode still shuts down on stdin EOF and after
120 seconds.

This is a narrow Mega configuration: flash/SRAM, GPIO registers, timers 0/1/2,
USART0, EEPROM, ADC and an empty I2C bus, with explicit Mega interrupt vectors
and pin assignments. ADC inputs are zero and EEPROM starts erased. External
and pin-change interrupts, other timers/UARTs, decoder acknowledgements,
motor-shield electronics, accessory devices, USB and actual network hardware
are not modeled. Simulated time is not wall-clock time.

These tests establish firmware/protocol/session integration. They do not prove
DCC waveform correctness, electrical safety, programming-track behavior, Fyne
visual interactions, or Android compatibility. Hardware testing is still
required. Additional simulated inputs need their own validation.

Upstream firmware retains its GPL license; AVR8js retains its MIT license.
Downloaded source and dependencies retain their upstream notices and stay out
of version control along with generated build products.
