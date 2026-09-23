// Keep release build selection separate from its always-reporting merge gate.
const jobNames = {
  validate: 'Release version and test validation',
  windows_amd64: 'Windows x64 package', windows_arm64: 'Windows ARM64 package',
  macos_amd64: 'macOS Intel package', macos_arm64: 'macOS Apple Silicon package',
  linux_amd64: 'Linux x64 package', linux_arm64: 'Linux ARM64 package',
  android: 'Android ARM64 release package (debug-signed)',
};
const optionalJobs = Object.keys(jobNames);

// Only these known documentation files can skip platform builds. New source,
// dependencies, assets, build scripts, and unknown paths select builds by default.
const documentation = new Set(['README.md', 'ANDROID.md', 'RELEASING.md',
  '.github/CI.md', 'stations/README.md', 'integration/emulator/README.md']);

function requiresReleaseBuild(paths) {
  return paths.some(path => !documentation.has(path));
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

function reportReleaseResults(needs) {
  checkReleaseResults(needs);
  const selected = needs.changes.outputs.required === 'true';
  const heading = selected
    ? 'All selected platform builds passed.'
    : 'Platform builds were not required: known documentation-only changes or an empty diff.';
  const rows = optionalJobs.map(job => `- ${jobNames[job]}: ${needs[job].result}`);
  return `## Platform build results\n\n${heading}\n\n${rows.join('\n')}\n`;
}

module.exports = { requiresReleaseBuild, checkReleaseResults, reportReleaseResults };
