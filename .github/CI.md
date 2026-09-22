# Required PR checks

The intended integration-branch ruleset requires these GitHub Actions checks:

- `Firmware (pull_request)`
- `Android APK (pull_request)`
- `Release builds (pull_request)`

The first two workflows run for every PR. The release workflow also starts for
every PR, but its selection job retains the existing release path filters.
Only selected changes run release validation, all six desktop targets, and the
versioned Android APK build. The selection policy and its tests also select
these builds when changed.

The final release gate always evaluates the selection and job results, even
after a dependency fails. An intentional path-filter skip is allowed; a failed,
cancelled, missing, or unexpectedly skipped selected job blocks merging. The
matrix must fully succeed. Manual checks have different names and cannot
satisfy the required PR checks. Tag pushes still publish releases; ordinary
branch pushes do not duplicate PR testing.

## Protection rollout

Workflow files do not update GitHub repository rulesets. After this workflow is
merged and available to PRs, add the Android and release gate names above to the
existing `master` ruleset, keeping the existing firmware check and its GitHub
Actions integration binding. Preserve unrelated rules and bypass settings.
Verify all three checks are required on a subsequent PR.

Do not require the new release gate before it is available to affected PRs:
GitHub leaves a required check pending when its workflow never runs. This
change targets the integration branch. Applying protection to feature branches
used as stack bases needs separate care because status-check rules also govern
direct pushes to those branches.
