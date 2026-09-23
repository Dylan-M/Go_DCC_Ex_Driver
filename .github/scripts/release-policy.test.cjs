const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync, mkdtempSync, mkdirSync, writeFileSync, copyFileSync, renameSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { spawnSync } = require('node:child_process');
const { join } = require('node:path');
const { requiresReleaseBuild, checkReleaseResults, reportReleaseResults } = require('./release-policy.cjs');

const platformJobs = ['validate', 'windows_amd64', 'windows_arm64', 'macos_amd64',
  'macos_arm64', 'linux_amd64', 'linux_arm64', 'android'];

test('application changes and unknown paths select builds; only known docs skip', () => {
  for (const path of [
    '.github/workflows/release.yml', '.github/scripts/release-policy.cjs',
    '.github/scripts/release-policy.test.cjs', 'internal/release/main.go',
    'internal/release/nested/test.go', 'scripts/build-android.sh',
    'scripts/android-versions.env', 'scripts/setup-android.sh',
    'cmd/dccex-driver/AndroidManifest.xml.in', 'internal/androidicon/main.go',
    'go.mod', 'go.sum', 'go.work', 'go.work.sum', 'config/throttles.go',
    'dccex/protocol/command.go', 'throttle/controller.go', 'stations/store.go',
    'startup/options.go', 'ui/fyne/view.go', 'cmd/dccex-driver/main.go',
    'ui/fyne/view_test.go', 'assets/icon.png', 'new-component/settings.json',
    '.github/workflows/application.yml', '.github/workflows/firmware.yml',
    '.github/workflows/android.yml', '.github/workflows/desktop-package.yml',
    'scripts/nested/build-android.sh', 'internal/release-notes.md',
  ]) assert.equal(requiresReleaseBuild([path]), true, path);
  for (const path of [
    'README.md', 'ANDROID.md', 'RELEASING.md', '.github/CI.md',
    'stations/README.md', 'integration/emulator/README.md',
  ]) assert.equal(requiresReleaseBuild([path]), false, path);
  assert.equal(requiresReleaseBuild([]), false);
  assert.equal(requiresReleaseBuild(['README.md', 'internal/release/deleted.go']), true);
  // Git diff supplies the entire list, not the API's first page or 300-file cap.
  assert.equal(requiresReleaseBuild([...Array(350).fill('README.md'), 'scripts/build-android.sh']), true);
});

function results(required = 'true', result = 'success') {
  return { changes: { result: 'success', outputs: { required } },
    ...Object.fromEntries(platformJobs.map(job => [job, { result }])) };
}

test('selected jobs must all succeed, including each desktop target', () => {
  assert.doesNotThrow(() => checkReleaseResults(results()));
  for (const job of ['changes', ...platformJobs]) {
    for (const state of ['failure', 'cancelled', 'skipped', '', undefined, 'neutral']) {
      const needs = results();
      needs[job].result = state;
      assert.throws(() => checkReleaseResults(needs), /did not succeed|check failed/);
    }
    const needs = results();
    delete needs[job];
    assert.throws(() => checkReleaseResults(needs), /did not succeed|Missing job/);
  }
});

test('unselected builds may skip, but any job that runs must pass', () => {
  assert.doesNotThrow(() => checkReleaseResults(results('false', 'skipped')));
  assert.doesNotThrow(() => checkReleaseResults(results('false')));
  for (const job of platformJobs) {
    for (const state of ['failure', 'cancelled', '', undefined]) {
      const needs = results('false', 'skipped');
      needs[job].result = state;
      assert.throws(() => checkReleaseResults(needs), /check failed/);
    }
  }
  for (const state of ['failure', 'cancelled', 'skipped']) {
    const needs = results('false', 'skipped');
    needs.additional = { result: state };
    assert.throws(() => checkReleaseResults(needs), /additional/);
  }
  const needs = results();
  needs.additional = { result: 'success' };
  assert.doesNotThrow(() => checkReleaseResults(needs));
});

test('missing or malformed build selection fails closed', () => {
  assert.throws(() => checkReleaseResults(undefined), /selection did not succeed/);
  for (const required of ['', undefined, true, false, 'yes']) {
    const needs = results();
    needs.changes.outputs.required = required;
    assert.throws(() => checkReleaseResults(needs), /invalid release build selection/);
  }
  const needs = results();
  delete needs.changes.outputs;
  assert.throws(() => checkReleaseResults(needs), /invalid release build selection/);
});

test('summary distinguishes passed builds from intentionally unselected builds', () => {
  const passed = reportReleaseResults(results());
  assert.match(passed, /All selected platform builds passed/);
  assert.match(passed, /Windows x64 package: success/);
  assert.match(passed, /Android ARM64 release package \(debug-signed\): success/);
  const skipped = reportReleaseResults(results('false', 'skipped'));
  assert.match(skipped, /not required: known documentation-only changes or an empty diff/);
  assert.match(skipped, /Windows x64 package: skipped/);
  assert.doesNotMatch(skipped, /All selected platform builds passed/);
  assert.throws(() => reportReleaseResults(results('true', 'skipped')), /check failed/);
  assert.throws(() => reportReleaseResults(results('false', 'failure')), /check failed/);
});

test('six static desktop names share the same build and are all required for publishing', () => {
  const workflow = readFileSync(join(__dirname, '../workflows/release.yml'), 'utf8');
  const publish = workflow.split('\n  publish:\n')[1];
  const expected = {
    windows_amd64: ['Windows x64', 'windows-2022', 'windows-amd64', 'windows', 'amd64'],
    windows_arm64: ['Windows ARM64', 'windows-11-arm', 'windows-arm64', 'windows', 'arm64'],
    macos_amd64: ['macOS Intel', 'macos-15-intel', 'macos-amd64', 'darwin', 'amd64'],
    macos_arm64: ['macOS Apple Silicon', 'macos-15', 'macos-arm64', 'darwin', 'arm64'],
    linux_amd64: ['Linux x64', 'ubuntu-24.04', 'linux-amd64', 'linux', 'amd64'],
    linux_arm64: ['Linux ARM64', 'ubuntu-24.04-arm', 'linux-arm64', 'linux', 'arm64'],
  };
  for (const [id, [name, runner, target, goos, goarch]] of Object.entries(expected)) {
    const job = workflow.split(`\n  ${id}:\n`)[1].split(/\n  [a-z_0-9]+:\n/)[0];
    for (const [field, value] of Object.entries({ name: `${name} package`, runner, target, goos, goarch })) {
      assert.ok(job.includes(`${field}: ${value}\n`), `${id}: ${field}`);
    }
    assert.match(job, /needs: validate/);
    assert.match(job, /uses: \.\/\.github\/workflows\/desktop-package.yml/);
    assert.ok(publish.includes(`needs.${id}.result == 'success'`), `${id} must block publication`);
  }
  assert.doesNotMatch(workflow, /matrix\./);
  const reusable = readFileSync(join(__dirname, '../workflows/desktop-package.yml'), 'utf8');
  assert.match(reusable, /workflow_call:/);
  assert.doesNotMatch(reusable, /pull_request:|push:/);
  assert.match(reusable, /go test -tags ci -timeout=3m \.\/\.\.\./);
  assert.match(reusable, /go build -trimpath -tags release/);
  assert.match(reusable, /hashFiles\('\.github\/workflows\/desktop-package.yml'\)/);
});

test('application, command-station, and Android checks describe separate responsibilities', () => {
  const read = name => readFileSync(join(__dirname, '../workflows', name), 'utf8');
  const application = read('application.yml');
  const station = read('firmware.yml');
  const android = read('android.yml');
  assert.match(application, /name: Application and UI tests \(\$\{\{ github.event_name \}\}\)/);
  assert.match(application, /go test -race -tags ci -count=1 -timeout=3m \.\/\.\.\./);
  assert.match(application, /go vet -tags ci,firmware \.\/\.\.\./);
  assert.match(application, /TestHeadlessApplicationStartupUsesTabDatabase/);
  assert.match(application, /GOOS=.*GOARCH=.*CGO_ENABLED=0 go build \.\/stations \.\/startup/);
  assert.match(station, /name: Command-station integration tests \(\$\{\{ github.event_name \}\}\)/);
  assert.match(station, /go test -race -tags firmware -count=1 -timeout=3m \.\/integration/);
  assert.match(station, /node integration\/emulator\/build.cjs/);
  assert.match(station, /run: npm test/);
  assert.doesNotMatch(station, /go test.*\.\/ui\/fyne/);
  assert.match(android, /name: Android phone and emulator builds \(\$\{\{ github.event_name \}\}\)/);
  assert.match(android, /build-android.sh arm64/);
  assert.match(android, /build-android.sh amd64/);
  for (const workflow of [application, station, android]) {
    assert.match(workflow, /  pull_request:/);
    assert.doesNotMatch(workflow, /  push:/);
    assert.match(workflow, /cache-dependency-path: go.sum/);
  }
});

test('workflow gate waits for every validation job and never publishes on PRs', () => {
  const workflow = readFileSync(join(__dirname, '../workflows/release.yml'), 'utf8');
  const jobs = workflow.split('\njobs:\n')[1];
  const jobNames = [...jobs.matchAll(/^  ([a-z_0-9]+):$/gm)].map(match => match[1]);
  const gate = jobs.split('\n  release_checks:\n')[1].split('\n  publish:\n')[0];
  const dependencies = gate.match(/needs: \[([^\]]+)\]/)[1].split(',').map(value => value.trim());
  assert.deepEqual(dependencies.sort(), jobNames.filter(name => !['publish', 'release_checks'].includes(name)).sort());
  assert.match(gate, /if: always\(\) && github.event_name == 'pull_request'/);
  assert.match(gate, /reportReleaseResults\(JSON\.parse\(process\.env\.CI_NEEDS\)\)/);
  assert.match(gate, /appendFileSync\(process\.env\.GITHUB_STEP_SUMMARY/);
  assert.match(jobs.split('\n  publish:\n')[1], /github\.event_name == 'push'/);
  const trigger = workflow.split('\njobs:\n')[0];
  assert.match(trigger, /  pull_request:\s*\n  workflow_dispatch:/);
  assert.doesNotMatch(trigger, /paths:/);
  assert.match(jobs, /git diff --no-renames --name-only -z/);
});

test('actual workflow commands select Git changes and enforce job outcomes', t => {
  const workflow = readFileSync(join(__dirname, '../workflows/release.yml'), 'utf8');
  const selection = workflow.split('- name: Apply release path filters')[1]
    .split('\n  validate:')[0].split('        run: |\n')[1]
    .split('\n').map(line => line.slice(10)).join('\n');
  const gate = workflow.split('- name: Require every selected build to succeed')[1]
    .match(/        run: (.+)/)[1];
  const dir = mkdtempSync(join(tmpdir(), 'dccex-ci-policy-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const env = { ...process.env, GIT_AUTHOR_NAME: 'CI Policy Test',
    GIT_AUTHOR_EMAIL: 'ci@example.invalid', GIT_COMMITTER_NAME: 'CI Policy Test',
    GIT_COMMITTER_EMAIL: 'ci@example.invalid',
    GITHUB_OUTPUT: join(dir, 'output').replaceAll('\\', '/'),
    GITHUB_STEP_SUMMARY: join(dir, 'summary').replaceAll('\\', '/'),
    RUNNER_TEMP: dir.replaceAll('\\', '/') };
  const execute = (exe, args, overrides = {}) => spawnSync(exe, args,
    { cwd: dir, env: { ...env, ...overrides }, encoding: 'utf8' });
  const git = (...args) => {
    const result = execute('git', args);
    assert.equal(result.status, 0, String(result.error || result.stderr));
    return result.stdout.trim();
  };
  mkdirSync(join(dir, '.github/scripts'), { recursive: true });
  mkdirSync(join(dir, 'scripts'));
  copyFileSync(join(__dirname, 'release-policy.cjs'), join(dir, '.github/scripts/release-policy.cjs'));
  writeFileSync(join(dir, 'README.md'), 'Initial documentation\n');
  writeFileSync(join(dir, 'scripts/build-android.sh'), '# fixture\n');
  git('init', '--quiet');
  git('add', '.');
  git('commit', '--quiet', '-m', 'Fixture baseline');
  const base = git('rev-parse', 'HEAD');
  writeFileSync(join(dir, 'README.md'), 'Documentation only\n');
  git('commit', '--quiet', '-am', 'Documentation change');
  const docs = git('rev-parse', 'HEAD');
  renameSync(join(dir, 'scripts/build-android.sh'), join(dir, 'ANDROID.md'));
  git('add', '-A');
  git('commit', '--quiet', '-m', 'Move file outside release paths');
  const moved = git('rev-parse', 'HEAD');
  writeFileSync(join(dir, 'go.mod'), 'module example.invalid/fixture\n');
  git('add', 'go.mod');
  git('commit', '--quiet', '-m', 'Dependency change');
  const dependency = git('rev-parse', 'HEAD');
  mkdirSync(join(dir, 'ui/fyne'), { recursive: true });
  writeFileSync(join(dir, 'ui/fyne/view.go'), 'package fyneui\n');
  git('add', 'ui');
  git('commit', '--quiet', '-m', 'Application change');
  const application = git('rev-parse', 'HEAD');
  const bash = process.env.CI_BASH || 'bash';
  for (const [event, from, head, expected] of [
    ['pull_request', base, docs, 'false'], ['pull_request', docs, moved, 'true'],
    ['pull_request', moved, dependency, 'true'], ['pull_request', dependency, application, 'true'],
    ['push', '', '', 'true'], ['workflow_dispatch', '', '', 'true'],
  ]) {
    writeFileSync(env.GITHUB_OUTPUT, '');
    const result = execute(bash, ['-c', selection], { EVENT_NAME: event, BASE_SHA: from, HEAD_SHA: head });
    assert.equal(result.status, 0, String(result.error || result.stderr));
    assert.equal(readFileSync(env.GITHUB_OUTPUT, 'utf8').trim(), `required=${expected}`);
  }
  const invalid = execute(bash, ['-c', selection],
    { EVENT_NAME: 'pull_request', BASE_SHA: 'missing-ref', HEAD_SHA: docs });
  assert.notEqual(invalid.status, 0, 'Failed Git diff must not become a successful skip');
  for (const required of ['true', 'false']) {
    for (const state of ['success', 'skipped', 'failure', 'cancelled']) {
      const result = execute(bash, ['-c', gate], { CI_NEEDS: JSON.stringify(results(required, state)) });
      assert.equal(result.error, undefined);
      const pass = state === 'success' || (required === 'false' && state === 'skipped');
      assert.equal(result.status === 0, pass, `${required}/${state}: ${result.stderr}`);
    }
  }
});
