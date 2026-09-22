#!/usr/bin/env bash
# Build debug-signed APKs for local testing and GitHub alpha releases.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/android-versions.env"
: "${ANDROID_HOME:?Set ANDROID_HOME to the SDK directory}"
: "${ANDROID_TOOLS_BIN:?Set ANDROID_TOOLS_BIN to the pinned Fyne tool directory}"
arch=${1:-arm64}
case "$arch" in arm64|amd64) ;; *) echo 'Expected arm64 or amd64' >&2; exit 1 ;; esac
tag=${2:-v0.0.1}
build=${ANDROID_VERSION_CODE:-1}
[[ "$build" =~ ^[1-9][0-9]{0,9}$ ]] && (( build <= 2100000000 )) || { echo 'Invalid Android version code' >&2; exit 1; }
cd "$root"
metadata=$(go run ./internal/release validate "$tag")
version=$(printf '%s\n' "$metadata" | sed -n 's/^version=//p')
version_name=${tag#v}
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/$ANDROID_NDK_VERSION"
export PATH="$ANDROID_TOOLS_BIN:$PATH"
fyne="$ANDROID_TOOLS_BIN/fyne"
[[ ! -f "$fyne.exe" ]] || fyne+=.exe
go version -m "$fyne" | grep -F "fyne.io/tools"$'\t'"$FYNE_TOOLS_VERSION"
mkdir -p "$root/dist/android"
cd "$root"
go run ./internal/androidicon > dist/android/icon.png
cd cmd/dccex-driver
sed -e "s/@VERSION_CODE@/$build/g" -e "s/@VERSION_NAME@/$version_name/g" \
  AndroidManifest.xml.in > AndroidManifest.xml
"$fyne" package --target "android/$arch" --app-id com.github.dylanm.go_dccex_driver \
  --name dccex --app-version "$version" --app-build "$build" --icon "$root/dist/android/icon.png"
mv dccex.apk "$root/dist/android/dccex-$arch.apk"
signer="$ANDROID_HOME/build-tools/$ANDROID_BUILD_TOOLS/apksigner"
[[ ! -f "$signer.bat" ]] || signer+=.bat
"$signer" verify "$root/dist/android/dccex-$arch.apk"
aapt="$ANDROID_HOME/build-tools/$ANDROID_BUILD_TOOLS/aapt"
[[ ! -f "$aapt.exe" ]] || aapt+=.exe
badging=$("$aapt" dump badging "$root/dist/android/dccex-$arch.apk")
printf '%s\n' "$badging" | grep -F "versionCode='$build'" | grep -F "versionName='$version_name'"
abi=arm64-v8a
[[ "$arch" != amd64 ]] || abi=x86_64
printf '%s\n' "$badging" | grep -Fx "native-code: '$abi'"
echo "Built dist/android/dccex-$arch.apk"
