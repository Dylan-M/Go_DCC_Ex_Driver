#!/usr/bin/env bash
# Build debug-signed test APKs, never publish or use release credentials.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/android-versions.env"
: "${ANDROID_HOME:?Set ANDROID_HOME to the SDK directory}"
: "${ANDROID_TOOLS_BIN:?Set ANDROID_TOOLS_BIN to the pinned Fyne tool directory}"
arch=${1:-arm64}
case "$arch" in arm64|amd64) ;; *) echo 'Expected arm64 or amd64' >&2; exit 1 ;; esac
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/$ANDROID_NDK_VERSION"
export PATH="$ANDROID_TOOLS_BIN:$PATH"
fyne="$ANDROID_TOOLS_BIN/fyne"
[[ ! -f "$fyne.exe" ]] || fyne+=.exe
go version -m "$fyne" | grep -F "fyne.io/tools"$'\t'"$FYNE_TOOLS_VERSION"
mkdir -p "$root/dist/android"
cd "$root"
go run ./internal/androidicon > dist/android/icon.png
cd cmd/dccex-driver
"$fyne" package --target "android/$arch" --app-id com.github.dylanm.go_dccex_driver \
  --name dccex --app-version 0.0.1 --app-build 1 --icon "$root/dist/android/icon.png"
mv dccex.apk "$root/dist/android/dccex-$arch.apk"
signer="$ANDROID_HOME/build-tools/$ANDROID_BUILD_TOOLS/apksigner"
[[ ! -f "$signer.bat" ]] || signer+=.bat
"$signer" verify "$root/dist/android/dccex-$arch.apk"
echo "Built dist/android/dccex-$arch.apk"
