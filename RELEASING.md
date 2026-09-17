# Desktop releases

The **Desktop releases** workflow builds native x64 and ARM64 packages for
Windows, macOS and Linux. Android is excluded until mobile packaging and device
testing are ready.

## Publish a version

Push a version tag on the commit you intend to release, for example:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Use `vMAJOR.MINOR.PATCH` for a normal release or a prerelease such as
`v0.1.0-rc.1`. Prereleases are not made the latest release. Leading zeros in
numeric version components, missing patch versions, and build-metadata suffixes
(`+...`) are not accepted. Invalid `v*` tags fail before native builds begin.

The workflow validates the tag, runs race-enabled unit/headless tests and static
checks, then tests and builds on six native OS/architecture runners with CGO.
It checks each executable's actual architecture and packages it with the README.
All six packages must succeed before checksums and assets are uploaded to a
draft release and then published. Only the final job has repository write access.

Normal branch pushes do not publish releases. Adding the workflow does not create
a tag or release. The existing firmware integration workflow also runs on tag
pushes; review its results alongside the release build.

## Rehearse without publishing

Use **Actions → Desktop releases → Run workflow** with a version such as
`v0.0.0-ci.1`. This runs the same build matrix and retains downloadable workflow
artifacts for seven days, but does not create a tag or GitHub release. Relevant
pull requests also run build-only checks. These runs consume GitHub Actions
minutes, including macOS and Windows runner time.

## Packages and toolchains

| Target | Runner | Artifact suffix |
| --- | --- | --- |
| Windows x64 | `windows-2022` | `windows-amd64.zip` |
| Windows ARM64 | `windows-11-arm` | `windows-arm64.zip` |
| macOS Intel | `macos-15-intel` | `macos-amd64.zip` |
| macOS Apple Silicon | `macos-15` | `macos-arm64.zip` |
| Linux x64 | `ubuntu-24.04` | `linux-amd64.tar.gz` |
| Linux ARM64 | `ubuntu-24.04-arm` | `linux-arm64.tar.gz` |

Archive names start with `Go_DCC_Ex_Driver-<tag>-`.
Windows uses checksum-pinned LLVM-MinGW 20260908, native to each architecture,
and statically links the compiler runtime. macOS packages contain a `.app`
bundle; the executable is ad-hoc signed, not Developer ID signed or notarized.
Linux builds use Ubuntu 24.04 system libraries, not a portable AppImage or a
fully static binary. None is an installer, and no custom app icon is bundled.
Windows Authenticode signing and macOS notarization need a future certificate
and secrets setup.

`internal/release` uses only Go's standard library. Tests cover version validation,
archive layout/permissions, executable headers and the checksum manifest.
App-bundle versions use the numeric portion of the tag; archive and release names
retain prerelease suffixes.

## Failures and reruns

If any build fails, no release is published. Uploads are staged in a draft so a
partial upload is not presented as complete. Reruns can finish that draft but
refuse to overwrite a published release. Do not move a published version tag;
use a new version for corrections.

Official references: [GitHub runner availability](https://docs.github.com/en/actions/reference/runners/github-hosted-runners),
[LLVM-MinGW](https://github.com/mstorsjo/llvm-mingw), and
[Fyne desktop packaging](https://docs.fyne.io/started/packaging/).
