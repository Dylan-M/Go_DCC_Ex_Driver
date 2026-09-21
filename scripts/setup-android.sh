#!/usr/bin/env bash
# Run with Bash (Git Bash on Windows). Requires Go, Java, curl, unzip and sha256sum.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/android-versions.env"
: "${ANDROID_HOME:?Set ANDROID_HOME to the SDK directory}"
: "${ANDROID_TOOLS_BIN:?Set ANDROID_TOOLS_BIN to a dedicated Fyne tool directory}"
case "$(uname -s)" in
  Linux*) host=linux; sha=$ANDROID_COMMAND_SHA_LINUX; suffix= ;;
  MINGW*|MSYS*) host=win; sha=$ANDROID_COMMAND_SHA_WINDOWS; suffix=.exe ;;
  *) echo 'This setup script supports Linux and Git Bash on Windows.' >&2; exit 1 ;;
esac
mkdir -p "$ANDROID_HOME/downloads" "$ANDROID_HOME/cmdline-tools" "$ANDROID_TOOLS_BIN"
archive="$ANDROID_HOME/downloads/commandlinetools-$host-$ANDROID_COMMAND_TOOLS.zip"
if [[ ! -f "$archive" ]]; then
  curl --fail --location --retry 3 -o "$archive.part" \
    "https://dl.google.com/android/repository/commandlinetools-${host}-${ANDROID_COMMAND_TOOLS}_latest.zip"
  mv "$archive.part" "$archive"
fi
echo "$sha  $archive" | sha256sum --check
if [[ ! -d "$ANDROID_HOME/cmdline-tools/$ANDROID_COMMAND_TOOLS" ]]; then
  stage=$(mktemp -d "$ANDROID_HOME/cmdline-tools/unpack.XXXXXX")
  unzip -q "$archive" -d "$stage"
  mv "$stage/cmdline-tools" "$ANDROID_HOME/cmdline-tools/$ANDROID_COMMAND_TOOLS"
  rmdir "$stage"
fi
manager="$ANDROID_HOME/cmdline-tools/$ANDROID_COMMAND_TOOLS/bin/sdkmanager"
[[ "$host" != win ]] || manager+=.bat
# Ignore only yes's expected broken pipe, never sdkmanager's exit status.
set +e
yes | "$manager" --sdk_root="$ANDROID_HOME" "platform-tools" \
  "platforms;android-$ANDROID_PLATFORM" "build-tools;$ANDROID_BUILD_TOOLS" "ndk;$ANDROID_NDK_VERSION"
license_status=${PIPESTATUS[1]}
set -e
[[ "$license_status" == 0 ]] || exit "$license_status"
tr -d '\r' < "$ANDROID_HOME/ndk/$ANDROID_NDK_VERSION/source.properties" | grep -F "Pkg.Revision = $ANDROID_NDK_VERSION"
fyne="$ANDROID_TOOLS_BIN/fyne$suffix"
if [[ ! -f "$fyne" ]] || ! go version -m "$fyne" | grep -Fq "fyne.io/tools"$'\t'"$FYNE_TOOLS_VERSION"; then
  bin="$ANDROID_TOOLS_BIN"
  [[ "$host" != win ]] || bin=$(cygpath -m "$bin")
  GOBIN="$bin" go install "fyne.io/tools/cmd/fyne@$FYNE_TOOLS_VERSION"
fi
go version -m "$fyne" | grep -F "fyne.io/tools"$'\t'"$FYNE_TOOLS_VERSION"
