# Android development

Android builds currently produce debug-signed APKs for testing, including ARM64
APKs attached to GitHub alpha releases. These are not Play Store releases or
publisher-authenticated builds. TCP is the supported connection path. Desktop serial device access
does not implement Android USB Host permissions or drivers. The Android activity
requests portrait orientation; landscape is not a supported mobile layout.

Android always uses Engineer mode: TCP connections, saved stations, all Run
controls, and a read-only console. Track power, current monitoring, programming,
and raw commands require desktop Power Throttle mode (`--power-throttle`).

## Build locally

Install the Go version in `go.mod`, Java 21 or newer, and Bash (Git Bash on
Windows). The setup script requires `curl`, `unzip`, and `sha256sum`.
Use absolute paths for both tool directories. For example, in Git Bash:

```bash
export JAVA_HOME='C:/Program Files/Java/jdk-24'
export ANDROID_HOME='C:/Users/ralgith/AppData/Local/Android/Sdk'
export ANDROID_TOOLS_BIN='C:/Users/ralgith/go/bin'
bash scripts/setup-android.sh
bash scripts/build-android.sh arm64
bash scripts/build-android.sh amd64
```

For versioned testing, pass a tag as the second build argument, for example
`ANDROID_VERSION_CODE=3 bash scripts/build-android.sh arm64 v0.0.1-alpha.3`.
The generated manifest records the complete version name, including the alpha
suffix. The version code must be between 1 and 2,100,000,000; release CI uses its
workflow run number. Build scripts verify the APK signature, version metadata,
and CPU architecture. Edit `AndroidManifest.xml.in`, not the generated XML.

On Linux, choose writable absolute paths such as `$HOME/android-sdk` and
`$HOME/android-tools/bin`. Ensure Go and Java are on PATH. Setup downloads
Google's command-line tools with a checked SHA-256, installs the pinned SDK/NDK,
and installs the pinned Fyne CLI. It accepts the licenses required by those SDK
packages. Review Google's SDK terms before running it. Versions live in
`scripts/android-versions.env`; local and CI builds use the same scripts.

Outputs are `dist/android/dccex-arm64.apk` for phones and
`dist/android/dccex-amd64.apk` for x86-64 emulators. Every build verifies its
APK signature. Reruns replace generated outputs, not application settings.
The lowercase Android application ID is `com.github.dylanm.go_dccex_driver`;
the existing desktop storage ID is unchanged. Only network access is requested;
saved stations and throttles use private app storage.

Fyne's pinned debug packager currently targets Android API 29 even though the
build uses SDK 36. Store targeting requirements, release signing, and AAB
publishing are separate work. Do not submit these debug packages to an app store.
Proper private-key signing is planned before beta. The current workflow publishes
Android APKs only for `-alpha` and `-alpha.*` tags, never beta, RC, or stable tags.
Until that signing change lands, those other tags produce desktop assets only.
Debug signatures do not authenticate the publisher; SHA256SUMS checks integrity,
not publisher identity. Switching to the future release key may require
uninstalling the test app and losing its private settings; export/backup support
is not yet provided.

## Android Emulator

Install `emulator` and `system-images;android-36;google_apis;x86_64` using the SDK
manager. Create an AVD once (omit creation on subsequent runs):

```bash
"$ANDROID_HOME/cmdline-tools/15859902/bin/avdmanager" create avd \
  --name DCCEX_API36 --package 'system-images;android-36;google_apis;x86_64' --device pixel_7
"$ANDROID_HOME/emulator/emulator" -accel-check
"$ANDROID_HOME/emulator/emulator" -avd DCCEX_API36
"$ANDROID_HOME/platform-tools/adb" -s emulator-5554 install -r dist/android/dccex-amd64.apk
"$ANDROID_HOME/platform-tools/adb" -s emulator-5554 shell am start -W \
  -n com.github.dylanm.go_dccex_driver/org.golang.app.GoNativeActivity
```

On Windows use `avdmanager.bat`. The device serial can differ; check `adb devices`.
Enable a supported hypervisor only if `-accel-check` reports one is unavailable.
The emulator and DCC-EX firmware simulator are independent processes.

Start the [firmware simulator](integration/emulator/README.md) on the host.
In the Android app select TCP, host **10.0.2.2**, and the simulator's reported
port. Android's `localhost` is the Android device, not your PC. Disconnect any
desktop test client first; the firmware simulator accepts one client at a time.
Closing the Android app must not turn off track power.

## CI and caches

`Android build` runs on pull requests or manual dispatch, never branch pushes.
It builds both architectures and retains test APKs for seven days. The separate
release workflow also builds the ARM64 APK and includes it in alpha releases and
their checksum manifest. Release publication waits for the Android build along
with all six desktop builds. This is an
actual application build, not the existing storage-only cross-compilation check.
It does not claim emulator runtime or physical-device validation.

The Android tool cache is keyed by OS, architecture, pinned versions, Go module,
and setup script. Setup runs on cache hits too, verifies the command-tool archive
and Fyne version, and installs missing SDK packages. No application APK is used
as a build cache. Go's module/build cache is managed by `setup-go`.

Existing workflows also cache npm downloads, the verified Arduino CLI archive,
AVR platform packages, pinned firmware source, Windows compiler archives, and
Linux graphics dependency downloads. Install and validation steps still run on
cache hits; missing caches are an optimization loss, not a correctness failure.
Firmware binaries are rebuilt and their provenance is recorded each run.
