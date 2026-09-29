const assert = require('node:assert/strict');
const { spawn, execFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');

const bash = process.platform === 'win32' ? 'C:/Program Files/Git/bin/bash.exe' : 'bash';

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'dccex-packaging-test-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  function write(name, content) {
    const file = path.join(root, name);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, content, { mode: 0o755 });
  }
  write('scripts/build-android.sh', fs.readFileSync(path.join(__dirname, 'build-android.sh')));
  write('scripts/android-versions.env', 'FYNE_TOOLS_VERSION=v1.7.2\nANDROID_NDK_VERSION=29\nANDROID_BUILD_TOOLS=36\n');
  write('.gitignore', '/dist/\n/tools/\n/sdk/\n/logs/\n');
  write('cmd/dccex-driver/AndroidManifest.xml.in', '@VERSION_CODE@ @VERSION_NAME@\n');
  write('cmd/dccex-driver/main.go', 'original source\n');
  write('deleted.go', 'tracked but removed before packaging\n');
  execFileSync('git', ['init', '--quiet'], { cwd: root });
  execFileSync('git', ['add', '.'], { cwd: root });
  fs.unlinkSync(path.join(root, 'deleted.go'));
  write('cmd/dccex-driver/main.go', 'modified source\n');
  write('new-source.go', 'untracked source\n');
  // Stale generated files must neither enter the snapshot nor be deleted from
  // the caller's checkout, even if they are not covered by its ignore rules.
  write('cmd/dccex-driver/fyne_metadata_init.go', 'leave original metadata alone\n');
  write('cmd/dccex-driver/AndroidManifest.xml', 'leave original manifest alone\n');
  write('tools/go', `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == version ]]; then printf 'mod\\tfyne.io/tools\\tv1.7.2\\n'; exit; fi
if [[ "$2" == ./internal/release ]]; then printf 'version=0.0.1\\n'; exit; fi
[[ "$2" == ./internal/androidicon ]]
[[ $(cat cmd/dccex-driver/main.go) == 'modified source' ]]
[[ $(cat new-source.go) == 'untracked source' ]]
[[ ! -e deleted.go && ! -e cmd/dccex-driver/fyne_metadata_init.go ]]
printf icon
`);
  write('tools/fyne', `#!/usr/bin/env bash
set -euo pipefail
arch=
while [[ $# -gt 0 ]]; do
  if [[ "$1" == --target ]]; then arch="\${2#android/}"; shift; fi
  shift
done
printf 'generated metadata' > fyne_metadata_init.go
pwd -W > "$TEST_LOG_DIR/$arch.ready" 2>/dev/null || pwd > "$TEST_LOG_DIR/$arch.ready"
if [[ \${FAIL_STAGE:-} == fyne ]]; then exit 41; fi
if [[ \${WAIT_FOR_PEER:-} == 1 ]]; then
  for ((i=0; i<1000; i++)); do
    [[ ! -f "$TEST_LOG_DIR/release" ]] || break
    sleep 0.01
  done
  [[ -f "$TEST_LOG_DIR/release" ]]
fi
printf '%s' "$arch" > dccex.apk
rm fyne_metadata_init.go
`);
  write('sdk/build-tools/36/apksigner', `#!/usr/bin/env bash
[[ \${FAIL_STAGE:-} != signer ]] || exit 42
[[ "$1" == verify && -s "$2" ]]
`);
  write('sdk/build-tools/36/aapt', `#!/usr/bin/env bash
[[ \${FAIL_STAGE:-} != badging ]] || exit 43
abi=arm64-v8a
[[ $(cat "$3") != amd64 ]] || abi=x86_64
printf "package: versionCode='1' versionName='0.0.1-alpha.3'\\n"
printf "native-code: '%s'\\n" "$abi"
`);
  fs.mkdirSync(path.join(root, 'logs'));
  return root;
}

function build(root, arch, extra = {}) {
  const child = spawn(bash, ['scripts/build-android.sh', arch, 'v0.0.1-alpha.3'], {
    cwd: root,
    env: {
      ...process.env,
      PATH: `${path.join(root, 'tools')}${path.delimiter}${process.env.PATH}`,
      ANDROID_HOME: path.join(root, 'sdk'),
      ANDROID_TOOLS_BIN: path.join(root, 'tools'),
      ANDROID_VERSION_CODE: '1',
      TEST_LOG_DIR: path.join(root, 'logs'),
      ...extra,
    },
    windowsHide: true,
    timeout: 30000,
  });
  let output = '';
  child.stdout.on('data', chunk => { output += chunk; });
  child.stderr.on('data', chunk => { output += chunk; });
  return new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('close', code => resolve({ code, output }));
  });
}

function assertCheckoutUntouched(root) {
  assert.equal(fs.readFileSync(path.join(root, 'cmd/dccex-driver/main.go'), 'utf8'), 'modified source\n');
  assert.equal(fs.readFileSync(path.join(root, 'cmd/dccex-driver/fyne_metadata_init.go'), 'utf8'), 'leave original metadata alone\n');
  assert.equal(fs.readFileSync(path.join(root, 'cmd/dccex-driver/AndroidManifest.xml'), 'utf8'), 'leave original manifest alone\n');
  assert.equal(fs.existsSync(path.join(root, 'cmd/dccex-driver/dccex.apk')), false);
}

test('parallel Android targets use distinct snapshots and preserve local source', async t => {
  const root = fixture(t);
  const builds = Promise.all(['arm64', 'amd64'].map(arch => build(root, arch, { WAIT_FOR_PEER: '1' })));
  const deadline = Date.now() + 15000;
  while (fs.readdirSync(path.join(root, 'logs')).filter(name => name.endsWith('.ready')).length < 2 && Date.now() < deadline) {
    await new Promise(resolve => setTimeout(resolve, 20));
  }
  fs.writeFileSync(path.join(root, 'logs/release'), 'continue');
  const results = await builds;
  for (const result of results) assert.equal(result.code, 0, result.output);
  const directories = ['arm64', 'amd64'].map(arch => fs.readFileSync(path.join(root, `logs/${arch}.ready`), 'utf8').trim());
  assert.notEqual(directories[0], directories[1]);
  for (const directory of directories) assert.equal(fs.existsSync(directory), false, 'temporary snapshot was not removed');
  for (const arch of ['arm64', 'amd64']) assert.equal(fs.readFileSync(path.join(root, `dist/android/dccex-${arch}.apk`), 'utf8'), arch);
  assertCheckoutUntouched(root);
});

for (const stage of ['fyne', 'signer', 'badging']) {
  test(`failed ${stage} leaves the last verified APK and checkout unchanged`, async t => {
    const root = fixture(t);
    fs.mkdirSync(path.join(root, 'dist/android'), { recursive: true });
    fs.writeFileSync(path.join(root, 'dist/android/dccex-arm64.apk'), 'previous verified APK');
    const result = await build(root, 'arm64', { FAIL_STAGE: stage });
    assert.notEqual(result.code, 0, result.output);
    assert.equal(fs.readFileSync(path.join(root, 'dist/android/dccex-arm64.apk'), 'utf8'), 'previous verified APK');
    assert.deepEqual(fs.readdirSync(path.join(root, 'dist/android')), ['dccex-arm64.apk']);
    const directory = fs.readFileSync(path.join(root, 'logs/arm64.ready'), 'utf8').trim();
    assert.equal(fs.existsSync(directory), false, 'failed build left a temporary snapshot');
    assertCheckoutUntouched(root);
  });
}
