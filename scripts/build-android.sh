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

# Fyne creates and deletes Go metadata in its working directory. Build from a
# private snapshot so Android targets and desktop builds can run concurrently.
# Include local edits and new source files, not only the last committed tree.
stage_parent=$(cd "${TMPDIR:-/tmp}" && pwd -P)
stage=$(mktemp -d "$stage_parent/dccex-android.XXXXXX")
publish_temp=
cleanup() {
  cd "$root"
  [[ -z "$publish_temp" ]] || rm -f -- "$publish_temp"
  # Validate the exact temporary directory before recursive removal.
  if [[ "${stage%/*}" == "$stage_parent" && "${stage##*/}" == dccex-android.* ]]; then
    rm -rf -- "$stage"
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir "$stage/source"
git -C "$root" ls-files -z --cached --others --exclude-standard > "$stage/files"
while IFS= read -r -d '' path; do
  case "$path" in
    cmd/dccex-driver/AndroidManifest.xml|cmd/dccex-driver/fyne_metadata_init.go|cmd/dccex-driver/*.syso|cmd/dccex-driver/*.apk) continue ;;
  esac
  [[ ! -f "$root/$path" ]] || printf '%s\0' "$path"
done < "$stage/files" | tar -C "$root" --null -T - -cf - | tar -C "$stage/source" -xf -

# Resolve tool paths before leaving the source checkout. Module and compiler
# caches remain shared; only package-specific generated files are isolated.
ANDROID_HOME=$(cd "$ANDROID_HOME" && pwd -P)
ANDROID_TOOLS_BIN=$(cd "$ANDROID_TOOLS_BIN" && pwd -P)
export ANDROID_HOME ANDROID_TOOLS_BIN
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/$ANDROID_NDK_VERSION"
fyne="$ANDROID_TOOLS_BIN/${fyne##*/}"
output="$root/dist/android"
mkdir -p "$output"
cd "$stage/source"
go run ./internal/androidicon > "$stage/icon.png"
cd cmd/dccex-driver
sed -e "s/@VERSION_CODE@/$build/g" -e "s/@VERSION_NAME@/$version_name/g" \
  AndroidManifest.xml.in > AndroidManifest.xml
"$fyne" package --target "android/$arch" --app-id com.github.dylanm.go_dccex_driver \
  --name dccex --app-version "$version" --app-build "$build" --icon "$stage/icon.png"
signer="$ANDROID_HOME/build-tools/$ANDROID_BUILD_TOOLS/apksigner"
[[ ! -f "$signer.bat" ]] || signer+=.bat
"$signer" verify dccex.apk
aapt="$ANDROID_HOME/build-tools/$ANDROID_BUILD_TOOLS/aapt"
[[ ! -f "$aapt.exe" ]] || aapt+=.exe
badging=$("$aapt" dump badging dccex.apk)
printf '%s\n' "$badging" | grep -F "versionCode='$build'" | grep -F "versionName='$version_name'"
abi=arm64-v8a
[[ "$arch" != amd64 ]] || abi=x86_64
printf '%s\n' "$badging" | grep -Fx "native-code: '$abi'"
# Publish only a fully verified APK. A unique temporary file prevents readers
# from observing partial copies, even when the destination is on another disk.
publish_temp=$(mktemp "$output/.dccex-$arch.XXXXXX.apk")
cp dccex.apk "$publish_temp"
mv -f "$publish_temp" "$output/dccex-$arch.apk"
publish_temp=
echo "Built dist/android/dccex-$arch.apk"
