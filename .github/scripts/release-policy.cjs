// Keep release build selection separate from its always-reporting merge gate.
const optionalJobs = ['validate', 'build', 'android'];

function requiresReleaseBuild(paths) {
  return paths.some(path =>
    path === '.github/workflows/release.yml' ||
    path.startsWith('.github/scripts/release-policy.') ||
    path.startsWith('internal/release/') ||
    /^scripts\/[^/]*android[^/]*$/.test(path) ||
    path === 'cmd/dccex-driver/AndroidManifest.xml.in' ||
    path.startsWith('internal/androidicon/'));
}

function checkReleaseResults(needs) {
  if (needs?.changes?.result !== 'success') {
    throw new Error('Release build selection did not succeed');
  }
  const required = needs.changes.outputs?.required;
  if (required !== 'true' && required !== 'false') {
    throw new Error('Missing or invalid release build selection');
  }
  for (const job of optionalJobs) {
    if (!needs[job]) throw new Error(`Missing job result: ${job}`);
  }
  for (const [job, value] of Object.entries(needs)) {
    if (value.result === 'success') continue;
    if (required === 'false' && optionalJobs.includes(job) && value.result === 'skipped') continue;
    throw new Error(`Release check failed: ${job} (${value.result})`);
  }
}

module.exports = { requiresReleaseBuild, checkReleaseResults };
