# PR checks and build selection

The integration branch requires four checks after the rename rollout below,
all bound to GitHub Actions:

- `Application and UI tests (pull_request)` tests application logic, protocol,
  preferences, storage, startup, and Fyne UI interactions. It includes race
  detection, static checks, repeated startup/cleanup tests, and storage
  cross-compilation. These are headless UI tests, not Android runtime tests.
- `Command-station integration tests (pull_request)` builds pinned DCC-EX
  firmware and executes it in AVR8js. Our Go connection/session code exchanges
  real protocol messages with that firmware. It also tests the simulator's UART
  and firmware loader. It does not emulate physical track loads or decoders.
- `Android phone and emulator builds (pull_request)` builds and verifies debug
  APKs for ARM64 phones and the x86-64 Android emulator. It does not launch an
  Android emulator or perform UI interactions inside Android.
- `Platform build results (pull_request)` requires every selected release
  validation and platform-package job to succeed. Its summary lists each result
  and explicitly distinguishes passing builds from builds not required.

The first three run for every PR. Platform builds run unless every changed
path is one of the known documentation files listed in
`scripts/release-policy.cjs` in this directory. An empty diff also selects no
platform builds. New paths select builds by default: application source, tests,
dependencies, assets, manifests, scripts, and workflows cannot silently bypass
the builds. Changes to packaged documentation alone do not trigger a package
rehearsal; every tag and manual release rehearsal still builds packages.

Six separately named desktop jobs call `workflows/desktop-package.yml`. Each
tests and packages one native OS/architecture. Static caller names remain
readable even when a job is skipped. The Android release-package job is a
separate rehearsal of versioned, debug-signed packaging, not the phone/emulator
build check. Only `Publish tagged GitHub release` writes releases, and it never
runs for PRs or manual rehearsals. Alpha tags require Android; non-alpha tags
intentionally omit Android until production signing is implemented.

The platform gate runs even after a dependency fails. Failed, cancelled,
missing, or unexpectedly skipped selected jobs block merging. When builds are
not required, any job that does run must still pass. Manual checks have distinct
names and cannot satisfy PR protection. Ordinary branch pushes do not duplicate
PR testing. Download caches, checksum verification, and native toolchains are
shared with release builds.

## Required-check rename rollout

Workflow changes do not modify repository rulesets. When introducing these
names, preserve protection throughout the rollout:

1. Publish the workflow PR with the existing requirements unchanged. The old
   `Firmware (pull_request)`, `Android APK (pull_request)`, and
   `Release builds (pull_request)` checks will remain pending because their
   replacements have different names. This is intentional: do not bypass them.
2. Verify all four replacement checks succeed on the current PR commit, and
   inspect the platform summary to confirm builds ran for this code change.
3. With explicit approval, replace the three old requirements with the four
   names above in one ruleset update. Preserve the GitHub Actions integration
   binding, branch scope, and all unrelated rules and bypass settings. Never
   remove old checks first and leave an unprotected interval.
4. Read back the effective branch rules and current PR checks before merging.
   Other open PRs using old workflow names must incorporate the new workflows
   before they can satisfy the new requirements.

Do not merge or change rules automatically. Protection of feature branches
used as stack bases is separate: status rules can also govern direct pushes.
