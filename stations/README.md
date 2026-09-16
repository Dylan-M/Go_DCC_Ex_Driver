# Saved command stations

Profiles use **bbolt 1.5.0**, an embedded Go key/value database with transactions
and a single local file. It needs no database server or additional C library.
The UI depends on the small `Repository` interface, not database internals.
[Upstream documents mobile use on Android and iOS](https://github.com/etcd-io/bbolt#mobile-use-iosandroid).

The app opens `stations.db` below `a.Storage().RootURI()`, using Fyne's existing
per-app storage directory on desktop and its sandbox directory on mobile. It
does not store profiles beside the executable or assume a desktop home folder.
An unavailable, locked, or unsupported database disables saved-station controls
and reports the error; manual connections remain available. Existing function
toggle configuration is separate and unchanged.

Each profile has a unique, case-insensitive name and either TCP host/port or
serial device/baud settings. Choose a profile to load it into the connection
form, then click **Connect**. Loading never automatically connects or changes an
active connection. **Save…** names the current settings; using an existing name
requires replacement confirmation. **Delete…** also requires confirmation and
only removes the local profile. After deletion, the form returns to the actual
active connection's settings without disconnecting it. When disconnected, the
current editable settings stay unchanged. Editing a name when saving creates a separate
profile; it does not silently rename or delete the old entry.

Writes are transactional and synced by bbolt. A versioned schema prevents an
older app from overwriting a future database format. The one-second file-lock
timeout prevents a second app instance from hanging indefinitely. Profiles are
not encrypted; no authentication credentials are stored. This is local storage,
not cross-device synchronization. Serial device names and actual mobile serial
access remain platform-specific, even though their profiles can be stored.

## Host and port arguments

```sh
dccex-driver --host localhost --port 2560
dccex-driver --host ::1 --port 2560
dccex-driver --help
```

Explicit `--host` or `--port` arguments prefill TCP fields and trigger one
connection attempt at launch; an omitted setting uses its default. Without
either argument, connection remains manual. Failures appear in the console
and leave manual Connect available, without automatic retries. Arguments do
not write or overwrite a profile.
Defaults remain `192.168.4.1:2560`. Hostnames, IP addresses and bracketed IPv6
addresses are accepted, but URLs and combined `host:port` strings are rejected.
Ports must be 1-65535. Invalid arguments exit with status 2 before opening the
GUI/database; help exits successfully. Explicitly selecting a saved station
after startup replaces the fields with that profile's settings.

## Validation and platform scope

Windows tests cover persistence across reopen, replacement protection,
deletion, invalid input, file locking, schema rejection, startup arguments and
profile loading in Fyne's headless UI. Storage/startup packages cross-compile
with CGO disabled for Windows/amd64, macOS/arm64, Linux/amd64, Android/arm64 and
iOS/arm64. CI repeats these builds.

Cross-compilation is not runtime validation: Android/iOS app packaging,
sandbox/lifecycle behavior and device testing still require the platform SDKs
and simulators/devices. No claim is made that the complete mobile throttle or
its serial transport has been validated by these storage-only builds.
