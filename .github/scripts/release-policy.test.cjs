const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync, mkdtempSync, mkdirSync, writeFileSync, copyFileSync, renameSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { spawnSync } = require('node:child_process');
const { join } = require('node:path');
const { requiresReleaseBuild, checkReleaseResults } = require('./release-policy.cjs');

test('release builds retain existing path filters and cover their policy', () => {
  for (const path of [
    '.github/workflows/release.yml', '.github/scripts/release-policy.cjs',
    '.github/scripts/release-policy.test.cjs', 'internal/release/main.go',
    'internal/release/nested/test.go', 'scripts/build-android.sh',
    'scripts/android-versions.env', 'scripts/setup-android.sh',
    'cmd/dccex-driver/AndroidManifest.xml.in', 'internal/androidicon/main.go',
  ]) assert.equal(requiresReleaseBuild([path]), true, path);
  for (const path of [
    'README.md', 'ui/fyne/layout.go', 'scripts/nested/build-android.sh',
    'internal/release-notes.md', 'cmd/dccex-driver/main.go',
    '.github/workflows/firmware.yml', '.github/workflows/android.yml',
  ]) assert.equal(requiresReleaseBuild([path]), false, path);
  assert.equal(requiresReleaseBuild([]), false);
  assert.equal(requiresReleaseBuild(['README.md', 'internal/release/deleted.go']), true);
  // Git diff supplies the entire list, not the API's first page or 300-file cap.
  assert.equal(requiresReleaseBuild([...Array(350).fill('README.md'), 'scripts/build-android.sh']), true);
});

function results(required = 'true', result = 'success') {
  return { changes: { result: 'success', outputs: { required } },
    validate: { result }, build: { result }, android: { result } };
}

test('selected jobs must all succeed, including the entire desktop matrix', () => {
  assert.doesNotThrow(() => checkReleaseResults(results()));
  for (const job of ['changes', 'validate', 'build', 'android']) {
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
  for (const job of ['validate', 'build', 'android']) {
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

test('workflow gate waits for every validation job and never publishes on PRs', () => {
  const workflow = readFileSync(join(__dirname, '../workflows/release.yml'), 'utf8');
  const jobs = workflow.split('\njobs:\n')[1];
  const jobNames = [...jobs.matchAll(/^  ([a-z_]+):$/gm)].map(match => match[1]);
  const gate = jobs.split('\n  release_checks:\n')[1].split('\n  publish:\n')[0];
  const dependencies = gate.match(/needs: \[([^\]]+)\]/)[1].split(',').map(value => value.trim());
  assert.deepEqual(dependencies.sort(), jobNames.filter(name => !['publish', 'release_checks'].includes(name)).sort());
  assert.match(gate, /if: always\(\) && github.event_name == 'pull_request'/);
  assert.match(gate, /checkReleaseResults\(JSON\.parse\(process\.env\.CI_NEEDS\)\)/);
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
  renameSync(join(dir, 'scripts/build-android.sh'), join(dir, 'notes.txt'));
  git('add', '-A');
  git('commit', '--quiet', '-m', 'Move file outside release paths');
  const moved = git('rev-parse', 'HEAD');
  const bash = process.env.CI_BASH || 'bash';
  for (const [event, head, expected] of [
    ['pull_request', docs, 'false'], ['pull_request', moved, 'true'],
    ['push', '', 'true'], ['workflow_dispatch', '', 'true'],
  ]) {
    writeFileSync(env.GITHUB_OUTPUT, '');
    const result = execute(bash, ['-c', selection], { EVENT_NAME: event, BASE_SHA: base, HEAD_SHA: head });
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
